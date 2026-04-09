package natsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const rootDiscoveryKind = "root"

// DiscoveryMessage seeds the discovery stream for a job.
type DiscoveryMessage struct {
	Token     string    `json:"token"`
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	StartedAt time.Time `json:"startedAt"`
}

// NewRootDiscoveryMessage builds the initial discovery payload for a job.
func NewRootDiscoveryMessage(token, path string, startedAt time.Time) DiscoveryMessage {
	return DiscoveryMessage{
		Token:     token,
		Path:      path,
		Kind:      rootDiscoveryKind,
		StartedAt: startedAt,
	}
}

// WorkMessage contains the metadata published for each discovered entry.
type WorkMessage struct {
	Token     string    `json:"token"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Mode      uint32    `json:"mode"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	IsDir     bool      `json:"isDir"`
	IsSymlink bool      `json:"isSymlink"`
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
