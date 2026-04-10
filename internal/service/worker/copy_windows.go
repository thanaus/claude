//go:build windows

package workerservice

import (
	"fmt"
	"os"
)

func copyEntryMetadata(_ string, dstPath string, info os.FileInfo) error {
	if err := os.Chmod(dstPath, info.Mode()); err != nil {
		return fmt.Errorf("cannot apply mode on destination entry %q: %w", dstPath, err)
	}

	mtime := info.ModTime()
	if err := os.Chtimes(dstPath, mtime, mtime); err != nil {
		return fmt.Errorf("cannot apply times on destination entry %q: %w", dstPath, err)
	}

	return nil
}
