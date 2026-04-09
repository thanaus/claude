package lsservice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	natsclient "github.com/nexus/nexus/internal/nats"
	"github.com/nexus/nexus/internal/config"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	readDirBatchSize = 1024
	workerCount      = 4
	entryBufferSize  = 4096
)

// Input holds the parameters required by the ls use case.
type Input struct {
	Token string
}

// Result contains the outcome of the ls workflow.
type Result struct {
	URL                string
	NATSReachable      bool
	JetStreamReady     bool
	KeyValue           natsclient.ResourceStatus
	Job                natsclient.Job
	DiscoveryPublished bool
	DiscoveredEntries  uint64
	PublishedWork      uint64
	Errors             uint64
	Warning            string
}

// Service orchestrates the ls workflow.
type Service struct{}

type scanStats struct {
	DiscoveredEntries uint64
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
		URL:            session.URL,
		NATSReachable:  true,
		JetStreamReady: true,
		KeyValue:       bucket,
		Job:            job,
		Warning:        workQueueLimitWarning(),
	}

	startedAt := time.Now().UTC()
	discoveryMessage := natsclient.NewRootDiscoveryMessage(job.Token, job.Source, startedAt)
	if err := natsclient.PublishJSON(session.Context, session.JetStream, job.DiscoverySubject, discoveryMessage); err != nil {
		return result, err
	}
	result.DiscoveryPublished = true

	job.State = natsclient.JobStateRunning
	job.StartedAt = &startedAt
	job.UpdatedAt = &startedAt
	if _, err := natsclient.UpdateJob(session.Context, kv, job); err != nil {
		return result, err
	}

	stats, scanErr := scanSource(session.Context, session.JetStream, job)
	result.DiscoveredEntries = stats.DiscoveredEntries
	result.PublishedWork = stats.PublishedWork
	result.Errors = stats.Errors

	finishedAt := time.Now().UTC()
	job.DiscoveredEntries = stats.DiscoveredEntries
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

	result.Job = job
	if scanErr != nil {
		return result, scanErr
	}

	return result, nil
}

func scanSource(ctx context.Context, js jetstream.JetStream, job natsclient.Job) (scanStats, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	entriesCh := make(chan os.DirEntry, entryBufferSize)

	var discoveredEntries atomic.Uint64
	var publishedWork atomic.Uint64
	var errorCount atomic.Uint64
	var firstErr error
	var firstErrOnce sync.Once
	var wg sync.WaitGroup

	setErr := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			errorCount.Add(1)
			cancel()
		})
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(entriesCh)

		f, err := os.Open(job.Source)
		if err != nil {
			setErr(fmt.Errorf("cannot open source directory %q: %w", job.Source, err))
			return
		}
		defer f.Close()

		for {
			if scanCtx.Err() != nil {
				return
			}

			entries, err := f.ReadDir(readDirBatchSize)
			if err != nil && !errors.Is(err, io.EOF) {
				setErr(fmt.Errorf("cannot read source directory %q: %w", job.Source, err))
				return
			}

			for _, entry := range entries {
				select {
				case <-scanCtx.Done():
					return
				case entriesCh <- entry:
					discoveredEntries.Add(1)
				}
			}

			if errors.Is(err, io.EOF) {
				return
			}
		}
	}()

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-scanCtx.Done():
					return
				case entry, ok := <-entriesCh:
					if !ok {
						return
					}

					fullPath := filepath.Join(job.Source, entry.Name())
					info, err := os.Lstat(fullPath)
					if err != nil {
						setErr(fmt.Errorf("cannot stat entry %q: %w", fullPath, err))
						return
					}

					message := buildWorkMessage(job.Token, fullPath, info)
					if err := natsclient.PublishJSON(scanCtx, js, job.WorkSubject, message); err != nil {
						setErr(err)
						return
					}

					publishedWork.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	stats := scanStats{
		DiscoveredEntries: discoveredEntries.Load(),
		PublishedWork:     publishedWork.Load(),
		Errors:            errorCount.Load(),
	}
	if firstErr != nil {
		return stats, firstErr
	}

	return stats, nil
}

func buildWorkMessage(token, fullPath string, info os.FileInfo) natsclient.WorkMessage {
	mode := info.Mode()

	return natsclient.WorkMessage{
		Token:     token,
		Path:      fullPath,
		Name:      info.Name(),
		Mode:      uint32(mode),
		Size:      info.Size(),
		ModTime:   info.ModTime().UTC(),
		IsDir:     info.IsDir(),
		IsSymlink: mode&os.ModeSymlink != 0,
	}
}

func workQueueLimitWarning() string {
	if natsclient.WorkQueueMaxMsgs <= 0 {
		return ""
	}

	return fmt.Sprintf(
		"DISCOVERY and WORK currently cap backlog at %d messages each; for very large directories, keep downstream consumers running or raise the stream limit.",
		natsclient.WorkQueueMaxMsgs,
	)
}
