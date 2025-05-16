package internal

import (
	"errors"
	"io"
	"strings"

	"github.com/spf13/afero"
)

type CacheProvider interface {
	GetEntry(string) (io.ReadCloser, error)
	PutEntry(string, io.Reader) (int64, error)
	GetFriendlyName() string
}

func GetCacheProviderFromURI(uri string) (CacheProvider, error) {
	if strings.HasPrefix(uri, "s3://") {
		return &S3Cache{URI: uri}, nil
	}

	if strings.HasPrefix(uri, "local://") {
		return &LocalCache{URI: uri, FS: afero.NewOsFs()}, nil
	}

	return nil, errors.New("unsupported cache provider")
}

// could this be better ??
// func CacheKeyExists(key string) bool {
// 	if _, err := os.Stat(getEntryPath(key)); errors.Is(err, os.ErrNotExist) {
// 		return false
// 	}
//
// 	return true
// }
