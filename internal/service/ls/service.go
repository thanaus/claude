package lsservice

import (
	"context"
	"fmt"

	natsclient "github.com/nexus/nexus/internal/nats"
	"github.com/nexus/nexus/internal/config"
)

// Input holds the parameters required by the ls use case.
type Input struct {
	Token string
}

// Result contains the outcome of the ls preflight checks.
type Result struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
	KeyValue       natsclient.ResourceStatus
}

// Service orchestrates the ls workflow.
type Service struct{}

// New returns an ls service ready to validate NATS prerequisites.
func New() Service {
	return Service{}
}

// Run validates NATS, JetStream, and the jobs KV bucket before scanning starts.
func (s Service) Run(ctx context.Context, in Input) (Result, error) {
	if in.Token == "" {
		return Result{}, fmt.Errorf("missing job token")
	}

	cfg, ok := config.NATSConfigFromContext(ctx)
	if !ok {
		return Result{}, fmt.Errorf("missing NATS configuration in context")
	}

	session, err := natsclient.OpenJetStream(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	_, bucket, err := natsclient.LookupJobsBucket(session.Context, session.JetStream)
	if err != nil {
		return Result{}, err
	}

	return Result{
		URL:            session.URL,
		NATSReachable:  true,
		JetStreamReady: true,
		KeyValue:       bucket,
	}, nil
}
