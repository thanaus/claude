package syncservice

import (
	"context"
	"fmt"

	"github.com/nexus/nexus/internal/config"
	natsclient "github.com/nexus/nexus/internal/nats"
)

// Input holds the parameters required by the sync use case.
type Input struct {
	Source      string
	Destination string
}

// Result contains the outcome of the initial sync checks.
type Result struct {
	NATS natsclient.ProbeResult
}

// NATSProber is the dependency required by the sync service.
type NATSProber interface {
	Probe(ctx context.Context, cfg config.NATSConfig) (natsclient.ProbeResult, error)
}

// Service orchestrates the initial sync workflow.
type Service struct {
	NATSProber NATSProber
}

// New returns a sync service ready to probe NATS.
func New(natsProber NATSProber) Service {
	return Service{
		NATSProber: natsProber,
	}
}

// CheckNATS loads the NATS configuration and verifies the server is usable.
func (s Service) CheckNATS(ctx context.Context, in Input) (Result, error) {
	if s.NATSProber == nil {
		return Result{}, fmt.Errorf("sync service is not configured")
	}

	cfg, err := config.LoadNATSFromEnv()
	if err != nil {
		return Result{}, err
	}

	probe, err := s.NATSProber.Probe(ctx, cfg)
	if err != nil {
		return Result{}, err
	}

	return Result{
		NATS: probe,
	}, nil
}
