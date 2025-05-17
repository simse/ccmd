package internal_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/simse/cmd-cache/internal"
	"github.com/spf13/afero"
)

// helper to create files and directories under root
func createTree(t *testing.T, root string, entries []string) {
	for _, e := range entries {
		full := filepath.Join(root, e)
		if e[len(e)-1] == os.PathSeparator {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("MkdirAll %q: %v", full, err)
			}
		} else {
			dir := filepath.Dir(full)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("MkdirAll %q: %v", dir, err)
			}
			f, err := os.Create(full)
			if err != nil {
				t.Fatalf("Create %q: %v", full, err)
			}
			f.Close()
		}
	}
}

func TestFindFiles(t *testing.T) {
	tests := []struct {
		name            string
		includeGlobs    []string
		ignoreGlobs     []string
		treeEntries     []string
		wantRelPaths    []string
		skipNodeModules bool
	}{
		{
			name:         "simple ts",
			includeGlobs: []string{"**/*.ts"},
			ignoreGlobs:  nil,
			treeEntries: []string{
				"foo.ts", "bar.js",
				"sub/baz.ts", "sub/qux.jsx",
			},
			wantRelPaths: []string{"foo.ts", "sub/baz.ts"},
		},
		{
			name:         "nested include pruning",
			includeGlobs: []string{"src/**/*.go"},
			ignoreGlobs:  nil,
			treeEntries: []string{
				"src/a.go", "src/pkg/b.go",
				"other/c.go", "src/pkg/vendor/d.go",
			},
			wantRelPaths: []string{"src/a.go", "src/pkg/b.go", "src/pkg/vendor/d.go"},
		},
		{
			name:         "ignore patterns",
			includeGlobs: []string{"**/*.graphql"},
			ignoreGlobs:  []string{"server/schemas/ignoreme.graphql", "**/excluded/**"},
			treeEntries: []string{
				"server/schemas/a.graphql",
				"server/schemas/ignoreme.graphql",
				"server/schemas/excluded/x.graphql",
				"other/excluded/y.graphql",
				"root.graphql",
			},
			wantRelPaths: []string{"root.graphql", "server/schemas/a.graphql"},
		},
		{
			name:         "skip node_modules always",
			includeGlobs: []string{"**/*.ts"},
			ignoreGlobs:  nil,
			treeEntries: []string{
				"foo.ts",
				"node_modules/should.js",
				"node_modules/pkg/x.ts",
				"lib/z.ts",
			},
			wantRelPaths: []string{"foo.ts", "lib/z.ts"},
		},
		{
			name:         "no include prefixes (flat matches)",
			includeGlobs: []string{"**/*.md"},
			ignoreGlobs:  nil,
			treeEntries: []string{
				"README.md", "docs/doc.md", "a/b/c.md",
			},
			wantRelPaths: []string{"README.md", "docs/doc.md", "a/b/c.md"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup temp dir
			root, err := os.MkdirTemp("", "findfiles-test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)

			// Create tree
			createTree(t, root, tc.treeEntries)

			// Run FindFiles
			got, err := internal.FindFiles(tc.includeGlobs, tc.ignoreGlobs, root)
			if err != nil {
				t.Fatalf("FindFiles returned error: %v", err)
			}

			// Convert got to relative paths for comparison
			var gotRel []string
			for _, abs := range got {
				rel, err := filepath.Rel(root, abs)
				if err != nil {
					t.Fatalf("Rel %q: %v", abs, err)
				}
				gotRel = append(gotRel, rel)
			}
			sort.Strings(gotRel)
			sort.Strings(tc.wantRelPaths)

			if !reflect.DeepEqual(gotRel, tc.wantRelPaths) {
				t.Errorf("got %v, want %v", gotRel, tc.wantRelPaths)
			}
		})
	}
}

// Additionally, test extractPrefixes for edge cases
func TestExtractPrefixes(t *testing.T) {
	cases := map[string][]string{
		"flat":    {"*.ts", "*.go"},
		"simple":  {"src/*.ts", "lib/*.js"},
		"nested":  {"foo/bar/*.py", "foo/**/*.py"},
		"mix":     {"*.md", "docs/**/*.md"},
		"nowild":  {"static/index.html"},
		"onlydir": {"assets/css/*.css"},
	}

	for name, pats := range cases {
		t.Run(name, func(t *testing.T) {
			pfx := internal.ExtractPrefixes(pats)
			// Validate: none of the returned prefixes is "." or empty
			for _, d := range pfx {
				if d == "" || d == "." {
					t.Errorf("ExtractPrefixes(%v) returned invalid prefix %q", pats, d)
				}
			}
			// For patterns without wildcards, the prefix should be the dir of the pattern
			for _, pat := range pats {
				if !strings.ContainsAny(pat, "*?[") {
					dir := filepath.Clean(filepath.Dir(pat))
					found := false
					for _, got := range pfx {
						if got == dir {
							found = true
						}
					}
					if !found {
						t.Errorf("expected prefix %q for pattern %q, got %v", dir, pat, pfx)
					}
				}
			}
		})
	}
}

// FuzzHashDir fuzzes HashDir(fs, []string{…}, fingerprint)
func FuzzHashDir(f *testing.F) {
	// Seed corpus with a couple of realistic cases
	f.Add("foo.txt", []byte("hello"), "fp1")
	f.Add("dir/bar.txt", []byte{}, "")

	f.Fuzz(func(t *testing.T, name string, content []byte, fingerprint string) {
		fs := afero.NewMemMapFs()

		// Ensure parent directory exists
		if dir := filepath.Dir(name); dir != "." {
			if err := fs.MkdirAll(dir, 0755); err != nil {
				// e.g. invalid path -> skip
				t.Skip()
			}
		}

		// Write the fuzzed file
		if err := afero.WriteFile(fs, name, content, 0644); err != nil {
			t.Skip() // e.g. name contains invalid chars
		}

		// Call into your function
		hash1, err := internal.HashDir(fs, []string{name}, fingerprint)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 1) Always a 16-char hex string
		if len(hash1) != 16 {
			t.Errorf("hash length = %d; want 16", len(hash1))
		}
		if _, err := fmt.Sscanf(hash1, "%016x", new(uint64)); err != nil {
			t.Errorf("hash %q not valid hex: %v", hash1, err)
		}

		// 2) Determinism: two calls with identical inputs must match
		hash2, err := internal.HashDir(fs, []string{name}, fingerprint)
		if err != nil {
			t.Fatalf("unexpected error on 2nd call: %v", err)
		}
		if hash1 != hash2 {
			t.Errorf("hash not stable: %q vs %q", hash1, hash2)
		}
	})
}
