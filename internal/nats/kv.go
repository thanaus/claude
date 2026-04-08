package natsclient

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

const jobsBucketName = "sync_jobs"

// EnsureBucket creates the KV bucket required to store sync job metadata.
func EnsureBucket(js nats.JetStreamContext) error {
	_, err := js.KeyValue(jobsBucketName)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, nats.ErrBucketNotFound):
		if _, err := js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:      jobsBucketName,
			Description: "Synchronization job metadata",
			Storage:     nats.FileStorage,
			Replicas:    1,
		}); err != nil {
			return fmt.Errorf("cannot create KV bucket %q: %w", jobsBucketName, err)
		}
		return nil
	default:
		return fmt.Errorf("cannot inspect KV bucket %q: %w", jobsBucketName, err)
	}
}
