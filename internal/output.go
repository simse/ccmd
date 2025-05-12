package internal

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func CaptureOutput(paths []string, key string) (int64, error) {
	createCacheDir()

	return CreateArchive(paths, getEntryPath(key))
}

func CreateArchive(paths []string, destPath string) (int64, error) {
	// Create destination file
	outFile, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	// Set up gzip writer with BestSpeed for fastest compression
	gzWriter, err := gzip.NewWriterLevel(outFile, gzip.BestSpeed)
	if err != nil {
		return 0, err
	}
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, source := range paths {
		// Walk through files and directories
		error := filepath.Walk(source, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Create header
			hdr, err := tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}

			// Preserve the relative path
			relPath, err := filepath.Rel(filepath.Dir(source), file)
			if err != nil {
				return err
			}
			hdr.Name = relPath

			// Write header
			if err := tarWriter.WriteHeader(hdr); err != nil {
				return err
			}

			// If not a regular file, skip writing content
			if !fi.Mode().IsRegular() {
				return nil
			}

			// Open file for reading
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			// Copy file data into tar
			_, err = io.Copy(tarWriter, f)
			return err
		})

		if error != nil {
			return 0, error
		}
	}

	// get archive size
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
