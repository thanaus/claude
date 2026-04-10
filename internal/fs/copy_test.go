package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyEntryCopiesFileContents(t *testing.T) {
	sourceRoot := t.TempDir()
	destinationRoot := t.TempDir()

	sourcePath := filepath.Join(sourceRoot, "nested", "file.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir source parent: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("payload"), 0o640); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source file: %v", err)
	}

	entry := NewEntry(filepath.Join("nested", "file.txt"), info)
	if err := CopyEntry(sourceRoot, destinationRoot, entry); err != nil {
		t.Fatalf("copy entry: %v", err)
	}

	destinationPath := filepath.Join(destinationRoot, "nested", "file.txt")
	data, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("expected copied payload, got %q", string(data))
	}
}
