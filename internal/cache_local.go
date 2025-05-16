package internal

import (
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

type LocalCache struct {
	URI string
	FS  afero.Fs
}

func (l *LocalCache) GetEntry(key string) (io.ReadCloser, error) {
	// check if file exists
	if _, err := l.FS.Stat(l.getEntryPath(key)); err != nil {
		return nil, err
	}

	// open file
	file, err := l.FS.Open(l.getEntryPath(key))
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (l *LocalCache) PutEntry(key string, body io.Reader) (int64, error) {
	// ensure directory exists
	l.createCacheDir()

	// open file
	file, err := l.FS.Create(l.getEntryPath(key))

	if err != nil {
		return 0, err
	}

	// write to file
	bytesWritten, err := io.Copy(file, body)

	if err != nil {
		return 0, err
	}

	file.Close()

	return bytesWritten, nil
}

func (l *LocalCache) GetFriendlyName() string {
	return "local folder"
}

func (l *LocalCache) getCacheDir() string {
	dir := strings.Replace(l.URI, "local://", "", 1)

	dirAbsolute, _ := filepath.Abs(dir)

	return dirAbsolute
}

func (l *LocalCache) getEntryPath(key string) string {
	return path.Join(l.getCacheDir(), key)
}

func (l *LocalCache) createCacheDir() {
	l.FS.MkdirAll(l.getCacheDir(), 0750)
}
