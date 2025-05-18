package cache

import (
	"errors"
	"io"
	"strings"

	"github.com/simse/ccmd/cache/s3"
	"github.com/simse/ccmd/internal"
	"github.com/spf13/afero"
)

type CacheProvider interface {
	GetEntry(string) (io.ReadCloser, error)
	PutEntry(string, io.Reader) (int64, error)
	GetFriendlyName() string
	Validate() error
}

func GetCacheProviderFromURI(uri string) (CacheProvider, error) {
	if strings.HasPrefix(uri, "s3://") {
		return &s3.S3Cache{URI: uri}, nil
	}

	if strings.HasPrefix(uri, "local://") {
		return &internal.LocalCache{URI: uri, FS: afero.NewOsFs()}, nil
	}

	return nil, errors.New("unsupported cache provider")
}
