package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cespare/xxhash/v2"
)

func FindFiles(patterns []string, rootDir string) ([]string, error) {
	var paths []string

	// Walk the directory tree
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		// Compute path relative to rootDir for matching & hashing
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Check each pattern
		for _, pat := range patterns {
			match, _ := doublestar.Match(pat, rel)
			if match {
				paths = append(paths, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return []string{}, err
	}

	// Deterministic order
	sort.Strings(paths)

	return paths, nil
}

func HashDir(paths []string, fingerprint string) (string, error) {
	// Deterministic order
	sort.Strings(paths)

	// Create a single xxhash hasher
	h := xxhash.New()

	// Hash paths and contents
	for _, path := range paths {
		// Include the file path in the hash
		h.WriteString(path)
		h.Write([]byte{0}) // separator

		// Open and hash file content
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		h.Write([]byte{0}) // separator
	}

	h.WriteString(fingerprint)

	rawHash := h.Sum64()

	// Return hex string
	return fmt.Sprintf("%016x", rawHash), nil
}
