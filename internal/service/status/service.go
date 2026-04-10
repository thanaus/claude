package statusservice

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus/nexus/internal/config"
	natsclient "github.com/nexus/nexus/internal/nats"
)

// Metrics contains the derived scan metrics displayed by the status command.
type Metrics struct {
	Elapsed          time.Duration
	Idle             time.Duration
	Backlog          uint64
	PublishRate      float64
	DiscoveryRate    float64
	WorkerRate       float64
	WorkerInstantRate float64
	WorkerInstantRateAvailable bool
	ErrorRate        float64
	PublishedPercent float64
	DiscoveredBytes  uint64
}

// Result contains the persisted job state and derived status metrics.
type Result struct {
	URL            string
	NATSReachable  bool
	JetStreamReady bool
	KeyValue       natsclient.ResourceStatus
	Job            natsclient.Job
	MonitoringPhase string
	CollectedAt    time.Time
	Metrics        Metrics
	lastWorkerUpdatedAt *time.Time
	lastWorkerProcessed uint64
}

// Service loads persisted job state and computes derived metrics for display.
type Service struct{}

// New returns a status service ready to inspect jobs.
func New() Service {
	return Service{}
}

// Load returns the current persisted job state and the derived scan metrics.
func (s Service) Load(ctx context.Context, token string, now time.Time) (Result, error) {
	if token == "" {
		return Result{}, fmt.Errorf("missing job token")
	}

	cfg, ok := config.NATSConfigFromContext(ctx)
	if !ok {
		return Result{}, fmt.Errorf("missing NATS configuration in context")
	}

	session, err := natsclient.OpenJetStream(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	kv, bucket, err := natsclient.LookupJobsBucket(session.Context, session.JetStream)
	if err != nil {
		return Result{}, err
	}

	job, err := natsclient.LoadJob(session.Context, kv, token)
	if err != nil {
		return Result{}, err
	}

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	return Result{
		URL:            session.URL,
		NATSReachable:  true,
		JetStreamReady: true,
		KeyValue:       bucket,
		Job:            job,
		CollectedAt:    now,
		Metrics:        deriveMetrics(job, now),
	}, nil
}

// Refresh recalculates the derived metrics for an existing status snapshot.
func Refresh(result Result, now time.Time) Result {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	previousMetrics := result.Metrics
	result.CollectedAt = now
	result.Metrics = deriveMetrics(result.Job, now)
	result.Metrics.WorkerInstantRate = previousMetrics.WorkerInstantRate
	result.Metrics.WorkerInstantRateAvailable = previousMetrics.WorkerInstantRateAvailable
	return result
}

// ApplyMonitoring overlays a live monitoring event onto a persisted status snapshot.
func ApplyMonitoring(result Result, update natsclient.MonitoringMessage, now time.Time) Result {
	startedAt := update.StartedAt.UTC()
	updatedAt := update.UpdatedAt.UTC()

	result.MonitoringPhase = update.Phase
	result.Job.State = update.State
	result.Job.StartedAt = &startedAt
	result.Job.UpdatedAt = &updatedAt

	switch update.Phase {
	case "worker":
		if update.DiscoveredEntries > 0 {
			result.Job.DiscoveredEntries = update.DiscoveredEntries
		}
		if update.DiscoveredBytes > 0 {
			result.Job.DiscoveredBytes = update.DiscoveredBytes
		}
		if update.PublishedWork > 0 {
			result.Job.PublishedWork = update.PublishedWork
		}
		if hasWorkerDelta(update) {
			result.Job.WorkerProcessed += update.WorkerProcessedDelta
			result.Job.WorkerToCopy += update.WorkerToCopyDelta
			result.Job.WorkerCopyMissing += update.WorkerCopyMissingDelta
			result.Job.WorkerCopySize += update.WorkerCopySizeDelta
			result.Job.WorkerCopyMTime += update.WorkerCopyMTimeDelta
			result.Job.WorkerCopyCTime += update.WorkerCopyCTimeDelta
			result.Job.WorkerOK += update.WorkerOKDelta
			result.Job.WorkerErrors += update.WorkerErrorsDelta
			result.Job.WorkerLStatNanos += update.WorkerLStatNanosDelta
			result.Job.WorkerCopyNanos += update.WorkerCopyNanosDelta
		} else {
			result.Job.WorkerProcessed = update.WorkerProcessed
			result.Job.WorkerToCopy = update.WorkerToCopy
			result.Job.WorkerCopyMissing = update.WorkerCopyMissing
			result.Job.WorkerCopySize = update.WorkerCopySize
			result.Job.WorkerCopyMTime = update.WorkerCopyMTime
			result.Job.WorkerCopyCTime = update.WorkerCopyCTime
			result.Job.WorkerOK = update.WorkerOK
			result.Job.WorkerErrors = update.WorkerErrors
			result.Job.WorkerLStatNanos = update.WorkerLStatNanos
			result.Job.WorkerCopyNanos = update.WorkerCopyNanos
		}
		result.Job.Errors = update.Errors
	case "scan":
		result.Job.DiscoveredEntries = update.DiscoveredEntries
		result.Job.DiscoveredBytes = update.DiscoveredBytes
		result.Job.PublishedWork = update.PublishedWork
		result.Job.Errors = update.Errors
	default:
		result.Job.DiscoveredEntries = update.DiscoveredEntries
		result.Job.DiscoveredBytes = update.DiscoveredBytes
		result.Job.PublishedWork = update.PublishedWork
		result.Job.Errors = update.Errors
		if hasWorkerTotals(update) {
			result.Job.WorkerProcessed = update.WorkerProcessed
			result.Job.WorkerToCopy = update.WorkerToCopy
			result.Job.WorkerCopyMissing = update.WorkerCopyMissing
			result.Job.WorkerCopySize = update.WorkerCopySize
			result.Job.WorkerCopyMTime = update.WorkerCopyMTime
			result.Job.WorkerCopyCTime = update.WorkerCopyCTime
			result.Job.WorkerOK = update.WorkerOK
			result.Job.WorkerErrors = update.WorkerErrors
			result.Job.WorkerLStatNanos = update.WorkerLStatNanos
			result.Job.WorkerCopyNanos = update.WorkerCopyNanos
		}
	}

	result = Refresh(result, now)
	if update.Phase == "worker" {
		if rate, ok := deriveInstantWorkerRate(result.lastWorkerUpdatedAt, updatedAt, result.lastWorkerProcessed, result.Job.WorkerProcessed, update); ok {
			result.Metrics.WorkerInstantRate = rate
			result.Metrics.WorkerInstantRateAvailable = true
		}
		result.lastWorkerUpdatedAt = &updatedAt
		result.lastWorkerProcessed = result.Job.WorkerProcessed
	}
	return result
}

func deriveMetrics(job natsclient.Job, now time.Time) Metrics {
	metrics := Metrics{
		Backlog:         saturatingSub(job.DiscoveredEntries, job.PublishedWork),
		DiscoveredBytes: job.DiscoveredBytes,
	}

	if job.UpdatedAt != nil {
		metrics.Idle = nonNegativeDuration(now.Sub(job.UpdatedAt.UTC()))
	}

	if job.StartedAt == nil {
		return metrics
	}

	endTime := now
	if isTerminalState(job.State) && job.UpdatedAt != nil {
		endTime = job.UpdatedAt.UTC()
	}

	elapsed := nonNegativeDuration(endTime.Sub(job.StartedAt.UTC()))
	metrics.Elapsed = elapsed

	if seconds := elapsed.Seconds(); seconds > 0 {
		metrics.PublishRate = float64(job.PublishedWork) / seconds
		metrics.DiscoveryRate = float64(job.DiscoveredEntries) / seconds
		metrics.WorkerRate = float64(job.WorkerProcessed) / seconds
	}

	if job.DiscoveredEntries > 0 {
		metrics.ErrorRate = float64(job.Errors) / float64(job.DiscoveredEntries)
		metrics.PublishedPercent = float64(job.PublishedWork) / float64(job.DiscoveredEntries)
	}

	return metrics
}

func deriveInstantWorkerRate(previousUpdatedAt *time.Time, updatedAt time.Time, previousProcessed, currentProcessed uint64, update natsclient.MonitoringMessage) (float64, bool) {
	if previousUpdatedAt == nil {
		return 0, false
	}

	elapsed := updatedAt.Sub(previousUpdatedAt.UTC())
	if elapsed <= 0 {
		return 0, false
	}

	processedDelta := update.WorkerProcessedDelta
	if processedDelta == 0 {
		if currentProcessed < previousProcessed {
			return 0, false
		}
		processedDelta = currentProcessed - previousProcessed
	}

	return float64(processedDelta) / elapsed.Seconds(), true
}

func hasWorkerDelta(update natsclient.MonitoringMessage) bool {
	return update.WorkerProcessedDelta > 0 ||
		update.WorkerToCopyDelta > 0 ||
		update.WorkerCopyMissingDelta > 0 ||
		update.WorkerCopySizeDelta > 0 ||
		update.WorkerCopyMTimeDelta > 0 ||
		update.WorkerCopyCTimeDelta > 0 ||
		update.WorkerOKDelta > 0 ||
		update.WorkerErrorsDelta > 0 ||
		update.WorkerLStatNanosDelta > 0 ||
		update.WorkerCopyNanosDelta > 0
}

func hasWorkerTotals(update natsclient.MonitoringMessage) bool {
	return update.WorkerProcessed > 0 ||
		update.WorkerToCopy > 0 ||
		update.WorkerCopyMissing > 0 ||
		update.WorkerCopySize > 0 ||
		update.WorkerCopyMTime > 0 ||
		update.WorkerCopyCTime > 0 ||
		update.WorkerOK > 0 ||
		update.WorkerErrors > 0 ||
		update.WorkerLStatNanos > 0 ||
		update.WorkerCopyNanos > 0
}

func isTerminalState(state string) bool {
	return state == natsclient.JobStateCompleted || state == natsclient.JobStateFailed
}

func nonNegativeDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}

	return d
}

func saturatingSub(left, right uint64) uint64 {
	if right >= left {
		return 0
	}

	return left - right
}
