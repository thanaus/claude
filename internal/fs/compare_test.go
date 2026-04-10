package fs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompareDestinationMissing(t *testing.T) {
	root := t.TempDir()

	result, err := CompareDestination(root, Entry{
		Path:  filepath.Join("missing.txt"),
		Type:  TypeFile,
		Size:  1,
		MTime: 1,
	})
	if err != nil {
		t.Fatalf("compare missing: %v", err)
	}
	if result != ComparisonMissing {
		t.Fatalf("expected missing comparison, got %d", result)
	}
}

func TestCompareDestinationSizeAndMTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	now := time.Unix(1700000000, 0)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	sizeResult, err := CompareDestination(root, Entry{
		Path:  "file.txt",
		Type:  TypeFile,
		Size:  99,
		MTime: now.Unix(),
	})
	if err != nil {
		t.Fatalf("compare size: %v", err)
	}
	if sizeResult != ComparisonSize {
		t.Fatalf("expected size comparison, got %d", sizeResult)
	}

	mtimeResult, err := CompareDestination(root, Entry{
		Path:  "file.txt",
		Type:  TypeFile,
		Size:  5,
		MTime: now.Add(time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("compare mtime: %v", err)
	}
	if mtimeResult != ComparisonMTime {
		t.Fatalf("expected mtime comparison, got %d", mtimeResult)
	}
}
