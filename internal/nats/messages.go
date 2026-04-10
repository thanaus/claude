package natsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const rootDiscoveryKind = "root"

const (
	TypeUnknown uint8 = iota
	TypeFile
	TypeDir
	TypeSymlink
	TypeCharDev
	TypeDevice
	TypePipe
	TypeSocket
)

// DiscoveryMessage seeds the discovery stream for a job.
type DiscoveryMessage struct {
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	StartedAt time.Time `json:"startedAt"`
}

// NewRootDiscoveryMessage builds the initial discovery payload for a job.
func NewRootDiscoveryMessage(path string, startedAt time.Time) DiscoveryMessage {
	return DiscoveryMessage{
		Path:      path,
		Kind:      rootDiscoveryKind,
		StartedAt: startedAt,
	}
}

// WorkMessage contains the metadata published for each discovered entry.
type WorkMessage struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Type      uint8     `json:"type"`
	Inode     uint64    `json:"inode"`
	Mode      uint32    `json:"mode"`
	Size      int64     `json:"size"`
	CTime     int64     `json:"ctime"`
	MTime     int64     `json:"mtime"`
}

// MonitoringMessage carries live scan counters for the status watch mode.
type MonitoringMessage struct {
	Phase             string    `json:"phase,omitempty"`
	State             string    `json:"state"`
	StartedAt         time.Time `json:"startedAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	DiscoveredEntries uint64    `json:"discoveredEntries"`
	DiscoveredBytes   uint64    `json:"discoveredBytes"`
	PublishedWork     uint64    `json:"publishedWork"`
	WorkerProcessedDelta uint64 `json:"workerProcessedDelta,omitempty"`
	WorkerToCopyDelta    uint64 `json:"workerToCopyDelta,omitempty"`
	WorkerCopyMissingDelta uint64 `json:"workerCopyMissingDelta,omitempty"`
	WorkerCopySizeDelta    uint64 `json:"workerCopySizeDelta,omitempty"`
	WorkerCopyMTimeDelta   uint64 `json:"workerCopyMtimeDelta,omitempty"`
	WorkerCopyCTimeDelta   uint64 `json:"workerCopyCtimeDelta,omitempty"`
	WorkerOKDelta        uint64 `json:"workerOKDelta,omitempty"`
	WorkerErrorsDelta    uint64 `json:"workerErrorsDelta,omitempty"`
	WorkerLStatNanosDelta uint64 `json:"workerLstatNanosDelta,omitempty"`
	WorkerCopyNanosDelta  uint64 `json:"workerCopyNanosDelta,omitempty"`
	WorkerProcessed   uint64    `json:"workerProcessed,omitempty"`
	WorkerToCopy      uint64    `json:"workerToCopy,omitempty"`
	WorkerCopyMissing uint64    `json:"workerCopyMissing,omitempty"`
	WorkerCopySize    uint64    `json:"workerCopySize,omitempty"`
	WorkerCopyMTime   uint64    `json:"workerCopyMtime,omitempty"`
	WorkerCopyCTime   uint64    `json:"workerCopyCtime,omitempty"`
	WorkerOK          uint64    `json:"workerOK,omitempty"`
	WorkerErrors      uint64    `json:"workerErrors,omitempty"`
	WorkerLStatNanos  uint64    `json:"workerLstatNanos,omitempty"`
	WorkerCopyNanos   uint64    `json:"workerCopyNanos,omitempty"`
	Errors            uint64    `json:"errors"`
}

// PublishJSON serializes a payload and publishes it to JetStream.
func PublishJSON(ctx context.Context, js jetstream.JetStream, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cannot encode message for subject %q: %w", subject, err)
	}
	if _, err := js.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("cannot publish message to subject %q: %w", subject, err)
	}

	return nil
}
