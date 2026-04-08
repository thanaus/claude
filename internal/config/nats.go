package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/nexus/nexus/internal/app"
)

// NATSConfig holds the minimal configuration required to reach NATS.
type NATSConfig struct {
	URL string
}

// LoadNATSFromEnv loads the NATS configuration from environment variables.
func LoadNATSFromEnv() (NATSConfig, error) {
	url := strings.TrimSpace(os.Getenv(app.NATSURLEnv))
	if url == "" {
		return NATSConfig{}, fmt.Errorf("missing required environment variable: %s", app.NATSURLEnv)
	}

	return NATSConfig{
		URL: url,
	}, nil
}
