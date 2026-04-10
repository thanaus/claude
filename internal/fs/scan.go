package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ScanStats reports the number of entries and bytes seen during a scan.
type ScanStats struct {
	DiscoveredEntries uint64
	DiscoveredBytes   uint64
}

const readDirBatchSize = 1024

// Scan reads the root directory entries and emits one neutral entry per direct child.
func Scan(ctx context.Context, root string, visit func(Entry) error) (ScanStats, error) {
	var stats ScanStats

	f, err := os.Open(root)
	if err != nil {
		return stats, fmt.Errorf("cannot open source directory %q: %w", root, err)
	}
	defer f.Close()

	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		entries, err := f.ReadDir(readDirBatchSize)
		if err != nil && !errors.Is(err, io.EOF) {
			return stats, fmt.Errorf("cannot read source directory %q: %w", root, err)
		}

		for _, entry := range entries {
			fullPath := filepath.Join(root, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return stats, fmt.Errorf("cannot stat entry %q: %w", fullPath, err)
			}

			stats.DiscoveredEntries++
			stats.DiscoveredBytes += sizeBytes(info.Size())

			if err := visit(NewEntry(entry.Name(), info)); err != nil {
				return stats, err
			}
		}

		if errors.Is(err, io.EOF) {
			return stats, nil
		}
	}
}

func sizeBytes(size int64) uint64 {
	if size <= 0 {
		return 0
	}

	return uint64(size)
}
