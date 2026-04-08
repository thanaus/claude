package natsclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nexus/nexus/internal/config"
)

// ProbeResult reports the current NATS and JetStream readiness.
type ProbeResult struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
}

// Client probes a NATS server and validates that it can be used.
type Client struct{}

// Probe connects to NATS, verifies the server answers, and checks JetStream.
func (Client) Probe(ctx context.Context, cfg config.NATSConfig) (ProbeResult, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return ProbeResult{}, fmt.Errorf("missing NATS server URL")
	}

	nc, err := nats.Connect(url)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("cannot connect to NATS server %q: %w", url, err)
	}
	defer nc.Close()

	if err := nc.FlushWithContext(ctx); err != nil {
		return ProbeResult{}, fmt.Errorf("connected to NATS server %q but the connection is not usable: %w", url, err)
	}

	if !nc.IsConnected() {
		return ProbeResult{}, fmt.Errorf("connection to NATS server %q is not ready", url)
	}

	result := ProbeResult{
		URL:           url,
		NATSReachable: true,
	}

	js, err := nc.JetStream()
	if err != nil {
		return result, fmt.Errorf("connected to NATS server %q but JetStream is unavailable: %w", url, err)
	}

	if _, err := js.AccountInfo(); err != nil {
		return result, fmt.Errorf("connected to NATS server %q but JetStream is not responding: %w", url, err)
	}

	result.JetStreamReady = true

	return result, nil
}
