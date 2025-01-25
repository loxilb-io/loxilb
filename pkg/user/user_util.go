package user

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt"
	"github.com/loxilb-io/loxilb/pkg/db"
	tk "github.com/loxilb-io/loxilib"
	"github.com/patrickmn/go-cache"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

// jwtKey is the secret key used for signing JWT tokens.
var (
	ErrTokenExpired = errors.New("token is expired")
	ErrInvalidToken = errors.New("invalid token")
	jwtKey          = []byte("netlox_secret_key")
)

// Claims represents the structure of the JWT claims.
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.StandardClaims
}

// RetryOperation retries the given operation function up to maxRetries times with a delay between retries.
func RetryOperation(operation func() error, maxRetries int, retryDelay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}
		time.Sleep(retryDelay)
	}
	return err
}

// UserService provides user-related operations such as user validation and token generation.
func (s *UserService) ValidateUser(username, password string) (string, bool, error) {
	var hashedPasswordBase64 string
	var role string
	// Query the database for the hashed password
	err := RetryOperation(func() error {
		query := db.SelectUserPasswordQuery
		err := s.DB.QueryRow(query, username).Scan(&hashedPasswordBase64, &role)
		if err != nil {
			if err == sql.ErrNoRows {
				tk.LogIt(tk.LogWarning, "User not found: %v\n", username)
				return errors.New("User not found") // User not found
			}
			tk.LogIt(tk.LogError, "Failed to query user: %v\n", err.Error())
			return err // Other errors
		}
		return nil
	}, db.MaxRetries, db.RetryDelay)

	if err != nil {
		if err == sql.ErrNoRows {
			tk.LogIt(tk.LogError, "User not found: %v\n", username)
			return "", false, err // User not found
		}
		return "", false, err // Other errors
	}

	hashedPasswordWithSalt, err := base64.StdEncoding.DecodeString(hashedPasswordBase64)
	if err != nil {
		tk.LogIt(tk.LogError, "Failed to decode hashed password: %v\n", err.Error())
		return "", false, err
	}

	salt := hashedPasswordWithSalt[:16]
	hashedPassword := hashedPasswordWithSalt[16:]

	hashedInputPassword := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)

	// Compare the hashed password with the input password using constant time comparison
	// to prevent timing attacks. The return value is 1 if the two slices are equal and 0 otherwise.
	if subtle.ConstantTimeCompare(hashedPassword, hashedInputPassword) == 1 {
		tk.LogIt(tk.LogInfo, "User validated successfully: %v\n", username)
		return role, true, nil // Valid password
	}

	tk.LogIt(tk.LogWarning, "Invalid password for user: %v\n", username)
	return "", false, err // Invalid password
}

// Validate the password against the following rules:
// - Must be at least 9 characters long
// - Must contain at least one uppercase letter
// - Must contain at least one lowercase letter
// - Must contain at least one number
// - Must contain at least one special character
// - Must not contain the same character more than twice in a row
// - Must not contain consecutive characters
// - Must not be the same as the username
// - Must not be the same as the previous password
func (s *UserService) validatePassword(username, password string) error {
	if len(password) < db.MinPasswordLength {
		err := errors.New("password must be at least 9 characters long")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}

	if password == username {
		err := errors.New("password must not be the same as the username")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	var prevChar rune
	var repeatCount int

	for i, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}

		if i > 0 {
			if char == prevChar {
				repeatCount++
				if repeatCount >= 2 {
					err := errors.New("password must not contain the same character more than twice in a row")
					tk.LogIt(tk.LogError, "%v\n", err.Error())
					return err
				}
			} else {
				repeatCount = 0
			}
		}

		prevChar = char
	}

	if !hasUpper {
		err := errors.New("password must contain at least one uppercase letter")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}
	if !hasLower {
		err := errors.New("password must contain at least one lowercase letter")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}
	if !hasNumber {
		err := errors.New("password must contain at least one number")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}
	if !hasSpecial {
		err := errors.New("password must contain at least one special character")
		tk.LogIt(tk.LogError, "%v\n", err.Error())
		return err
	}

	// FIXME: Check if the password is the same as the previous password
	var previousPassword string
	query := db.SelectUserPasswordQuery
	err := s.DB.QueryRow(query, username).Scan(&previousPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			// No previous password found, continue with validation
			tk.LogIt(tk.LogInfo, "No previous password found for username: %v\n", username)
		} else {
			tk.LogIt(tk.LogError, "Failed to query previous password: %v\n", err.Error())
			return err
		}
	} else {
		err = bcrypt.CompareHashAndPassword([]byte(previousPassword), []byte(password))
		if err == nil {
			err := errors.New("password must not be the same as the previous password")
			tk.LogIt(tk.LogError, "%v\n", err.Error())
			return err
		}
	}

	tk.LogIt(tk.LogInfo, "Password validated successfully")

	return nil
}

// Public wrapper function
func (s *UserService) ValidatePassword(username, password string) error {
	return s.validatePassword(username, password)
}

// ValidateToken validates a token using the in-memory cache and the database as a fallback.
func (s *UserService) ValidateToken(token string) (interface{}, error) {
	// Check the cache first
	if caches, found := s.Cache.Get(token); found {
		return caches, nil
	}

	// If not found in cache, check the database
	var username string
	var role string
	err := RetryOperation(func() error {
		query := db.ValidateTokenQuery
		err := s.DB.QueryRow(query, token).Scan(&username, &role)
		if err != nil {
			if err == sql.ErrNoRows {
				tk.LogIt(tk.LogError, "Token not found: %v\n", token)
				return errors.New("Token not found") // Token not found
			}
			tk.LogIt(tk.LogError, "Failed to query token: %v\n", err.Error())
			return err // Other errors
		}
		return nil
	}, db.MaxRetries, db.RetryDelay)

	if err != nil {
		return nil, err
	}

	// Cache the token
	combined := username + "|" + role
	s.Cache.Set(token, combined, cache.DefaultExpiration)

	return combined, nil
}

// GenerateToken generates a JWT token for a given username with a specified expiration time.
//
// Parameters:
//   - username: the username to be included in the token claims.
//   - expirationMinutes: the number of minutes until the token expires.
//
// Returns:
//   - string: the generated JWT token.
//   - error: an error if the token generation fails.
func GenerateToken(username, role string, expirationMinutes int) (string, error) {
	expiration := time.Now().Add(time.Duration(expirationMinutes) * time.Minute)
	claims := &Claims{
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiration.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func (s *UserService) saveToken(username, token, role string) error {
	expirationTime := time.Now().Add(time.Duration(db.TokenExpirationMinutes) * time.Minute)
	fmt.Printf("expirationTime: %v\n", expirationTime)
	query := db.InsertTokenQuery
	_, err := s.DB.Exec(query, token, username, expirationTime, role)
	return err
}
