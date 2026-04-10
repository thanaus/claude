package lsservice

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	natsclient "github.com/nexus/nexus/internal/nats"
	"github.com/nexus/nexus/internal/config"
	internalfs "github.com/nexus/nexus/internal/fs"
	"github.com/nexus/nexus/internal/monitoring"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultPublisherCount = 4
	entryBufferSize  = 4096
	publishMaxRetries = 5
	publishRetryBase  = 100 * time.Millisecond
)

// Input holds the parameters required by the ls use case.
type Input struct {
	Token   string
	Sink    EventSink
	Workers int
}

// Snapshot describes the stable workflow state known before scan counters change.
type Snapshot struct {
	URL                string
	NATSReachable      bool
	JetStreamReady     bool
	KeyValue           natsclient.ResourceStatus
	Job                natsclient.Job
	DiscoveryPublished bool
}

// Result contains the outcome of the ls workflow.
type Result struct {
	Snapshot
	DiscoveredEntries uint64
	DiscoveredBytes   uint64
	PublishedWork     uint64
	Errors            uint64
}

// Progress reports intermediate scan counters while ls is running.
type Progress struct {
	DiscoveredEntries uint64
	DiscoveredBytes   uint64
	PublishedWork     uint64
	Errors            uint64
}

// Service orchestrates the ls workflow.
type Service struct{}

type scanStats struct {
	DiscoveredEntries uint64
	DiscoveredBytes   uint64
	PublishedWork     uint64
	Errors            uint64
}

// New returns an ls service ready to run the listing workflow.
func New() Service {
	return Service{}
}

// Run loads the job, seeds discovery, updates the job state, and publishes work items.
func (s Service) Run(ctx context.Context, in Input) (Result, error) {
	if in.Token == "" {
		return Result{}, fmt.Errorf("missing job token")
	}

	sink := in.Sink
	if sink == nil {
		sink = NoOpSink{}
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

	job, err := natsclient.LoadJob(session.Context, kv, in.Token)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Snapshot: Snapshot{
			URL:            session.URL,
			NATSReachable:  true,
			JetStreamReady: true,
			KeyValue:       bucket,
			Job:            job,
		},
	}

	startedAt := time.Now().UTC()
	discoveryMessage := natsclient.NewRootDiscoveryMessage(job.Source, startedAt)
	if err := natsclient.PublishJSON(session.Context, session.JetStream, job.DiscoverySubject, discoveryMessage); err != nil {
		return result, err
	}
	result.DiscoveryPublished = true

	job.State = natsclient.JobStateRunning
	job.StartedAt = &startedAt
	job.UpdatedAt = &startedAt
	job.DiscoveredEntries = 0
	job.DiscoveredBytes = 0
	job.PublishedWork = 0
	job.Errors = 0
	if _, err := natsclient.UpdateJob(session.Context, kv, job); err != nil {
		return result, err
	}
	result.Job = job

	sink.Emit(PreparedEvent{Snapshot: result.Snapshot})

	stats, scanErr := scanSource(session.Context, session.JetStream, job, sink)
	result.DiscoveredEntries = stats.DiscoveredEntries
	result.DiscoveredBytes = stats.DiscoveredBytes
	result.PublishedWork = stats.PublishedWork
	result.Errors = stats.Errors

	finishedAt := time.Now().UTC()
	job.DiscoveredEntries = stats.DiscoveredEntries
	job.DiscoveredBytes = stats.DiscoveredBytes
	job.PublishedWork = stats.PublishedWork
	job.Errors = stats.Errors
	job.UpdatedAt = &finishedAt
	if scanErr != nil {
		job.State = natsclient.JobStateFailed
	} else {
		job.State = natsclient.JobStateCompleted
	}

	if _, err := natsclient.UpdateJob(session.Context, kv, job); err != nil {
		if scanErr != nil {
			return result, fmt.Errorf("%w; additionally failed to persist final job state: %v", scanErr, err)
		}
		return result, err
	}

	if job.StartedAt != nil {
		monitoringMessage := natsclient.MonitoringMessage{
			Phase:             "scan",
			State:             job.State,
			StartedAt:         job.StartedAt.UTC(),
			UpdatedAt:         finishedAt,
			DiscoveredEntries: stats.DiscoveredEntries,
			DiscoveredBytes:   stats.DiscoveredBytes,
			PublishedWork:     stats.PublishedWork,
			Errors:            stats.Errors,
		}
		if err := natsclient.PublishJSON(session.Context, session.JetStream, job.MonitoringSubject, monitoringMessage); err != nil && scanErr == nil {
			return result, err
		}
	}

	result.Job = job
	if scanErr != nil {
		return result, scanErr
	}

	return result, nil
}

func scanSource(ctx context.Context, js jetstream.JetStream, job natsclient.Job, sink EventSink) (scanStats, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultsCh := make(chan natsclient.WorkMessage, entryBufferSize)

	var discoveredEntries atomic.Uint64
	var discoveredBytes atomic.Uint64
	var publishedWork atomic.Uint64
	var errorCount atomic.Uint64
	var firstErr error
	var firstErrOnce sync.Once
	var publishWG sync.WaitGroup
	var progressWG sync.WaitGroup
	progressDone := make(chan struct{})

	setFatalErr := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			errorCount.Add(1)
			cancel()
		})
	}

	reportProgress := func() {
		updatedAt := time.Now().UTC()
		progress := Progress{
			DiscoveredEntries: discoveredEntries.Load(),
			DiscoveredBytes:   discoveredBytes.Load(),
			PublishedWork:     publishedWork.Load(),
			Errors:            errorCount.Load(),
		}
		sink.Emit(ProgressEvent{Progress: progress})
		message := natsclient.MonitoringMessage{
			Phase:             "scan",
			State:             natsclient.JobStateRunning,
			StartedAt:         job.StartedAt.UTC(),
			UpdatedAt:         updatedAt,
			DiscoveredEntries: progress.DiscoveredEntries,
			DiscoveredBytes:   progress.DiscoveredBytes,
			PublishedWork:     progress.PublishedWork,
			Errors:            progress.Errors,
		}
		if err := natsclient.PublishJSON(scanCtx, js, job.MonitoringSubject, message); err != nil && scanCtx.Err() == nil {
			setFatalErr(err)
		}
	}

	progressWG.Add(1)
	go func() {
		defer progressWG.Done()

		ticker := time.NewTicker(monitoring.UpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-progressDone:
				reportProgress()
				return
			case <-scanCtx.Done():
				reportProgress()
				return
			case <-ticker.C:
				reportProgress()
			}
		}
	}()

	for range defaultPublisherCount {
		publishWG.Add(1)
		go func() {
			defer publishWG.Done()

			for message := range resultsCh {
				var publishErr error
				for attempt := range publishMaxRetries {
					if scanCtx.Err() != nil {
						return
					}

					if err := natsclient.PublishJSON(scanCtx, js, job.WorkSubject, message); err == nil {
						publishedWork.Add(1)
						publishErr = nil
						break
					} else {
						publishErr = err
					}

					if attempt < publishMaxRetries-1 {
						time.Sleep(time.Duration(1<<attempt) * publishRetryBase)
					}
				}
				if publishErr != nil {
					setFatalErr(publishErr)
					return
				}
			}
		}()
	}

	if _, scanErr := internalfs.Scan(scanCtx, job.Source, func(entry internalfs.Entry) error {
		discoveredEntries.Add(1)
		if entry.Size > 0 {
			discoveredBytes.Add(uint64(entry.Size))
		}

		message := buildWorkMessage(entry)
		select {
		case <-scanCtx.Done():
			return scanCtx.Err()
		case resultsCh <- message:
			return nil
		}
	}); scanErr != nil && !errors.Is(scanErr, context.Canceled) {
		setFatalErr(scanErr)
	}

	close(resultsCh)
	publishWG.Wait()
	close(progressDone)
	progressWG.Wait()

	stats := scanStats{
		DiscoveredEntries: discoveredEntries.Load(),
		DiscoveredBytes:   discoveredBytes.Load(),
		PublishedWork:     publishedWork.Load(),
		Errors:            errorCount.Load(),
	}
	if firstErr != nil {
		return stats, firstErr
	}

	return stats, nil
}

func buildWorkMessage(entry internalfs.Entry) natsclient.WorkMessage {
	return natsclient.WorkMessage{
		Path:      entry.Path,
		Name:      entry.Name,
		Type:      entryTypeToNATSType(entry.Type),
		Inode:     entry.Inode,
		Mode:      entry.Mode,
		Size:      entry.Size,
		CTime:     entry.CTime,
		MTime:     entry.MTime,
	}
}

func entryTypeToNATSType(t internalfs.EntryType) uint8 {
	switch {
	case t == internalfs.TypeFile:
		return natsclient.TypeFile
	case t == internalfs.TypeDir:
		return natsclient.TypeDir
	case t == internalfs.TypeSymlink:
		return natsclient.TypeSymlink
	case t == internalfs.TypeCharDev:
		return natsclient.TypeCharDev
	case t == internalfs.TypeDevice:
		return natsclient.TypeDevice
	case t == internalfs.TypePipe:
		return natsclient.TypePipe
	case t == internalfs.TypeSocket:
		return natsclient.TypeSocket
	default:
		return natsclient.TypeUnknown
	}
}

