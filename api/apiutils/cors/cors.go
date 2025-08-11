/*
 * Copyright (c) 2025 LoxiLB Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cors

import (
	"fmt"
	"sync"
)

type CORSManager struct {
	mutex          sync.RWMutex
	allowedOrigins map[string]bool
}

// CORS is a global instance of CORSManager
var CORS CORSManager

func GetCORSManager() *CORSManager {
	return &CORS
}

func (c *CORSManager) GetOrigin() map[string]bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.allowedOrigins == nil {
		c.allowedOrigins = make(map[string]bool)
		c.allowedOrigins["*"] = true // Default to allow all origins
	}

	if len(c.allowedOrigins) == 0 {
		c.allowedOrigins["*"] = true // Ensure at least the wildcard origin is present
	}
	// Return the allowed origins map
	return c.allowedOrigins
}

func (c *CORSManager) AddOrigin(origin string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	//Error origin already exists
	if _, exists := c.allowedOrigins[origin]; exists {
		err := fmt.Errorf("origin %s already exists in allowed origins", origin)
		return err
	}
	if c.allowedOrigins["*"] {
		delete(c.allowedOrigins, "*")
	}
	if c.allowedOrigins == nil {
		c.allowedOrigins = make(map[string]bool)
	}
	c.allowedOrigins[origin] = true
	return nil
}

func (c *CORSManager) RemoveOrigin(origin string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// Error if the origin is not present
	if _, exists := c.allowedOrigins[origin]; !exists {
		err := fmt.Errorf("origin %s not found in allowed origins", origin)
		return err
	}

	delete(c.allowedOrigins, origin)
	if len(c.allowedOrigins) == 0 {
		c.allowedOrigins["*"] = true
	}
	return nil
}

func (c *CORSManager) IsAllowed(origin string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.allowedOrigins["*"] {
		return true
	}
	return c.allowedOrigins[origin]
}
