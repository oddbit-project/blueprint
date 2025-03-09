package secure

import (
	"os"
	"sync"
)

var (
	// envCache caches environment variables to avoid repeated system calls
	envCache     = make(map[string]string)
	envCacheMu   sync.RWMutex
	envCacheInit sync.Once
)

// GetEnvVar safely retrieves an environment variable
// It caches values to improve performance and reduce system calls
func GetEnvVar(name string) string {
	// Initialize the cache on first use
	envCacheInit.Do(func() {
		for _, env := range os.Environ() {
			for i := 0; i < len(env); i++ {
				if env[i] == '=' {
					envCacheMu.Lock()
					envCache[env[:i]] = env[i+1:]
					envCacheMu.Unlock()
					break
				}
			}
		}
	})

	// Check cache first
	envCacheMu.RLock()
	if val, ok := envCache[name]; ok {
		envCacheMu.RUnlock()
		return val
	}
	envCacheMu.RUnlock()

	// If not in cache, get from OS
	val := os.Getenv(name)
	
	// Update cache
	envCacheMu.Lock()
	envCache[name] = val
	envCacheMu.Unlock()
	
	return val
}

// SetEnvVar sets an environment variable and updates the cache
func SetEnvVar(name, value string) error {
	err := os.Setenv(name, value)
	if err != nil {
		return err
	}
	
	// Update cache
	envCacheMu.Lock()
	envCache[name] = value
	envCacheMu.Unlock()
	
	return nil
}