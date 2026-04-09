package natsclient

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/nexus/nexus/internal/config"
)

const maxTokenCollisionRetries = 3

// PrepareResult reports the NATS readiness and the resources prepared for Nexus.
type PrepareResult struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
	Token          string
	Streams        []ResourceStatus
	KeyValue       ResourceStatus
	Job            ResourceStatus
}

// Provisioner provisions the NATS resources required by Nexus.
type Provisioner struct{}

// Provision ensures the required streams and KV bucket exist and reports the outcome.
func (Provisioner) Provision(ctx context.Context, cfg config.NATSConfig, source, destination string) (PrepareResult, error) {
	url := strings.TrimSpace(cfg.URL)
	nc, provisionCtx, cancel, err := connect(ctx, cfg)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("cannot connect to NATS server %q: %w", url, err)
	}
	if cancel != nil {
		defer cancel()
	}
	defer nc.Close()

	if err := nc.FlushWithContext(provisionCtx); err != nil {
		return PrepareResult{}, fmt.Errorf("connected to NATS server %q but the provisioning connection is not usable: %w", url, err)
	}

	if !nc.IsConnected() {
		return PrepareResult{}, fmt.Errorf("connection to NATS server %q is not ready", url)
	}

	result := PrepareResult{
		URL:           url,
		NATSReachable: true,
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return result, fmt.Errorf("connected to NATS server %q but JetStream is unavailable: %w", url, err)
	}

	if _, err := js.AccountInfo(provisionCtx); err != nil {
		return result, fmt.Errorf("connected to NATS server %q but JetStream is not responding: %w", url, err)
	}
	result.JetStreamReady = true

	streams, err := EnsureStreams(provisionCtx, js)
	if err != nil {
		return result, err
	}
	result.Streams = streams

	kv, bucket, err := EnsureJobsBucket(provisionCtx, js)
	if err != nil {
		return result, err
	}
	result.KeyValue = bucket

	for attempt := 0; attempt < maxTokenCollisionRetries; attempt++ {
		token, err := newToken()
		if err != nil {
			return result, fmt.Errorf("cannot generate sync token: %w", err)
		}

		job := Job{
			Token:             token,
			Source:            source,
			Destination:       destination,
			DiscoverySubject:  DiscoverySubject(token),
			WorkSubject:       WorkSubject(token),
			MonitoringSubject: MonitoringSubject(token),
			State:             "active",
			CreatedAt:         time.Now().UTC(),
		}

		jobStatus, err := SaveJob(provisionCtx, kv, job)
		if err == nil {
			result.Token = token
			result.Job = jobStatus
			return result, nil
		}
		if !errors.Is(err, jetstream.ErrKeyExists) {
			return result, err
		}
	}

	return result, fmt.Errorf("cannot allocate a unique sync token after %d attempts", maxTokenCollisionRetries)
}

func newToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
