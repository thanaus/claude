package config

import (
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
