package internal

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func CaptureOutput(paths []string, key string, cwd string) (int64, error) {
	createCacheDir()

	return CreateArchive(paths, getEntryPath(key), cwd)
}

func CreateArchive(paths []string, destPath string, cwd string) (int64, error) {
	outFile, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	gzWriter, err := gzip.NewWriterLevel(outFile, gzip.BestSpeed)
	if err != nil {
		return 0, err
	}
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	absoluteCwd, err := filepath.Abs(cwd)
	if err != nil {
		return 0, err
	}

	for _, source := range paths {
		err := filepath.Walk(source, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}

			// make file absolute so Rel() always works
			absFile := file
			if !filepath.IsAbs(file) {
				absFile, err = filepath.Abs(file)
				if err != nil {
					return err
				}
			}

			// compute path inside the tar relative to cwd
			relPath, err := filepath.Rel(absoluteCwd, absFile)
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(relPath)

			if err := tarWriter.WriteHeader(hdr); err != nil {
				return err
			}

			if fi.Mode().IsRegular() {
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()

				if _, err := io.Copy(tarWriter, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}

	stat, _ := outFile.Stat()
	return stat.Size(), nil
}

// ExtractArchive takes a .tar.gz archive at srcPath and extracts its
// contents into destDir, recreating the original file structure.
func ExtractArchive(key, destDir string) ([]string, error) {
	srcPath := getEntryPath(key)

	// Open the archive for reading
	inFile, err := os.Open(srcPath)
	if err != nil {
		return []string{}, err
	}
	defer inFile.Close()

	// Set up gzip reader
	gzReader, err := gzip.NewReader(inFile)
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
