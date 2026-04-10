package fs

import (
	iofs "io/fs"
	"os"
)

// StatMetadata contains platform-specific metadata used by comparison logic.
type StatMetadata struct {
	Inode uint64
	CTime int64
}

// EntryType classifies a filesystem entry independently from transport types.
type EntryType uint8

const (
	TypeUnknown EntryType = iota
	TypeFile
	TypeDir
	TypeSymlink
	TypeCharDev
	TypeDevice
	TypePipe
	TypeSocket
)

// Entry is the shared filesystem model consumed by scan, compare and copy.
type Entry struct {
	Path  string
	Name  string
	Type  EntryType
	Inode uint64
	Mode  uint32
	Size  int64
	CTime int64
	MTime int64
}

// MetadataFromFileInfo extracts the platform-specific metadata needed by callers.
func MetadataFromFileInfo(info os.FileInfo) StatMetadata {
	inode, ctime := fileStatMetadata(info)
	return StatMetadata{
		Inode: inode,
		CTime: ctime,
	}
}

// NewEntry builds a transport-neutral entry description from local file info.
func NewEntry(relativePath string, info os.FileInfo) Entry {
	mode := info.Mode()
	meta := MetadataFromFileInfo(info)

	return Entry{
		Path:  relativePath,
		Name:  info.Name(),
		Type:  TypeFromFileMode(mode),
		Inode: meta.Inode,
		Mode:  uint32(mode),
		Size:  info.Size(),
		CTime: meta.CTime,
		MTime: info.ModTime().Unix(),
	}
}

// TypeFromFileMode maps Go file modes to a stable entry type.
func TypeFromFileMode(mode iofs.FileMode) EntryType {
	t := mode.Type()
	switch {
	case t == 0:
		return TypeFile
	case t&iofs.ModeDir != 0:
		return TypeDir
	case t&iofs.ModeSymlink != 0:
		return TypeSymlink
	case t&iofs.ModeDevice != 0 && t&iofs.ModeCharDevice != 0:
		return TypeCharDev
	case t&iofs.ModeDevice != 0:
		return TypeDevice
	case t&iofs.ModeNamedPipe != 0:
		return TypePipe
	case t&iofs.ModeSocket != 0:
		return TypeSocket
	default:
		return TypeUnknown
	}
}
