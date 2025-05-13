package internal

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cespare/xxhash/v2"
)

func FindFiles(
	includePatterns []string,
	ignorePatterns []string,
	rootDir string,
) ([]string, error) {
	var paths []string
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// compute relative path
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// 1) global ignores
		for _, pat := range ignorePatterns {
			if ok, _ := doublestar.Match(pat, rel); ok {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 2) built-in skips
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}

		// 3) match includes
		if !d.IsDir() {
			for _, pat := range includePatterns {
				if ok, _ := doublestar.Match(pat, rel); ok {
					paths = append(paths, path)
					break
				}
			}
		}

		return nil
	}

	if err := filepath.WalkDir(rootDir, walkFn); err != nil {
		return nil, err
	}

	sort.Strings(paths)
	return paths, nil
}

// extractPrefixes pulls the static directory prefixes from your globs.
// It deliberately skips any empty or “.” prefix.
func ExtractPrefixes(patterns []string) []string {
	seen := make(map[string]struct{})
	for _, pat := range patterns {
		cut := pat
		if idx := strings.IndexAny(pat, "*?["); idx >= 0 {
			cut = pat[:idx]
		}
		dir := filepath.Clean(filepath.Dir(cut))
		// skip empty (".") prefixes
		if dir == "" || dir == "." {
			continue
		}
		seen[dir] = struct{}{}
	}
	var prefixes []string
	for p := range seen {
		prefixes = append(prefixes, p)
	}
	return prefixes
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
