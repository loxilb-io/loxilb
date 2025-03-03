package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"os"
	"strings"
	"time"
)

const (
	OauthCacheExpirationTime  = 10                              // 10 minutes
	OauthCacheCleanupInterval = 15                              // 15 minutes
	OauthTokenFilePath        = "/opt/loxilb/oauth_tokens.json" // Path to store the OAuth tokens on disk
)

type OauthUserService struct {
	Cache *cache.Cache
}

// TokenData represents the structure for storing OAuth token credentials on disk
type TokenData struct {
	Username     string    `json:"username"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// NewUserService creates a new UserService instance.
func NewOauthUserService() *OauthUserService {
	// Initialize the in-memory cache with expiration and cleanup intervals from the config
	c := cache.New(time.Duration(OauthCacheExpirationTime)*time.Minute, time.Duration(OauthCacheCleanupInterval)*time.Minute)

	newOauthService := &OauthUserService{Cache: c}

	// Load tokens from the file
	tokens, err := newOauthService.loadTokensFromFile()
	if err != nil {
		return nil
	}

	// Populate the cache with tokens from the file
	var updatedTokens []TokenData
	for _, tokenData := range tokens {
		if tokenData.Expiry.After(time.Now()) { // Only cache non-expired tokens
			role := "admin" // Assume the role is 'admin' as per your original logic
			combined := tokenData.Username + "|" + role + "|" + tokenData.RefreshToken
			c.Set(tokenData.AccessToken, combined, tokenData.Expiry.Sub(time.Now())) // Set with expiry time
			updatedTokens = append(updatedTokens, tokenData)
		}
	}

	newOauthService.saveTokensToFile(updatedTokens)

	return newOauthService
}

// ValidateOauthToken validates a token using the in-memory cache and the api callback
func (s *OauthUserService) ValidateOuathToken(token string) (interface{}, error) {
	// Check the cache first
	if caches, found := s.Cache.Get(token); found {
		return caches, nil
	} else {
		return nil, errors.New("token not found")
	}
}

// ValidateOuthTokenWithRefreshToken validates a token alongwith a refresh token using the in-memory cache
func (s *OauthUserService) ValidateOuathTokenWithRefreshToken(token, refreshToken string) (interface{}, error) {
	// Check the cache first
	if caches, found := s.Cache.Get(token); found {
		cachedTokenStr, ok := caches.(string)
		if !ok {
			return nil, errors.New("invalid token format in cache")
		}
		cacheKeys := strings.Split(cachedTokenStr, "|")
		if len(cacheKeys) != 3 {
			return nil, errors.New("invalid token format in cache")
		}
		if cacheKeys[2] != refreshToken {
			return nil, errors.New("invalid refresh token in cache")
		}
		return caches, nil
	} else {
		return nil, errors.New("token not found")
	}
}

// StoreOauthTokenCredentials stores the Ouath token credentials in cache
func (s *OauthUserService) StoreOauthTokenCredentials(username, token, refreshToken string, expiry time.Time) (string, bool, error) {
	// Save token
	// Store token in cache
	role := "admin"

	combined := username + "|" + role + "|" + refreshToken
	s.Cache.Set(token, combined, expiry.Sub(time.Now()))

	// Create TokenData structure
	tokenData := TokenData{
		Username:     username,
		AccessToken:  token,
		RefreshToken: refreshToken,
		Expiry:       expiry,
	}

	// Load existing tokens from file
	tokens, err := s.loadTokensFromFile()
	if err != nil {
		return "", false, err
	}

	// Append the new token to the list
	tokens = append(tokens, tokenData)

	// Save the updated list back to the file
	err = s.saveTokensToFile(tokens)
	if err != nil {
		return "", false, err
	}

	return token, true, nil
}

// DeleteOauthTokenCredential deletes the token from the cache and the database.
func (s *OauthUserService) DeleteOauthTokenCredential(token string) error {
	// Remove the token from the cache
	if _, found := s.Cache.Get(token); found {
		s.Cache.Delete(token)
	}

	// Load the existing tokens from the file
	tokens, err := s.loadTokensFromFile()
	if err != nil {
		return err
	}

	// Find and remove the token from the slice
	var updatedTokens []TokenData
	for _, tokenData := range tokens {
		if tokenData.AccessToken != token {
			updatedTokens = append(updatedTokens, tokenData)
		}
	}

	// Save the updated tokens back to the file
	return s.saveTokensToFile(updatedTokens)
}

// Helper function to load tokens from the file
func (s *OauthUserService) loadTokensFromFile() ([]TokenData, error) {
	// Check if the file exists
	if _, err := os.Stat(OauthTokenFilePath); os.IsNotExist(err) {
		// If the file doesn't exist, return an empty list of tokens
		return []TokenData{}, nil
	}

	// Read the tokens file
	data, err := os.ReadFile(OauthTokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %v", err)
	}

	// Unmarshal the JSON data into a list of tokens
	var tokens []TokenData
	err = json.Unmarshal(data, &tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %v", err)
	}

	return tokens, nil
}

// Helper function to save tokens to the file
func (s *OauthUserService) saveTokensToFile(tokens []TokenData) error {
	// Marshal the token data into JSON format
	tokenJson, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %v", err)
	}

	// Write the JSON data to the file
	err = os.WriteFile(OauthTokenFilePath, tokenJson, 0644)
	if err != nil {
		return fmt.Errorf("failed to write token data to file: %v", err)
	}

	return nil
}
