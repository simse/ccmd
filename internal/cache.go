package internal

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

type CacheProvider interface {
	GetEntry(string) (io.ReadCloser, error)
	PutEntry(string, io.Reader) (int64, error)
}

func GetCacheProviderFromURI(uri string) (CacheProvider, error) {
	if strings.HasPrefix(uri, "s3://") {
		return &S3Cache{URI: uri}, nil
	}

	return nil, errors.New("unsupported cache provider")
}

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
