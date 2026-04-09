//go:build windows

package lsservice

import "os"

func fileStatMetadata(info os.FileInfo) (uint64, int64) {
	return 0, 0
}
