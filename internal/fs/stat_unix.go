//go:build unix

package fs

import (
	"os"
	"syscall"
)

func fileStatMetadata(info os.FileInfo) (uint64, int64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, 0
	}

	return stat.Ino, stat.Ctim.Sec
}
