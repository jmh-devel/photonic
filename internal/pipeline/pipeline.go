package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"log/slog"

	"photonic/internal/config"
	"photonic/internal/logging"
	"photonic/internal/storage"
)

// JobType enumerates supported processing categories.
type JobType string

const (
	JobTimelapse JobType = "timelapse"
	JobPanoramic JobType = "panoramic"
	JobStack     JobType = "stack"
	JobScan      JobType = "scan"
	JobAlign     JobType = "align"
	JobRaw       JobType = "raw"
)

// Job represents a single processing request.
type Job struct {
	ID        string
	Type      JobType
	InputPath string
	Output    string
	Options   map[string]any
}

// Result captures the outcome of a Job.
type Result struct {
	Job   Job
	Error error
	Meta  map[string]any
}

// Processor executes a job and returns a Result.
type Processor interface {
	Process(ctx context.Context, job Job) Result
}

// Pipeline orchestrates job dispatch across workers.
type Pipeline struct {
	processor Processor
	log       *slog.Logger
	jobs      chan Job
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	startOnce sync.Once
	stopOnce  sync.Once
	store     *storage.Store
	alignCfg  *config.AlignmentConfig
	rawCfg    *config.RawProcessing
	mu        sync.Mutex
	subs      map[int]chan Result
	nextSubID int
}

// New creates a new Pipeline with the given concurrency and processor implementation.
func New(ctx context.Context, concurrency int, logger *slog.Logger, store *storage.Store, alignCfg *config.AlignmentConfig, rawCfg *config.RawProcessing) *Pipeline {
	if concurrency < 1 {
		concurrency = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	p := &Pipeline{
		log:      logger,
		jobs:     make(chan Job, concurrency*2),
		cancel:   cancel,
		store:    store,
		subs:     make(map[int]chan Result),
		alignCfg: alignCfg,
		rawCfg:   rawCfg,
	}

	p.startOnce.Do(func() {
		p.processor = newRouter(logger, store, alignCfg, rawCfg)
		for i := 0; i < concurrency; i++ {
			p.wg.Add(1)
			go p.worker(ctx, i)
		}
	})

	return p
}

// Submit adds a job to the processing queue.
func (p *Pipeline) Submit(job Job) error {
	if p.store != nil {
		optsJSON, _ := json.Marshal(job.Options)
		_ = p.store.RecordJobQueued(storage.JobRecord{
			ID:          job.ID,
			JobType:     string(job.Type),
			Status:      "queued",
			InputPath:   job.InputPath,
			OutputPath:  job.Output,
			OptionsJSON: string(optsJSON),
		})
	}

	select {
	case p.jobs <- job:
		return nil
	default:
		return errors.New("job queue is full")
	}
}

// Stop signals workers to exit and waits for completion.
func (p *Pipeline) Stop() {
	p.stopOnce.Do(func() {
		p.cancel()
		close(p.jobs)
		p.wg.Wait()
		p.mu.Lock()
		for id, ch := range p.subs {
			close(ch)
			delete(p.subs, id)
		}
		p.mu.Unlock()
	})
}

func (p *Pipeline) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			start := time.Now()

			// Enhanced job start logging
			logging.LogJobStart(p.log, string(job.Type), job.ID, job.InputPath, job.Output, job.Options)

			if p.store != nil {
				_ = p.store.RecordJobStart(job.ID)
			}
			res := p.processor.Process(ctx, job)
			duration := time.Since(start)

			// Enhanced job completion/error logging
			if res.Error != nil {
				logging.LogJobError(p.log, string(job.Type), job.ID, duration, res.Error, map[string]any{
					"input":   job.InputPath,
					"output":  job.Output,
					"options": job.Options,
				})
				status := "failed"
				if p.store != nil {
					_ = p.store.RecordJobResult(job.ID, status, res.Meta, errString(res.Error))
				}
			} else {
				logging.LogJobComplete(p.log, string(job.Type), job.ID, duration, res.Meta)
				status := "completed"
				if p.store != nil {
					_ = p.store.RecordJobResult(job.ID, status, res.Meta, errString(res.Error))
				}
			}

			p.broadcast(res)
		}
	}
}

// Subscribe returns a channel for receiving job results and an unsubscribe function.
func (p *Pipeline) Subscribe() (<-chan Result, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.nextSubID
	p.nextSubID++
	ch := make(chan Result, 8)
	p.subs[id] = ch
	unsub := func() {
		p.mu.Lock()
		if c, ok := p.subs[id]; ok {
			close(c)
			delete(p.subs, id)
		}
		p.mu.Unlock()
	}
	return ch, unsub
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (p *Pipeline) broadcast(res Result) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, ch := range p.subs {
		select {
		case ch <- res:
		default:
			p.log.Warn("result channel full", "subscriber", id, "job", res.Job.ID)
		}
	}
}
