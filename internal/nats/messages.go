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
