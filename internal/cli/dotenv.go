package cli

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

var (
	dotenvMu     sync.Mutex
	dotenvValues map[string]string
	dotenvLoaded bool
)

// loadDotenv reads KEY=VALUE pairs from .env in the current directory.
// Results are cached after the first call. If the file does not exist
// or cannot be read, the cache is set to an empty map (no error).
func loadDotenv() map[string]string {
	dotenvMu.Lock()
	defer dotenvMu.Unlock()

	if dotenvLoaded {
		return dotenvValues
	}
	dotenvLoaded = true
	dotenvValues = make(map[string]string)

	f, err := os.Open(".env")
	if err != nil {
		return dotenvValues
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip matching quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		dotenvValues[key] = value
	}
	return dotenvValues
}

// resetDotenv clears the cached .env values, forcing a reload on the
// next call to getenv. Used by tests.
func resetDotenv() {
	dotenvMu.Lock()
	defer dotenvMu.Unlock()
	dotenvValues = nil
	dotenvLoaded = false
}

// getenv returns the value of key from the environment, falling back
// to the .env file in the current directory. Environment variables
// always take precedence.
func getenv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return loadDotenv()[key]
}

// getUser returns the current user identity: BS_USER env var, then
// .env fallback, then "anonymous".
func getUser() string {
	if u := getenv("BS_USER"); u != "" {
		return u
	}
	return "anonymous"
}
