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

const (
	JobStatePending   = "pending"
	JobStateRunning   = "running"
	JobStateCompleted = "completed"
	JobStateFailed    = "failed"
)

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
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	UpdatedAt         *time.Time `json:"updatedAt,omitempty"`
	DiscoveredEntries uint64    `json:"discoveredEntries,omitempty"`
	DiscoveredBytes   uint64    `json:"discoveredBytes,omitempty"`
	PublishedWork     uint64    `json:"publishedWork,omitempty"`
	WorkerProcessed   uint64    `json:"workerProcessed,omitempty"`
	WorkerToCopy      uint64    `json:"workerToCopy,omitempty"`
	WorkerCopyMissing uint64    `json:"workerCopyMissing,omitempty"`
	WorkerCopySize    uint64    `json:"workerCopySize,omitempty"`
	WorkerCopyMTime   uint64    `json:"workerCopyMtime,omitempty"`
	WorkerCopyCTime   uint64    `json:"workerCopyCtime,omitempty"`
	WorkerOK          uint64    `json:"workerOK,omitempty"`
	WorkerErrors      uint64    `json:"workerErrors,omitempty"`
	WorkerLStatNanos  uint64    `json:"workerLstatNanos,omitempty"`
	WorkerCopyNanos   uint64    `json:"workerCopyNanos,omitempty"`
	Errors            uint64    `json:"errors,omitempty"`
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

// LoadJob retrieves a job from the KV bucket.
func LoadJob(ctx context.Context, kv jetstream.KeyValue, token string) (Job, error) {
	entry, err := kv.Get(ctx, token)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return Job{}, fmt.Errorf("job token %q not found: %w", token, err)
		}
		return Job{}, fmt.Errorf("cannot load job %q: %w", token, err)
	}

	var job Job
	if err := json.Unmarshal(entry.Value(), &job); err != nil {
		return Job{}, fmt.Errorf("cannot decode job %q: %w", token, err)
	}
	if job.Token == "" {
		job.Token = token
	}

	return job, nil
}

// UpdateJob overwrites an existing job in the KV bucket.
func UpdateJob(ctx context.Context, kv jetstream.KeyValue, job Job) (ResourceStatus, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return ResourceStatus{}, fmt.Errorf("cannot encode job %q: %w", job.Token, err)
	}

	if _, err := kv.Put(ctx, job.Token, payload); err != nil {
		return ResourceStatus{}, fmt.Errorf("cannot update job %q: %w", job.Token, err)
	}

	return ResourceStatus{Name: job.Token, Status: "updated"}, nil
}
