package user

import (
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/loxilb-io/loxilb/pkg/db"
	"github.com/patrickmn/go-cache"
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

// ValidateToken validates a token using the in-memory cache and the api callback
func (s *OauthUserService) ValidateToken(token string) (interface{}, error) {
	// Check the cache first
	if caches, found := s.Cache.Get(token); found {
		return caches, nil
	} else {
		return nil, errors.New("token not found")
	}
}

func (s *OauthUserService) Login(username, token string) (string, bool, error) {
	// Save token
	// Store token in cache
	role := "admin"

	combined := username + "|" + role
	s.Cache.Set(token, combined, db.TokenExpirationMinutes*time.Minute)

	return token, true, nil
}

// Logout deletes the token from the cache and the database.
func (s *OauthUserService) Logout(token string) error {
	// Remove the token from the cache
	if _, found := s.Cache.Get(token); found {
		s.Cache.Delete(token)
		return nil
	} else {
		return errors.New("token not found")
	}
}
