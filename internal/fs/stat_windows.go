//go:build windows

package fs

import "os"

func fileStatMetadata(info os.FileInfo) (uint64, int64) {
	return 0, 0
}
