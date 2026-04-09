package syncservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nexus/nexus/internal/config"
	natsclient "github.com/nexus/nexus/internal/nats"
	"github.com/nats-io/nats.go/jetstream"
)

const maxTokenCollisionRetries = 3

// Input holds the parameters required by the sync use case.
type Input struct {
	Source      string
	Destination string
}

// Result contains the outcome of the initial sync checks.
type Result struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
	Token          string
	Streams        []natsclient.ResourceStatus
	KeyValue       natsclient.ResourceStatus
	Job            natsclient.ResourceStatus
}

// Service orchestrates the initial sync workflow.
type Service struct{}

// New returns a sync service ready to provision NATS resources.
func New() Service {
	return Service{}
}

// Provision prepares the required NATS resources using the resolved config in ctx.
func (s Service) Provision(ctx context.Context, in Input) (Result, error) {
	cfg, ok := config.NATSConfigFromContext(ctx)
	if !ok {
		return Result{}, fmt.Errorf("missing NATS configuration in context")
	}

	session, err := natsclient.OpenJetStream(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	result := Result{
		URL:            session.URL,
		NATSReachable:  true,
		JetStreamReady: true,
	}

	streams, err := natsclient.EnsureStreams(session.Context, session.JetStream)
	if err != nil {
		return Result{}, err
	}
	result.Streams = streams

	kv, bucket, err := natsclient.EnsureJobsBucket(session.Context, session.JetStream)
	if err != nil {
		return Result{}, err
	}
	result.KeyValue = bucket

	for attempt := 0; attempt < maxTokenCollisionRetries; attempt++ {
		token := uuid.NewString()
		now := time.Now().UTC()

		job := natsclient.Job{
			Token:             token,
			Source:            in.Source,
			Destination:       in.Destination,
			DiscoverySubject:  natsclient.DiscoverySubject(token),
			WorkSubject:       natsclient.WorkSubject(token),
			MonitoringSubject: natsclient.MonitoringSubject(token),
			State:             natsclient.JobStatePending,
			CreatedAt:         now,
			UpdatedAt:         &now,
		}

		jobStatus, err := natsclient.SaveJob(session.Context, kv, job)
		if err == nil {
			result.Token = token
			result.Job = jobStatus
			return result, nil
		}
		if !errors.Is(err, jetstream.ErrKeyExists) {
			return Result{}, err
		}
	}

	return Result{}, fmt.Errorf("cannot allocate a unique sync token after %d attempts", maxTokenCollisionRetries)
}
