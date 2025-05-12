package internal

import (
	"errors"
	"os"
	"path"
)

func getEntryPath(key string) string {
	homeDir, _ := os.UserHomeDir()

	return path.Join(homeDir, ".ccmd", key)
}

func createCacheDir() {
	homeDir, _ := os.UserHomeDir()

	os.Mkdir(path.Join(homeDir, ".ccmd"), 0750)
}

// could this be better ??
func CacheKeyExists(key string) bool {
	if _, err := os.Stat(getEntryPath(key)); errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}
