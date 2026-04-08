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
	NATS natsclient.PrepareResult
}

// NATSProvisioner is the dependency required to provision NATS resources.
type NATSProvisioner interface {
	Provision(ctx context.Context, cfg config.NATSConfig, source, destination string) (natsclient.PrepareResult, error)
}

// Service orchestrates the initial sync workflow.
type Service struct {
	NATSProvisioner NATSProvisioner
}

// New returns a sync service ready to provision NATS resources.
func New(natsProvisioner NATSProvisioner) Service {
	return Service{
		NATSProvisioner: natsProvisioner,
	}
}

// Provision prepares the required NATS resources using the resolved config in ctx.
func (s Service) Provision(ctx context.Context, in Input) (Result, error) {
	if s.NATSProvisioner == nil {
		return Result{}, fmt.Errorf("sync service is not configured")
	}

	cfg, ok := config.NATSConfigFromContext(ctx)
	if !ok {
		return Result{}, fmt.Errorf("missing NATS configuration in context")
	}

	prepared, err := s.NATSProvisioner.Provision(ctx, cfg, in.Source, in.Destination)
	if err != nil {
		return Result{}, err
	}

	return Result{
		NATS: prepared,
	}, nil
}
