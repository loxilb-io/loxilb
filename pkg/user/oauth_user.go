package user

import (
	"errors"
	"github.com/patrickmn/go-cache"
	"strings"
	"time"
)

const (
	OauthCacheExpirationTime  = 10 // 10 minutes
	OauthCacheCleanupInterval = 15 // 15 minutes
)

type OauthUserService struct {
	Cache *cache.Cache
}

// NewUserService creates a new UserService instance.
func NewOauthUserService() *OauthUserService {
	// Initialize the in-memory cache with expiration and cleanup intervals from the config
	c := cache.New(time.Duration(OauthCacheExpirationTime)*time.Minute, time.Duration(OauthCacheCleanupInterval)*time.Minute)
	return &OauthUserService{Cache: c}
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

	return token, true, nil
}

// DeleteOauthTokenCredential deletes the token from the cache and the database.
func (s *OauthUserService) DeleteOauthTokenCredential(token string) error {
	// Remove the token from the cache
	if _, found := s.Cache.Get(token); found {
		s.Cache.Delete(token)
		return nil
	} else {
		return errors.New("token not found")
	}
}
