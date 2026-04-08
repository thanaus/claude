package natsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const jobsBucketName = "sync_jobs"

// SyncJob stores the metadata required to operate a synchronization job.
type SyncJob struct {
	Token         string    `json:"token"`
	Source        string    `json:"source"`
	Destination   string    `json:"destination"`
	FilesSubject  string    `json:"filesSubject"`
	StatusSubject string  `json:"statusSubject"`
	State         string    `json:"state"`
	CreatedAt     time.Time `json:"createdAt"`
}

// EnsureBucket creates the KV bucket required to store sync job metadata.
func EnsureBucket(ctx context.Context, js jetstream.JetStream) (jetstream.KeyValue, ResourceStatus, error) {
	kv, err := js.KeyValue(ctx, jobsBucketName)
	switch {
	case err == nil:
		return kv, ResourceStatus{Name: jobsBucketName, Status: "ready"}, nil
	case errors.Is(err, jetstream.ErrBucketNotFound):
		kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:      jobsBucketName,
			Description: "Synchronization job metadata",
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

// SaveJob stores a sync job in the KV bucket.
func SaveJob(ctx context.Context, kv jetstream.KeyValue, job SyncJob) (ResourceStatus, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return ResourceStatus{}, fmt.Errorf("cannot encode sync job %q: %w", job.Token, err)
	}

	if _, err := kv.Create(ctx, job.Token, payload); err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return ResourceStatus{}, fmt.Errorf("sync job token %q already exists: %w", job.Token, err)
		}
		return ResourceStatus{}, fmt.Errorf("cannot store sync job %q: %w", job.Token, err)
	}

	return ResourceStatus{Name: job.Token, Status: "created"}, nil
}
