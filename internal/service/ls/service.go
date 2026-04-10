package lsservice

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	defaultWorkerCount = 4
	defaultPublisherCount = 4
	entryBufferSize  = 4096
	publishMaxRetries = 5
	publishRetryBase  = 100 * time.Millisecond
	progressInterval = time.Second
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

	workers := in.Workers
	if workers <= 0 {
		workers = defaultWorkerCount
	}

	stats, scanErr := scanSource(session.Context, session.JetStream, job, workers, sink)
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

func scanSource(ctx context.Context, js jetstream.JetStream, job natsclient.Job, workers int, sink EventSink) (scanStats, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	entriesCh := make(chan os.DirEntry, entryBufferSize)
	resultsCh := make(chan natsclient.WorkMessage, entryBufferSize)

	var discoveredEntries atomic.Uint64
	var discoveredBytes atomic.Uint64
	var publishedWork atomic.Uint64
	var errorCount atomic.Uint64
	var firstErr error
	var firstErrOnce sync.Once
	var statWG sync.WaitGroup
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

	recordEntryErr := func(error) {
		errorCount.Add(1)
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

		ticker := time.NewTicker(progressInterval)
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

	go func() {
		defer close(entriesCh)

		f, err := os.Open(job.Source)
		if err != nil {
			setFatalErr(fmt.Errorf("cannot open source directory %q: %w", job.Source, err))
			return
		}
		defer f.Close()

		for {
			if scanCtx.Err() != nil {
				return
			}

			entries, err := f.ReadDir(readDirBatchSize)
			if err != nil && !errors.Is(err, io.EOF) {
				setFatalErr(fmt.Errorf("cannot read source directory %q: %w", job.Source, err))
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

	for range workers {
		statWG.Add(1)
		go func() {
			defer statWG.Done()

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
						recordEntryErr(fmt.Errorf("cannot stat entry %q: %w", fullPath, err))
						continue
					}
					discoveredBytes.Add(statSizeBytes(info))

					relativePath, err := filepath.Rel(job.Source, fullPath)
					if err != nil {
						setFatalErr(fmt.Errorf("cannot derive relative path for %q from source %q: %w", fullPath, job.Source, err))
						return
					}

					message := buildWorkMessage(relativePath, info)
					select {
					case <-scanCtx.Done():
						return
					case resultsCh <- message:
					}
				}
			}
		}()
	}

	go func() {
		statWG.Wait()
		close(resultsCh)
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

func buildWorkMessage(relativePath string, info os.FileInfo) natsclient.WorkMessage {
	mode := info.Mode()
	inode, ctime := fileStatMetadata(info)

	return natsclient.WorkMessage{
		Path:      relativePath,
		Name:      info.Name(),
		Type:      modeToType(mode),
		Inode:     inode,
		Mode:      uint32(mode),
		Size:      info.Size(),
		CTime:     ctime,
		MTime:     info.ModTime().Unix(),
	}
}

func statSizeBytes(info os.FileInfo) uint64 {
	size := info.Size()
	if size <= 0 {
		return 0
	}

	return uint64(size)
}

func modeToType(m fs.FileMode) uint8 {
	t := m.Type()
	switch {
	case t == 0:
		return natsclient.TypeFile
	case t&fs.ModeDir != 0:
		return natsclient.TypeDir
	case t&fs.ModeSymlink != 0:
		return natsclient.TypeSymlink
	case t&fs.ModeDevice != 0 && t&fs.ModeCharDevice != 0:
		return natsclient.TypeCharDev
	case t&fs.ModeDevice != 0:
		return natsclient.TypeDevice
	case t&fs.ModeNamedPipe != 0:
		return natsclient.TypePipe
	case t&fs.ModeSocket != 0:
		return natsclient.TypeSocket
	default:
		return natsclient.TypeUnknown
	}
}

