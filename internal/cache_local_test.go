package internal_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/simse/ccmd/internal"
	"github.com/spf13/afero"
)

func TestPutEntryAndGetEntry(t *testing.T) {
	// Set up an in-memory filesystem and cache
	fs := afero.NewMemMapFs()
	cache := &internal.LocalCache{
		URI: "local://testdir",
		FS:  fs,
	}

	// Test PutEntry writes data and returns the correct byte count
	data := "hello world"
	n, err := cache.PutEntry("foo.txt", bytes.NewBufferString(data))
	if err != nil {
		t.Fatalf("PutEntry returned unexpected error: %v", err)
	}
	if n != int64(len(data)) {
		t.Errorf("PutEntry wrote %d bytes; want %d", n, len(data))
	}

	// Test GetEntry retrieves the same data
	rc, err := cache.GetEntry("foo.txt")
	if err != nil {
		t.Fatalf("GetEntry returned unexpected error: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading from GetEntry returned error: %v", err)
	}
	if string(content) != data {
		t.Errorf("GetEntry returned %q; want %q", string(content), data)
	}
}

func TestGetEntryNotExist(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache := &internal.LocalCache{
		URI: "local://testdir",
		FS:  fs,
	}

	// Attempting to get a non-existent entry should return an error
	if _, err := cache.GetEntry("nofile"); err == nil {
		t.Errorf("GetEntry expected error for missing file; got nil")
	}
}

func TestGetFriendlyName(t *testing.T) {
	cache := &internal.LocalCache{}
	want := "local folder"
	if got := cache.GetFriendlyName(); got != want {
		t.Errorf("GetFriendlyName = %q; want %q", got, want)
	}
}

func TestPutEntryCreateError(t *testing.T) {
	// A read-only filesystem should cause Create to fail
	base := afero.NewMemMapFs()
	fs := afero.NewReadOnlyFs(base)
	cache := &internal.LocalCache{
		URI: "local://testdir",
		FS:  fs,
	}
	if _, err := cache.PutEntry("bar.txt", bytes.NewBufferString("data")); err == nil {
		t.Error("PutEntry expected error when create fails; got nil")
	}
}
