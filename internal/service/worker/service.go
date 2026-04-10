package workerservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/nexus/nexus/internal/config"
	natsclient "github.com/nexus/nexus/internal/nats"
)

const (
	defaultWorkerCount   = 4
	workerFetchBatchSize = 256
	workerFetchWait      = time.Second
	workerConsumerPrefix = "worker"
	workerBufferSize     = 256
	workerProgressPeriod = time.Second
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
	WorkerOK        uint64
	WorkerErrors    uint64
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
	var okCount atomic.Uint64
	var workerErrorCount atomic.Uint64
	var firstErr error
	var firstErrOnce sync.Once
	var fetchWG sync.WaitGroup
	var compareWG sync.WaitGroup
	var progressWG sync.WaitGroup
	progressDone := make(chan struct{})
	var lastPublishedProcessed uint64
	var lastPublishedToCopy uint64
	var lastPublishedOK uint64
	var lastPublishedErrors uint64

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
			WorkerOK:        okCount.Load(),
			WorkerErrors:    workerErrorCount.Load(),
		}
	}

	reportProgress := func() {
		current := currentResult()
		delta := Result{
			Job:             job,
			WorkerProcessed: current.WorkerProcessed - lastPublishedProcessed,
			WorkerToCopy:    current.WorkerToCopy - lastPublishedToCopy,
			WorkerOK:        current.WorkerOK - lastPublishedOK,
			WorkerErrors:    current.WorkerErrors - lastPublishedErrors,
		}
		if delta.WorkerProcessed == 0 && delta.WorkerToCopy == 0 && delta.WorkerOK == 0 && delta.WorkerErrors == 0 {
			return
		}
		lastPublishedProcessed = current.WorkerProcessed
		lastPublishedToCopy = current.WorkerToCopy
		lastPublishedOK = current.WorkerOK
		lastPublishedErrors = current.WorkerErrors

		if err := publishWorkerMonitoring(workerCtx, session.JetStream, job, delta); err != nil && workerCtx.Err() == nil {
			setFatalErr(err)
		}
	}

	progressWG.Add(1)
	go func() {
		defer progressWG.Done()

		ticker := time.NewTicker(workerProgressPeriod)
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

					classification, compareErr := compareDestination(job.Destination, item.work)
					processedCount.Add(1)

					switch {
					case compareErr == nil:
						switch classification {
						case comparisonToCopy:
							toCopyCount.Add(1)
						case comparisonOK:
							okCount.Add(1)
						}
					default:
						workerErrorCount.Add(1)
					}

					if err := item.msg.Ack(); err != nil {
						setFatalErr(fmt.Errorf("cannot ack work message for job %q: %w", job.Token, err))
						return
					}
					if compareErr != nil {
						setFatalErr(compareErr)
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

type destinationComparison uint8

const (
	comparisonUnknown destinationComparison = iota
	comparisonOK
	comparisonToCopy
)

func compareDestination(root string, work natsclient.WorkMessage) (destinationComparison, error) {
	fullPath := filepath.Join(root, work.Path)
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return comparisonToCopy, nil
		}
		return comparisonUnknown, fmt.Errorf("cannot stat destination entry %q: %w", fullPath, err)
	}

	_, ctime := fileStatMetadata(info)
	if info.Size() != work.Size || info.ModTime().Unix() != work.MTime || ctime != work.CTime {
		return comparisonToCopy, nil
	}

	return comparisonOK, nil
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
		WorkerProcessedDelta: result.WorkerProcessed,
		WorkerToCopyDelta:    result.WorkerToCopy,
		WorkerOKDelta:        result.WorkerOK,
		WorkerErrorsDelta:    result.WorkerErrors,
		WorkerProcessed:   result.WorkerProcessed,
		WorkerToCopy:      result.WorkerToCopy,
		WorkerOK:          result.WorkerOK,
		WorkerErrors:      result.WorkerErrors,
		Errors:            job.Errors,
	})
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
		latest.WorkerOK += delta.WorkerOK
		latest.WorkerErrors += delta.WorkerErrors

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
