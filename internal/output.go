package internal

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func CaptureOutput(paths []string, key string, cwd string) (io.Reader, error) {
	createCacheDir()

	return CreateArchive(paths, cwd)
}

func CreateArchive(paths []string, cwd string) (io.Reader, error) {
	pr, pw := io.Pipe()

	go func() {
		// If anything errors out, send it down the pipe and close
		defer pw.Close()

		gz := gzip.NewWriter(pw)
		defer gz.Close()

		tw := tar.NewWriter(gz)
		defer tw.Close()

		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			pw.CloseWithError(err)
			return
		}

		for _, src := range paths {
			err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				hdr, err := tar.FileInfoHeader(fi, "")
				if err != nil {
					return err
				}
				absFile := file
				if !filepath.IsAbs(file) {
					absFile, err = filepath.Abs(file)
					if err != nil {
						return err
					}
				}
				rel, err := filepath.Rel(absCwd, absFile)
				if err != nil {
					return err
				}
				hdr.Name = filepath.ToSlash(rel)

				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				if fi.Mode().IsRegular() {
					f, err := os.Open(file)
					if err != nil {
						return err
					}
					defer f.Close()
					if _, err := io.Copy(tw, f); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	return pr, nil
}

// ExtractArchive takes a .tar.gz archive at srcPath and extracts its
// contents into destDir, recreating the original file structure.
func ExtractArchive(cacheBody io.Reader, destDir string) ([]string, error) {
	// srcPath := getEntryPath(key)

	// Open the archive for reading
	// inFile, err := os.Open(srcPath)
	// if err != nil {
	// 	return []string{}, err
	// }
	// defer inFile.Close()

	// Set up gzip reader
	gzReader, err := gzip.NewReader(cacheBody)
	if err != nil {
		return []string{}, err
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	writtenFiles := []string{}

	// Iterate through entries
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return writtenFiles, err
		}

		targetPath := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
				return writtenFiles, err
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return writtenFiles, err
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return writtenFiles, err
			}
			defer outFile.Close()

			// Copy file contents
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return writtenFiles, err
			}

			writtenFiles = append(writtenFiles, outFile.Name())
		default:
			// Handle other file types if needed
		}
	}

	return writtenFiles, nil
}
