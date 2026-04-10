package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyEntry copies a supported source entry into the destination tree.
func CopyEntry(sourceRoot, destinationRoot string, entry Entry) error {
	sourcePath := filepath.Join(sourceRoot, entry.Path)
	destinationPath := filepath.Join(destinationRoot, entry.Path)

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("cannot create destination parent for %q: %w", destinationPath, err)
	}

	switch entry.Type {
	case TypeFile:
		return copyFile(sourcePath, destinationPath)
	case TypeDir:
		if err := os.MkdirAll(destinationPath, os.FileMode(entry.Mode).Perm()); err != nil {
			return fmt.Errorf("cannot create destination directory %q: %w", destinationPath, err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("cannot stat source directory %q: %w", sourcePath, err)
		}
		return copyEntryMetadata(sourcePath, destinationPath, info)
	default:
		return fmt.Errorf("cannot copy unsupported entry type %d for %q", entry.Type, entry.Path)
	}
}

func copyFile(sourcePath, destinationPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot open source file %q: %w", sourcePath, err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat source file %q: %w", sourcePath, err)
	}

	dst, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("cannot create destination file %q: %w", destinationPath, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("cannot copy file %q to %q: %w", sourcePath, destinationPath, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("cannot close destination file %q: %w", destinationPath, err)
	}

	if err := copyEntryMetadata(sourcePath, destinationPath, info); err != nil {
		return err
	}

	return nil
}
