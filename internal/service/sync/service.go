package syncservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

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
		token, err := newToken()
		if err != nil {
			return Result{}, fmt.Errorf("cannot generate sync token: %w", err)
		}

		job := natsclient.Job{
			Token:             token,
			Source:            in.Source,
			Destination:       in.Destination,
			DiscoverySubject:  natsclient.DiscoverySubject(token),
			WorkSubject:       natsclient.WorkSubject(token),
			MonitoringSubject: natsclient.MonitoringSubject(token),
			State:             "active",
			CreatedAt:         time.Now().UTC(),
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

func newToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
