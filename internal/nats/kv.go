package natsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const jobsBucketName = "jobs"

// Job stores the metadata required to operate a job.
type Job struct {
	Token             string    `json:"token"`
	Source            string    `json:"source"`
	Destination       string    `json:"destination"`
	DiscoverySubject  string    `json:"discoverySubject"`
	WorkSubject       string    `json:"workSubject"`
	MonitoringSubject string    `json:"monitoringSubject"`
	State             string    `json:"state"`
	CreatedAt         time.Time `json:"createdAt"`
}

// EnsureJobsBucket creates the KV bucket required to store job metadata.
func EnsureJobsBucket(ctx context.Context, js jetstream.JetStream) (jetstream.KeyValue, ResourceStatus, error) {
	kv, err := js.KeyValue(ctx, jobsBucketName)
	switch {
	case err == nil:
		return kv, ResourceStatus{Name: jobsBucketName, Status: "ready"}, nil
	case errors.Is(err, jetstream.ErrBucketNotFound):
		kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:      jobsBucketName,
			Description: "Job metadata",
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		})
		if err != nil {
			return nil, ResourceStatus{}, fmt.Errorf("cannot create KV bucket %q: %w", jobsBucketName, err)
		}
		return kv, ResourceStatus{Name: jobsBucketName, Status: "created"}, nil
	default:
		return nil, ResourceStatus{}, fmt.Errorf("cannot inspect KV bucket %q: %w", jobsBucketName, err)
	}
}

// LookupJobsBucket checks that the jobs KV bucket already exists.
func LookupJobsBucket(ctx context.Context, js jetstream.JetStream) (jetstream.KeyValue, ResourceStatus, error) {
	kv, err := js.KeyValue(ctx, jobsBucketName)
	switch {
	case err == nil:
		return kv, ResourceStatus{Name: jobsBucketName, Status: "ready"}, nil
	case errors.Is(err, jetstream.ErrBucketNotFound):
		return nil, ResourceStatus{}, fmt.Errorf("job KV bucket %q not found", jobsBucketName)
	default:
		return nil, ResourceStatus{}, fmt.Errorf("cannot inspect KV bucket %q: %w", jobsBucketName, err)
	}
}

// SaveJob stores a job in the KV bucket.
func SaveJob(ctx context.Context, kv jetstream.KeyValue, job Job) (ResourceStatus, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return ResourceStatus{}, fmt.Errorf("cannot encode job %q: %w", job.Token, err)
	}

	if _, err := kv.Create(ctx, job.Token, payload); err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return ResourceStatus{}, fmt.Errorf("job token %q already exists: %w", job.Token, err)
		}
		return ResourceStatus{}, fmt.Errorf("cannot store job %q: %w", job.Token, err)
	}

	return ResourceStatus{Name: job.Token, Status: "created"}, nil
}
