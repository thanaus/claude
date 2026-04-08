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

// NATSProvisioner is the dependency required to provision NATS resources.
type NATSProvisioner interface {
	Provision(ctx context.Context, cfg config.NATSConfig) error
}

// Service orchestrates the initial sync workflow.
type Service struct {
	NATSProber      NATSProber
	NATSProvisioner NATSProvisioner
}

// New returns a sync service ready to verify and provision NATS.
func New(natsProber NATSProber, natsProvisioner NATSProvisioner) Service {
	return Service{
		NATSProber:      natsProber,
		NATSProvisioner: natsProvisioner,
	}
}

// Provision loads the NATS configuration, verifies the server, and provisions resources.
func (s Service) Provision(ctx context.Context, in Input) (Result, error) {
	if s.NATSProber == nil || s.NATSProvisioner == nil {
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

	if err := s.NATSProvisioner.Provision(ctx, cfg); err != nil {
		return Result{}, err
	}

	return Result{
		NATS: probe,
	}, nil
}
