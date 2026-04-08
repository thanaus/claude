package natsclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

const (
	filesStreamName  = "SYNC_FILES"
	statusStreamName = "SYNC_STATUS"
)

var streamConfigs = []*nats.StreamConfig{
	{
		Name:      filesStreamName,
		Subjects:  []string{"sync.files.>"},
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		Replicas:  1,
	},
	{
		Name:      statusStreamName,
		Subjects:  []string{"sync.status.>"},
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		Replicas:  1,
	},
}

// EnsureStreams creates or updates the JetStream streams required by Nexus.
func EnsureStreams(ctx context.Context, js nats.JetStreamContext) error {
	for _, cfg := range streamConfigs {
		if err := ensureStream(ctx, js, cfg); err != nil {
			return err
		}
	}

	return nil
}

func ensureStream(ctx context.Context, js nats.JetStreamContext, cfg *nats.StreamConfig) error {
	_, err := js.StreamInfo(cfg.Name, nats.Context(ctx))
	switch {
	case err == nil:
		if _, err := js.UpdateStream(cfg, nats.Context(ctx)); err != nil {
			return fmt.Errorf("cannot update stream %q: %w", cfg.Name, err)
		}
		return nil
	case errors.Is(err, nats.ErrStreamNotFound):
		if _, err := js.AddStream(cfg, nats.Context(ctx)); err != nil {
			return fmt.Errorf("cannot create stream %q: %w", cfg.Name, err)
		}
		return nil
	default:
		return fmt.Errorf("cannot inspect stream %q: %w", cfg.Name, err)
	}
}
