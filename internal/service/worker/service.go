package workerservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/nexus/nexus/internal/config"
	internalfs "github.com/nexus/nexus/internal/fs"
	"github.com/nexus/nexus/internal/monitoring"
	natsclient "github.com/nexus/nexus/internal/nats"
)

const (
	defaultWorkerCount   = 4
	workerFetchBatchSize = 256
	workerFetchWait      = time.Second
	workerConsumerPrefix = "worker"
	workerBufferSize     = 256
	workerKVUpdateRetries = 5
)

// Input holds the parameters required by the worker use case.
type Input struct {
	Token   string
	Workers int
}

// Result contains the outcome of the worker comparison workflow.
type Result struct {
	Job             natsclient.Job
	WorkerProcessed uint64
	WorkerToCopy    uint64
	WorkerCopyMissing uint64
	WorkerCopySize    uint64
	WorkerCopyMTime   uint64
	WorkerCopyCTime   uint64
	WorkerOK        uint64
	WorkerErrors    uint64
	WorkerLStatNanos uint64
	WorkerCopyNanos  uint64
}

// Service orchestrates the worker workflow.
type Service struct{}

// New returns a worker service ready to compare destination files.
func New() Service {
	return Service{}
}

// Run consumes WORK messages for a job and compares them against destination metadata.
func (s Service) Run(ctx context.Context, in Input) (Result, error) {
	if in.Token == "" {
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

	kv, _, err := natsclient.LookupJobsBucket(session.Context, session.JetStream)
	if err != nil {
		return Result{}, err
	}

	job, err := natsclient.LoadJob(session.Context, kv, in.Token)
	if err != nil {
		return Result{}, err
	}

	workers := in.Workers
	if workers <= 0 {
		workers = defaultWorkerCount
	}

	stream, err := session.JetStream.Stream(session.Context, natsclient.WorkStreamName)
	if err != nil {
		return Result{}, fmt.Errorf("cannot inspect stream %q: %w", natsclient.WorkStreamName, err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(session.Context, jetstream.ConsumerConfig{
		Durable:       workerConsumerName(job.Token),
		FilterSubject: job.WorkSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return Result{}, fmt.Errorf("cannot create worker consumer for subject %q: %w", job.WorkSubject, err)
	}

	type workItem struct {
		msg  jetstream.Msg
		work natsclient.WorkMessage
	}

	workCh := make(chan workItem, workerBufferSize)
	workerCtx, cancel := context.WithCancel(session.Context)
	defer cancel()

	var processedCount atomic.Uint64
	var toCopyCount atomic.Uint64
	var toCopyMissingCount atomic.Uint64
	var toCopySizeCount atomic.Uint64
	var toCopyMTimeCount atomic.Uint64
	var toCopyCTimeCount atomic.Uint64
	var okCount atomic.Uint64
	var workerErrorCount atomic.Uint64
	var workerLStatNanos atomic.Uint64
	var workerCopyNanos atomic.Uint64
	var firstErr error
	var firstErrOnce sync.Once
	var fetchWG sync.WaitGroup
	var compareWG sync.WaitGroup
	var progressWG sync.WaitGroup
	progressDone := make(chan struct{})
	var lastPublishedProcessed uint64
	var lastPublishedToCopy uint64
	var lastPublishedToCopyMissing uint64
	var lastPublishedToCopySize uint64
	var lastPublishedToCopyMTime uint64
	var lastPublishedToCopyCTime uint64
	var lastPublishedOK uint64
	var lastPublishedErrors uint64
	var lastPublishedLStatNanos uint64
	var lastPublishedCopyNanos uint64

	setFatalErr := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	currentResult := func() Result {
		return Result{
			Job:             job,
			WorkerProcessed: processedCount.Load(),
			WorkerToCopy:    toCopyCount.Load(),
			WorkerCopyMissing: toCopyMissingCount.Load(),
			WorkerCopySize:    toCopySizeCount.Load(),
			WorkerCopyMTime:   toCopyMTimeCount.Load(),
			WorkerCopyCTime:   toCopyCTimeCount.Load(),
			WorkerOK:        okCount.Load(),
			WorkerErrors:    workerErrorCount.Load(),
			WorkerLStatNanos: workerLStatNanos.Load(),
			WorkerCopyNanos:  workerCopyNanos.Load(),
		}
	}

	reportProgress := func() {
		current := currentResult()
		delta := Result{
			Job:             job,
			WorkerProcessed: current.WorkerProcessed - lastPublishedProcessed,
			WorkerToCopy:    current.WorkerToCopy - lastPublishedToCopy,
			WorkerCopyMissing: current.WorkerCopyMissing - lastPublishedToCopyMissing,
			WorkerCopySize:    current.WorkerCopySize - lastPublishedToCopySize,
			WorkerCopyMTime:   current.WorkerCopyMTime - lastPublishedToCopyMTime,
			WorkerCopyCTime:   current.WorkerCopyCTime - lastPublishedToCopyCTime,
			WorkerOK:        current.WorkerOK - lastPublishedOK,
			WorkerErrors:    current.WorkerErrors - lastPublishedErrors,
			WorkerLStatNanos: current.WorkerLStatNanos - lastPublishedLStatNanos,
			WorkerCopyNanos:  current.WorkerCopyNanos - lastPublishedCopyNanos,
		}
		if delta.WorkerProcessed == 0 &&
			delta.WorkerToCopy == 0 &&
			delta.WorkerCopyMissing == 0 &&
			delta.WorkerCopySize == 0 &&
			delta.WorkerCopyMTime == 0 &&
			delta.WorkerCopyCTime == 0 &&
			delta.WorkerOK == 0 &&
			delta.WorkerErrors == 0 &&
			delta.WorkerLStatNanos == 0 &&
			delta.WorkerCopyNanos == 0 {
			return
		}
		lastPublishedProcessed = current.WorkerProcessed
		lastPublishedToCopy = current.WorkerToCopy
		lastPublishedToCopyMissing = current.WorkerCopyMissing
		lastPublishedToCopySize = current.WorkerCopySize
		lastPublishedToCopyMTime = current.WorkerCopyMTime
		lastPublishedToCopyCTime = current.WorkerCopyCTime
		lastPublishedOK = current.WorkerOK
		lastPublishedErrors = current.WorkerErrors
		lastPublishedLStatNanos = current.WorkerLStatNanos
		lastPublishedCopyNanos = current.WorkerCopyNanos

		if err := publishWorkerMonitoring(workerCtx, session.JetStream, job, delta); err != nil && workerCtx.Err() == nil {
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
			case <-workerCtx.Done():
				reportProgress()
				return
			case <-ticker.C:
				reportProgress()
			}
		}
	}()

	fetchWG.Add(1)
	go func() {
		defer fetchWG.Done()
		defer close(workCh)

		for {
			batch, err := consumer.Fetch(workerFetchBatchSize, jetstream.FetchMaxWait(workerFetchWait))
			if err != nil {
				switch {
				case workerCtx.Err() != nil:
					return
				case isFetchTimeout(err):
					latestJob, loadErr := natsclient.LoadJob(session.Context, kv, job.Token)
					if loadErr != nil {
						setFatalErr(loadErr)
						return
					}
					if isTerminalWorkerInput(latestJob.State) {
						return
					}
					continue
				default:
					setFatalErr(err)
					return
				}
			}

			for msg := range batch.Messages() {
				var work natsclient.WorkMessage
				if err := json.Unmarshal(msg.Data(), &work); err != nil {
					setFatalErr(fmt.Errorf("cannot decode work message for job %q: %w", job.Token, err))
					return
				}

				select {
				case <-workerCtx.Done():
					return
				case workCh <- workItem{msg: msg, work: work}:
				}
			}

			if err := batch.Error(); err != nil {
				switch {
				case workerCtx.Err() != nil:
					return
				case isFetchTimeout(err):
					latestJob, loadErr := natsclient.LoadJob(session.Context, kv, job.Token)
					if loadErr != nil {
						setFatalErr(loadErr)
						return
					}
					if isTerminalWorkerInput(latestJob.State) {
						return
					}
				default:
					setFatalErr(err)
					return
				}
			}
		}
	}()

	for range workers {
		compareWG.Add(1)
		go func() {
			defer compareWG.Done()

			for {
				select {
				case <-workerCtx.Done():
					return
				case item, ok := <-workCh:
					if !ok {
						return
					}

					entry := workEntry(item.work)
					lstatStartedAt := time.Now()
					classification, compareErr := internalfs.CompareDestination(job.Destination, entry)
					workerLStatNanos.Add(uint64(time.Since(lstatStartedAt)))
					processedCount.Add(1)

					switch {
					case compareErr == nil:
						switch classification {
						case internalfs.ComparisonMissing:
							copyStartedAt := time.Now()
							compareErr = internalfs.CopyEntry(job.Source, job.Destination, entry)
							workerCopyNanos.Add(uint64(time.Since(copyStartedAt)))
							toCopyCount.Add(1)
							toCopyMissingCount.Add(1)
						case internalfs.ComparisonSize:
							copyStartedAt := time.Now()
							compareErr = internalfs.CopyEntry(job.Source, job.Destination, entry)
							workerCopyNanos.Add(uint64(time.Since(copyStartedAt)))
							toCopyCount.Add(1)
							toCopySizeCount.Add(1)
						case internalfs.ComparisonMTime:
							copyStartedAt := time.Now()
							compareErr = internalfs.CopyEntry(job.Source, job.Destination, entry)
							workerCopyNanos.Add(uint64(time.Since(copyStartedAt)))
							toCopyCount.Add(1)
							toCopyMTimeCount.Add(1)
						case internalfs.ComparisonCTime:
							copyStartedAt := time.Now()
							compareErr = internalfs.CopyEntry(job.Source, job.Destination, entry)
							workerCopyNanos.Add(uint64(time.Since(copyStartedAt)))
							toCopyCount.Add(1)
							toCopyCTimeCount.Add(1)
						case internalfs.ComparisonOK:
							okCount.Add(1)
						}
						if compareErr != nil {
							workerErrorCount.Add(1)
						}
					default:
						workerErrorCount.Add(1)
					}

					if compareErr != nil {
						setFatalErr(compareErr)
						return
					}
					if err := item.msg.Ack(); err != nil {
						setFatalErr(fmt.Errorf("cannot ack work message for job %q: %w", job.Token, err))
						return
					}
				}
			}
		}()
	}

	fetchWG.Wait()
	compareWG.Wait()
	close(progressDone)
	progressWG.Wait()

	result := currentResult()
	job, err = accumulateWorkerTotals(session.Context, kv, job, result)
	if err != nil {
		return result, err
	}

	result.Job = job

	if firstErr != nil {
		return result, firstErr
	}

	return result, nil
}

func publishWorkerMonitoring(ctx context.Context, js jetstream.JetStream, job natsclient.Job, result Result) error {
	startedAt := time.Now().UTC()
	if job.StartedAt != nil {
		startedAt = job.StartedAt.UTC()
	}

	return natsclient.PublishJSON(ctx, js, job.MonitoringSubject, natsclient.MonitoringMessage{
		Phase:             "worker",
		State:             job.State,
		StartedAt:         startedAt,
		UpdatedAt:         time.Now().UTC(),
		DiscoveredEntries: job.DiscoveredEntries,
		DiscoveredBytes:   job.DiscoveredBytes,
		PublishedWork:     job.PublishedWork,
		WorkerProcessedDelta:  result.WorkerProcessed,
		WorkerToCopyDelta:     result.WorkerToCopy,
		WorkerCopyMissingDelta: result.WorkerCopyMissing,
		WorkerCopySizeDelta:    result.WorkerCopySize,
		WorkerCopyMTimeDelta:   result.WorkerCopyMTime,
		WorkerCopyCTimeDelta:   result.WorkerCopyCTime,
		WorkerOKDelta:         result.WorkerOK,
		WorkerErrorsDelta:     result.WorkerErrors,
		WorkerLStatNanosDelta: result.WorkerLStatNanos,
		WorkerCopyNanosDelta:  result.WorkerCopyNanos,
		WorkerProcessed:       result.WorkerProcessed,
		WorkerToCopy:          result.WorkerToCopy,
		WorkerCopyMissing:     result.WorkerCopyMissing,
		WorkerCopySize:        result.WorkerCopySize,
		WorkerCopyMTime:       result.WorkerCopyMTime,
		WorkerCopyCTime:       result.WorkerCopyCTime,
		WorkerOK:              result.WorkerOK,
		WorkerErrors:          result.WorkerErrors,
		WorkerLStatNanos:      result.WorkerLStatNanos,
		WorkerCopyNanos:       result.WorkerCopyNanos,
		Errors:                job.Errors,
	})
}

func workEntry(work natsclient.WorkMessage) internalfs.Entry {
	return internalfs.Entry{
		Path:  work.Path,
		Name:  work.Name,
		Type:  natTypeToEntryType(work.Type),
		Inode: work.Inode,
		Mode:  work.Mode,
		Size:  work.Size,
		CTime: work.CTime,
		MTime: work.MTime,
	}
}

func natTypeToEntryType(t uint8) internalfs.EntryType {
	switch t {
	case natsclient.TypeFile:
		return internalfs.TypeFile
	case natsclient.TypeDir:
		return internalfs.TypeDir
	case natsclient.TypeSymlink:
		return internalfs.TypeSymlink
	case natsclient.TypeCharDev:
		return internalfs.TypeCharDev
	case natsclient.TypeDevice:
		return internalfs.TypeDevice
	case natsclient.TypePipe:
		return internalfs.TypePipe
	case natsclient.TypeSocket:
		return internalfs.TypeSocket
	default:
		return internalfs.TypeUnknown
	}
}

func accumulateWorkerTotals(ctx context.Context, kv jetstream.KeyValue, job natsclient.Job, delta Result) (natsclient.Job, error) {
	for range workerKVUpdateRetries {
		entry, err := kv.Get(ctx, job.Token)
		if err != nil {
			return natsclient.Job{}, fmt.Errorf("cannot load job %q for worker totals update: %w", job.Token, err)
		}

		var latest natsclient.Job
		if err := json.Unmarshal(entry.Value(), &latest); err != nil {
			return natsclient.Job{}, fmt.Errorf("cannot decode job %q for worker totals update: %w", job.Token, err)
		}
		if latest.Token == "" {
			latest.Token = job.Token
		}

		latest.WorkerProcessed += delta.WorkerProcessed
		latest.WorkerToCopy += delta.WorkerToCopy
		latest.WorkerCopyMissing += delta.WorkerCopyMissing
		latest.WorkerCopySize += delta.WorkerCopySize
		latest.WorkerCopyMTime += delta.WorkerCopyMTime
		latest.WorkerCopyCTime += delta.WorkerCopyCTime
		latest.WorkerOK += delta.WorkerOK
		latest.WorkerErrors += delta.WorkerErrors
		latest.WorkerLStatNanos += delta.WorkerLStatNanos
		latest.WorkerCopyNanos += delta.WorkerCopyNanos

		payload, err := json.Marshal(latest)
		if err != nil {
			return natsclient.Job{}, fmt.Errorf("cannot encode job %q for worker totals update: %w", job.Token, err)
		}
		if _, err := kv.Update(ctx, job.Token, payload, entry.Revision()); err == nil {
			return latest, nil
		}
	}

	return natsclient.Job{}, fmt.Errorf("cannot update worker totals for job %q after %d attempts", job.Token, workerKVUpdateRetries)
}

func workerConsumerName(token string) string {
	return fmt.Sprintf("%s-%s", workerConsumerPrefix, token)
}

func isFetchTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, os.ErrDeadlineExceeded) ||
		strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func isTerminalWorkerInput(state string) bool {
	return state == natsclient.JobStateCompleted || state == natsclient.JobStateFailed
}
