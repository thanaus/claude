//go:build unix

package workerservice

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/pkg/xattr"
)

func copyEntryMetadata(srcPath, dstPath string, info os.FileInfo) error {
	if err := os.Chmod(dstPath, info.Mode()); err != nil {
		return fmt.Errorf("cannot apply mode on destination entry %q: %w", dstPath, err)
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
		if err := os.Chown(dstPath, int(stat.Uid), int(stat.Gid)); err != nil && !isIgnorableMetadataError(err) {
			return fmt.Errorf("cannot apply ownership on destination entry %q: %w", dstPath, err)
		}
	}

	if err := copyExtendedAttributes(srcPath, dstPath); err != nil {
		return err
	}

	atime, mtime := fileAccessAndModTime(info)
	if err := os.Chtimes(dstPath, atime, mtime); err != nil {
		return fmt.Errorf("cannot apply times on destination entry %q: %w", dstPath, err)
	}

	return nil
}

func fileAccessAndModTime(info os.FileInfo) (time.Time, time.Time) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
		return time.Unix(stat.Atim.Sec, stat.Atim.Nsec), info.ModTime()
	}

	return info.ModTime(), info.ModTime()
}

func copyExtendedAttributes(srcPath, dstPath string) error {
	attrs, err := xattr.List(srcPath)
	if err != nil {
		if isIgnorableMetadataError(err) {
			return nil
		}
		return fmt.Errorf("cannot list xattrs for %q: %w", srcPath, err)
	}

	for _, attr := range attrs {
		data, err := xattr.Get(srcPath, attr)
		if err != nil {
			if isIgnorableMetadataError(err) {
				continue
			}
			return fmt.Errorf("cannot read xattr %q for %q: %w", attr, srcPath, err)
		}
		if err := xattr.Set(dstPath, attr, data); err != nil {
			if isIgnorableMetadataError(err) {
				continue
			}
			return fmt.Errorf("cannot write xattr %q for %q: %w", attr, dstPath, err)
		}
	}

	return nil
}

func isIgnorableMetadataError(err error) bool {
	return errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP)
}
