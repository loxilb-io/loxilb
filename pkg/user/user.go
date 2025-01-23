package user

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
	cmn "github.com/loxilb-io/loxilb/common"
	"github.com/loxilb-io/loxilb/pkg/db"
	tk "github.com/loxilb-io/loxilib"
	"github.com/patrickmn/go-cache"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

const (
	CacheExpirationTime  = 5  // 5 minutes
	CacheCleanupInterval = 10 // 10 minutes
)

type UserService struct {
	DB    *sql.DB
	Cache *cache.Cache
}

// NewUserService creates a new UserService instance.
func NewUserService() *UserService {
	// Initialize the in-memory cache with expiration and cleanup intervals from the config
	userDB, err := db.InitDB()
	if err != nil {
		tk.LogIt(tk.LogCritical, "Failed to initialize database: %v\n", err)
	}
	c := cache.New(time.Duration(CacheExpirationTime)*time.Minute, time.Duration(CacheCleanupInterval)*time.Minute)
	return &UserService{DB: userDB, Cache: c}
}

// ValidateUser validates the user credentials.
// It returns the user's role if the credentials are valid, or an error if the credentials are invalid.
func (s *UserService) AddUser(user cmn.User) (int, error) {
	var userID int
	err := RetryOperation(func() error {
		if err := s.validatePassword(user.Username, user.Password); err != nil {
			tk.LogIt(tk.LogError, "Password validation failed: %v\n", err.Error())
			return err
		}

		salt := make([]byte, 16)
		_, err := rand.Read(salt)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to generate salt: %v\n", err.Error())
			return err
		}

		// Hash the password with the salt
		hashedPassword := pbkdf2.Key([]byte(user.Password), salt, 10000, 32, sha256.New)
		hashedPasswordWithSalt := append(salt, hashedPassword...)
		hashedPasswordBase64 := base64.StdEncoding.EncodeToString(hashedPasswordWithSalt)

		// Insert the user into the database
		query := db.InsertUserQuery
		result, err := s.DB.Exec(query, user.Username, hashedPasswordBase64, user.CreatedAt, user.Role)
		if err != nil {
			if db.IsDuplicateEntryError(err) {
				tk.LogIt(tk.LogWarning, "Duplicate username: %v\n", user.Username)
				return errors.New("username already exists")
			}
			tk.LogIt(tk.LogError, "Failed to insert user: %v\n", err.Error())
			return err
		}

		lastInsertID, err := result.LastInsertId()
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to get last insert ID: %v\n", err.Error())
			return err
		}
		userID = int(lastInsertID)

		tk.LogIt(tk.LogInfo, "User created: %v\n", user.Username)
		return nil
	}, db.MaxRetries, db.RetryDelay)
	return userID, err
}

// GetUsers returns all users from the database.
func (s *UserService) GetUsers() ([]cmn.User, error) {
	var users []cmn.User
	err := RetryOperation(func() error {
		query := db.SelectAllUsersQuery
		rows, err := s.DB.Query(query)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to fetch users: %v\n", err.Error())
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var user cmn.User
			var createdAt string
			var role string
			err := rows.Scan(&user.ID, &user.Username, &user.Password, &createdAt, &role)
			if err != nil {
				tk.LogIt(tk.LogError, "Failed to scan user: %v\n", err.Error())
				return err
			}
			user.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)

			if err != nil {
				tk.LogIt(tk.LogError, "Failed to parse created_at: %v\n", err.Error())
				return err
			}
			users = append(users, user)
		}

		if err = rows.Err(); err != nil {
			tk.LogIt(tk.LogError, "Rows error: %v\n", err.Error())
			return err
		}

		return nil
	}, db.MaxRetries, db.RetryDelay)
	return users, err
}

// DeleteUser deletes a user from the database.
func (s *UserService) DeleteUser(id int) error {
	return RetryOperation(func() error {
		query := db.DeleteUserQuery
		_, err := s.DB.Exec(query, id)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete user: %v\n", err.Error())
		}
		return err
	}, db.MaxRetries, db.RetryDelay)
}

// UpdateUser updates a user in the database.
func (s *UserService) UpdateUser(user cmn.User) error {
	return RetryOperation(func() error {
		// Check if the user exists
		var existingUser cmn.User
		query := db.SelectUserQuery
		err := s.DB.QueryRow(query, user.ID).Scan(&existingUser.ID, &existingUser.Username, &existingUser.Password, &existingUser.Role)
		if err != nil {
			if err == sql.ErrNoRows {
				tk.LogIt(tk.LogError, "User not found: %s\n", err.Error())
				return errors.New("user not found")
			}
			tk.LogIt(tk.LogError, "Failed to query user: %s\n", err.Error())
			return err
		}

		// Validate the new password
		if err := s.validatePassword(user.Username, user.Password); err != nil {
			tk.LogIt(tk.LogError, "Password validation failed: %v\n", err.Error())
			return err
		}

		// Hash the new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to hash password: %v\n", err.Error())
			return err
		}

		// Update Role if it is not empty
		if user.Role == "" {
			user.Role = existingUser.Role
		}

		// Update the user information
		updateQuery := db.UpdateUserQuery
		_, err = s.DB.Exec(updateQuery, user.Username, string(hashedPassword), user.Role, user.ID)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to update user: %v\n", err.Error())
			return err
		}

		tk.LogIt(tk.LogInfo, "User updated successfully: %v\n", user.Username)
		return nil
	}, db.MaxRetries, db.RetryDelay)
}

func (s *UserService) Login(username, password string) (string, bool, error) {
	// User check
	role, vaild, err := s.ValidateUser(username, password)
	if err != nil {
		return "", false, err
	}
	// Gen Token
	if vaild {
		token, err := GenerateToken(username, role, db.TokenExpirationMinutes)
		if err != nil {
			return "", false, err
		}
		// Save token
		// Store token in cache
		combined := username + "|" + role
		s.Cache.Set(token, combined, db.TokenExpirationMinutes*time.Minute)

		// insert token in the DB
		err = s.saveToken(username, token, role)
		if err != nil {
			tk.LogIt(tk.LogWarning, "Save fail : %v \n", err.Error())
			return "", false, err
		}
		// return result
		return token, true, nil // Valid login

	}
	return "", false, nil // Invalid Login
}

// Logout deletes the token from the cache and the database.
func (s *UserService) Logout(tokenString string) error {
	// Remove the token from the cache
	s.Cache.Delete(tokenString)

	return RetryOperation(func() error {
		query := db.DeleteTokenQuery
		_, err := s.DB.Exec(query, tokenString)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete token: %v\n", err.Error())
			return err
		}

		tk.LogIt(tk.LogInfo, "User logged out and token deleted: %v\n", tokenString)
		return nil
	}, db.MaxRetries, db.RetryDelay)
}

// UserServiceTicker is a periodic function that runs every 10 seconds.
func (s *UserService) UserServiceTicker() {
	// DB Connection Check
	if s.DB == nil {
		tk.LogIt(tk.LogCritical, "Database connection is nil\n")
		if err := s.reconnectDB(); err != nil {
			return
		}
	}
	if err := s.DB.Ping(); err != nil {
		tk.LogIt(tk.LogError, "Failed to ping database: %v\n", err.Error())
		// reconnect to the database
		if err := s.reconnectDB(); err != nil {
			return
		}
	}

	// Expired Token Cleanup
	s.cleanupExpiredTokens()
}

// reconnectDB reconnects to the database.
func (s *UserService) reconnectDB() error {
	tempDB, err := db.ConnectWithRetry(1, 2*time.Second)
	if err != nil {
		tk.LogIt(tk.LogCritical, "Failed to reconnect to the database: %v\n", err)
		return err
	}
	s.DB = tempDB
	tk.LogIt(tk.LogInfo, "Reconnected to the database\n")
	return nil
}

// cleanupExpiredTokens removes tokens with expired 'expires_at' values
func (s *UserService) cleanupExpiredTokens() {
	query := db.DeleteExpiredTokenQuery

	result, err := s.DB.Exec(query)
	if err != nil {
		tk.LogIt(tk.LogInfo, "Failed to delete expired tokens: %v\n", err)
		return
	}

	// Log the number of deleted rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tk.LogIt(tk.LogInfo, "Failed to retrieve rows affected: %v\n", err)
		return
	}

	tk.LogIt(tk.LogInfo, "Deleted %d expired tokens\n", rowsAffected)
}
