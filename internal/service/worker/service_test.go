package workerservice

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	natsclient "github.com/nexus/nexus/internal/nats"
)

func TestCompareDestinationMissingFileNeedsCopy(t *testing.T) {
	root := t.TempDir()

	result, err := compareDestination(root, natsclient.WorkMessage{Path: "missing.txt"})
	if err != nil {
		t.Fatalf("expected no error for missing destination file, got %v", err)
	}
	if result != comparisonToCopy {
		t.Fatalf("expected missing destination to require copy, got %v", result)
	}
}

func TestCompareDestinationMatchingFileIsOK(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "same.txt")
	content := []byte("hello")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("failed to stat fixture file: %v", err)
	}
	_, ctime := fileStatMetadata(info)
	work := natsclient.WorkMessage{
		Path:  "same.txt",
		Size:  info.Size(),
		MTime: info.ModTime().Unix(),
		CTime: ctime,
	}

	result, err := compareDestination(root, work)
	if err != nil {
		t.Fatalf("expected no error for matching file, got %v", err)
	}
	if result != comparisonOK {
		t.Fatalf("expected identical destination to be OK, got %v", result)
	}
}

func TestCompareDestinationDifferentMetadataNeedsCopy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "diff.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("failed to stat fixture file: %v", err)
	}
	_, ctime := fileStatMetadata(info)
	work := natsclient.WorkMessage{
		Path:  "diff.txt",
		Size:  info.Size() + 1,
		MTime: info.ModTime().Unix(),
		CTime: ctime,
	}

	result, err := compareDestination(root, work)
	if err != nil {
		t.Fatalf("expected no error for differing file, got %v", err)
	}
	if result != comparisonToCopy {
		t.Fatalf("expected differing destination to require copy, got %v", result)
	}
}

func TestIsFetchTimeoutRecognizesCommonTimeouts(t *testing.T) {
	if !isFetchTimeout(os.ErrDeadlineExceeded) {
		t.Fatalf("expected os deadline exceeded to be treated as timeout")
	}
	if !isFetchTimeout(&timeoutError{}) {
		t.Fatalf("expected timeout-like error message to be treated as timeout")
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string {
	return "fetch timeout"
}

func (e *timeoutError) Timeout() bool {
	return true
}

func (e *timeoutError) Temporary() bool {
	return true
}

var _ interface {
	error
	Timeout() bool
	Temporary() bool
} = (*timeoutError)(nil)

func TestWorkerConsumerNameStable(t *testing.T) {
	if got := workerConsumerName("abc-123"); got != "worker-abc-123" {
		t.Fatalf("unexpected consumer name: %s", got)
	}
}

func TestDefaultWorkerCountValue(t *testing.T) {
	if defaultWorkerCount != 4 {
		t.Fatalf("expected default worker count to be 4, got %d", defaultWorkerCount)
	}
}

func TestWorkerFetchBatchSizeValue(t *testing.T) {
	if workerFetchBatchSize != 256 {
		t.Fatalf("expected worker fetch batch size to be 256, got %d", workerFetchBatchSize)
	}
}

func TestIsTerminalWorkerInput(t *testing.T) {
	if !isTerminalWorkerInput(natsclient.JobStateCompleted) {
		t.Fatalf("expected completed state to be terminal")
	}
	if !isTerminalWorkerInput(natsclient.JobStateFailed) {
		t.Fatalf("expected failed state to be terminal")
	}
	if isTerminalWorkerInput(natsclient.JobStateRunning) {
		t.Fatalf("expected running state to be non-terminal")
	}
}

func TestCompareDestinationMTimeDifferenceNeedsCopy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mtime.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("failed to stat fixture file: %v", err)
	}
	_, ctime := fileStatMetadata(info)
	work := natsclient.WorkMessage{
		Path:  "mtime.txt",
		Size:  info.Size(),
		MTime: info.ModTime().Add(time.Second).Unix(),
		CTime: ctime,
	}

	result, err := compareDestination(root, work)
	if err != nil {
		t.Fatalf("expected no error for differing mtime, got %v", err)
	}
	if result != comparisonToCopy {
		t.Fatalf("expected differing mtime to require copy, got %v", result)
	}
}
