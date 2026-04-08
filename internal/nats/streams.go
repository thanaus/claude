package natsclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	filesStreamName  = "SYNC_FILES"
	statusStreamName = "SYNC_STATUS"
)

// FilesSubject returns the subject prefix used to store file events for a job.
func FilesSubject(token string) string {
	return fmt.Sprintf("sync.files.%s", token)
}

// StatusSubject returns the subject prefix used to store status events for a job.
func StatusSubject(token string) string {
	return fmt.Sprintf("sync.status.%s", token)
}

// ResourceStatus describes the provisioning status of a NATS resource.
type ResourceStatus struct {
	Name   string
	Status string
}

var streamConfigs = []jetstream.StreamConfig{
	{
		Name:      filesStreamName,
		Subjects:  []string{"sync.files.>"},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	},
	{
		Name:      statusStreamName,
		Subjects:  []string{"sync.status.>"},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	},
}

// EnsureStreams creates or updates the JetStream streams required by Nexus.
func EnsureStreams(ctx context.Context, js jetstream.JetStream) ([]ResourceStatus, error) {
	statuses := make([]ResourceStatus, 0, len(streamConfigs))
	for _, cfg := range streamConfigs {
		status, err := ensureStream(ctx, js, cfg)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func ensureStream(ctx context.Context, js jetstream.JetStream, cfg jetstream.StreamConfig) (ResourceStatus, error) {
	_, err := js.Stream(ctx, cfg.Name)
	if err == nil {
		if _, err := js.UpdateStream(ctx, cfg); err != nil {
			return ResourceStatus{}, fmt.Errorf("cannot update stream %q: %w", cfg.Name, err)
		}
		return ResourceStatus{Name: cfg.Name, Status: "ready"}, nil
	}
	if !errors.Is(err, jetstream.ErrStreamNotFound) {
		return ResourceStatus{}, fmt.Errorf("cannot inspect stream %q: %w", cfg.Name, err)
	}

	if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
		return ResourceStatus{}, fmt.Errorf("cannot create stream %q: %w", cfg.Name, err)
	}
	return ResourceStatus{Name: cfg.Name, Status: "created"}, nil
}
