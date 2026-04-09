package natsclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nexus/nexus/internal/config"
)

const defaultProbeTimeout = 5 * time.Second

// ProbeResult reports the current NATS and JetStream readiness.
type ProbeResult struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
}

// JetStreamSession wraps a ready-to-use NATS connection and JetStream handle.
type JetStreamSession struct {
	URL       string
	Conn      *nats.Conn
	JetStream jetstream.JetStream
	Context   context.Context
	cancel    context.CancelFunc
}

// Client probes a NATS server and validates that it can be used.
type Client struct{}

// OpenJetStream connects to NATS and returns a ready-to-use JetStream session.
func OpenJetStream(ctx context.Context, cfg config.NATSConfig) (*JetStreamSession, error) {
	url := strings.TrimSpace(cfg.URL)
	nc, opCtx, cancel, err := connect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to NATS server %q: %w", url, err)
	}

	if err := nc.FlushWithContext(opCtx); err != nil {
		nc.Close()
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("connected to NATS server %q but the connection is not usable: %w", url, err)
	}

	if !nc.IsConnected() {
		nc.Close()
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("connection to NATS server %q is not ready", url)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("connected to NATS server %q but JetStream is unavailable: %w", url, err)
	}

	if _, err := js.AccountInfo(opCtx); err != nil {
		nc.Close()
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("connected to NATS server %q but JetStream is not responding: %w", url, err)
	}

	return &JetStreamSession{
		URL:       url,
		Conn:      nc,
		JetStream: js,
		Context:   opCtx,
		cancel:    cancel,
	}, nil
}

// Close releases the resources owned by the JetStream session.
func (s *JetStreamSession) Close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.Conn != nil {
		s.Conn.Close()
	}
}

// Probe connects to NATS, verifies the server answers, and checks JetStream.
func (Client) Probe(ctx context.Context, cfg config.NATSConfig) (ProbeResult, error) {
	session, err := OpenJetStream(ctx, cfg)
	if err != nil {
		return ProbeResult{}, err
	}
	defer session.Close()

	return ProbeResult{
		URL:           session.URL,
		NATSReachable: true,
		JetStreamReady: true,
	}, nil
}

func connect(ctx context.Context, cfg config.NATSConfig) (*nats.Conn, context.Context, context.CancelFunc, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, nil, nil, fmt.Errorf("missing NATS server URL")
	}

	timeout := cfg.ProbeTimeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}

	opCtx := ctx
	var cancel context.CancelFunc
	connectOpts := []nats.Option{}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		opCtx, cancel = context.WithTimeout(ctx, timeout)
		connectOpts = append(connectOpts, nats.Timeout(timeout))
	}

	nc, err := nats.Connect(url, connectOpts...)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, nil, nil, err
	}

	return nc, opCtx, cancel, nil
}
