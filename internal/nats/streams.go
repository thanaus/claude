package natsclient

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	discoveryStreamName  = "DISCOVERY"
	workStreamName       = "WORK"
	monitoringStreamName = "MONITORING"
	WorkQueueMaxMsgs     = 500_000
)

// DiscoverySubject returns the subject prefix used to store discovery work for a job.
func DiscoverySubject(token string) string {
	return fmt.Sprintf("sync.discovery.%s", token)
}

// WorkSubject returns the subject prefix used to store execution work for a job.
func WorkSubject(token string) string {
	return fmt.Sprintf("sync.work.%s", token)
}

// MonitoringSubject returns the subject prefix used to store monitoring events for a job.
func MonitoringSubject(token string) string {
	return fmt.Sprintf("sync.monitoring.%s", token)
}

// ResourceStatus describes the provisioning status of a NATS resource.
type ResourceStatus struct {
	Name   string
	Status string
}

var streamConfigs = []jetstream.StreamConfig{
	{
		Name:      discoveryStreamName,
		Subjects:  []string{"sync.discovery.>"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
		MaxAge:    24 * time.Hour,
		MaxMsgs:   WorkQueueMaxMsgs,
		Discard:   jetstream.DiscardOld,
	},
	{
		Name:      workStreamName,
		Subjects:  []string{"sync.work.>"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
		MaxAge:    24 * time.Hour,
		MaxMsgs:   WorkQueueMaxMsgs,
		Discard:   jetstream.DiscardOld,
	},
	{
		Name:      monitoringStreamName,
		Subjects:  []string{"sync.monitoring.>"},
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Replicas:  1,
		MaxAge:    time.Hour,
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
	stream, err := js.Stream(ctx, cfg.Name)
	if err == nil {
		info, err := stream.Info(ctx)
		if err != nil {
			return ResourceStatus{}, fmt.Errorf("cannot inspect stream %q: %w", cfg.Name, err)
		}
		if streamConfigMatches(info.Config, cfg) {
			return ResourceStatus{Name: cfg.Name, Status: "ready"}, nil
		}

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

func streamConfigMatches(existing, expected jetstream.StreamConfig) bool {
	return existing.Name == expected.Name &&
		slices.Equal(existing.Subjects, expected.Subjects) &&
		existing.Retention == expected.Retention &&
		existing.Storage == expected.Storage &&
		existing.Replicas == expected.Replicas
}
