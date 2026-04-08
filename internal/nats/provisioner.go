package natsclient

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nexus/nexus/internal/config"
)

// Provisioner provisions the NATS resources required by Nexus.
type Provisioner struct{}

// Provision ensures the required streams and KV bucket exist.
func (Provisioner) Provision(ctx context.Context, cfg config.NATSConfig) error {
	url := cfg.URL
	if url == "" {
		return fmt.Errorf("missing NATS server URL")
	}

	provisionTimeout := cfg.ProbeTimeout
	if provisionTimeout <= 0 {
		provisionTimeout = defaultProbeTimeout
	}

	provisionCtx := ctx
	connectOpts := []nats.Option{}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		provisionCtx, cancel = context.WithTimeout(ctx, provisionTimeout)
		defer cancel()

		connectOpts = append(connectOpts, nats.Timeout(provisionTimeout))
	}

	nc, err := nats.Connect(url, connectOpts...)
	if err != nil {
		return fmt.Errorf("cannot connect to NATS server %q: %w", url, err)
	}
	defer nc.Close()

	if err := nc.FlushWithContext(provisionCtx); err != nil {
		return fmt.Errorf("connected to NATS server %q but the provisioning connection is not usable: %w", url, err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("connected to NATS server %q but JetStream is unavailable: %w", url, err)
	}

	if err := EnsureStreams(provisionCtx, js); err != nil {
		return err
	}

	if err := EnsureBucket(js); err != nil {
		return err
	}

	return nil
}
