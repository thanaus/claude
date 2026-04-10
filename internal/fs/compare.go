package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Comparison classifies how a destination entry differs from the source entry.
type Comparison uint8

const (
	ComparisonUnknown Comparison = iota
	ComparisonOK
	ComparisonMissing
	ComparisonSize
	ComparisonMTime
	ComparisonCTime
)

// CompareDestination compares a discovered entry with its destination counterpart.
func CompareDestination(root string, entry Entry) (Comparison, error) {
	fullPath := filepath.Join(root, entry.Path)
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ComparisonMissing, nil
		}
		return ComparisonUnknown, fmt.Errorf("cannot stat destination entry %q: %w", fullPath, err)
	}

	ctime := MetadataFromFileInfo(info).CTime
	switch {
	case info.Size() != entry.Size:
		return ComparisonSize, nil
	case info.ModTime().Unix() != entry.MTime:
		return ComparisonMTime, nil
	case entry.CTime > ctime:
		return ComparisonCTime, nil
	}

	return ComparisonOK, nil
}
