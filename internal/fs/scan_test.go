package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanReadsOnlyDirectChildren(t *testing.T) {
	root := t.TempDir()
	nestedDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	rootFile := filepath.Join(root, "top.txt")
	if err := os.WriteFile(rootFile, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	filePath := filepath.Join(nestedDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var got []Entry
	stats, err := Scan(context.Background(), root, func(entry Entry) error {
		got = append(got, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if stats.DiscoveredEntries != 2 {
		t.Fatalf("expected 2 entries, got %d", stats.DiscoveredEntries)
	}
	if stats.DiscoveredBytes != 3 {
		t.Fatalf("expected 3 discovered bytes, got %d", stats.DiscoveredBytes)
	}

	expectedDir := "nested"
	expectedFile := "top.txt"
	seen := map[string]Entry{}
	for _, entry := range got {
		seen[entry.Path] = entry
	}

	if entry, ok := seen[expectedDir]; !ok || entry.Type != TypeDir {
		t.Fatalf("expected nested directory entry %q, got %#v", expectedDir, entry)
	}
	if entry, ok := seen[expectedFile]; !ok || entry.Type != TypeFile {
		t.Fatalf("expected nested file entry %q, got %#v", expectedFile, entry)
	}
	if _, ok := seen[filepath.Join("nested", "file.txt")]; ok {
		t.Fatalf("did not expect nested file to be scanned")
	}
}
