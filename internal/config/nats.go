package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nexus/nexus/internal/app"
)

// NATSConfig holds the minimal configuration required to reach NATS.
type NATSConfig struct {
	URL          string
	ProbeTimeout time.Duration
}

type natsConfigContextKey struct{}

// WithNATSConfig stores a resolved NATS configuration in the context.
func WithNATSConfig(ctx context.Context, cfg NATSConfig) context.Context {
	return context.WithValue(ctx, natsConfigContextKey{}, cfg)
}

// NATSConfigFromContext returns the resolved NATS configuration from the context.
func NATSConfigFromContext(ctx context.Context) (NATSConfig, bool) {
	cfg, ok := ctx.Value(natsConfigContextKey{}).(NATSConfig)
	return cfg, ok
}

// LoadNATSFromEnv loads the NATS configuration from environment variables.
func LoadNATSFromEnv() (NATSConfig, error) {
	url := strings.TrimSpace(os.Getenv(app.NATSURLEnv))
	if url == "" {
		return NATSConfig{}, fmt.Errorf("missing required environment variable: %s", app.NATSURLEnv)
	}

	probeTimeoutValue := strings.TrimSpace(os.Getenv(app.NATSProbeTimeoutEnv))
	var probeTimeout time.Duration
	if probeTimeoutValue != "" {
		parsed, err := time.ParseDuration(probeTimeoutValue)
		if err != nil {
			return NATSConfig{}, fmt.Errorf("invalid %s value %q: %w", app.NATSProbeTimeoutEnv, probeTimeoutValue, err)
		}
		if parsed <= 0 {
			return NATSConfig{}, fmt.Errorf("invalid %s value %q: duration must be greater than zero", app.NATSProbeTimeoutEnv, probeTimeoutValue)
		}
		probeTimeout = parsed
	}

	return NATSConfig{
		URL:          url,
		ProbeTimeout: probeTimeout,
	}, nil
}
