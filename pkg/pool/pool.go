// Package pool provides a generic, dynamically-scaling worker pool.
// Users only need to implement the Job[T] interface to submit work.
package pool

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Sentinel errors returned by Submit and delivered via Result.Err.
var (
	// ErrPoolStopped is returned when submitting to a stopped pool,
	// or delivered to jobs that were queued but never executed before Stop().
	ErrPoolStopped = errors.New("pool: pool has been stopped")

	// ErrQueueFull is returned by Submit when the job queue is at capacity.
	ErrQueueFull = errors.New("pool: job queue is full")
)

// Job is the interface that all submitted work must implement.
// T is the result type produced by the job; use struct{} if no result is needed.
type Job[T any] interface {
	// Execute performs the job. ctx is the pool's root context — check it
	// for cancellation in long-running jobs to support graceful shutdown.
	Execute(ctx context.Context) (T, error)
}

// Result carries the outcome of a single job execution back to the caller.
type Result[T any] struct {
	Value T
	Err   error
}

// Config holds all tunables for a Pool. Zero values produce sensible defaults.
type Config struct {
	// MinWorkers is the minimum number of goroutines kept alive at all times.
	// Default: 2
	MinWorkers int

	// MaxWorkers is the upper bound on goroutines the pool may spawn.
	// Default: runtime.NumCPU() * 2
	MaxWorkers int

	// QueueSize is the capacity of the internal job channel.
	// Default: MaxWorkers * 10
	QueueSize int

	// IdleTimeout is how long a worker above MinWorkers may sit idle before
	// it exits, enabling scale-down. Default: 30s
	IdleTimeout time.Duration

	// ScaleInterval is how often the scaler goroutine checks whether new
	// workers should be spawned. Default: 100ms
	ScaleInterval time.Duration
}

func (c *Config) applyDefaults() {
	if c.MinWorkers <= 0 {
		c.MinWorkers = 2
	}
	if c.MaxWorkers <= 0 {
		c.MaxWorkers = runtime.NumCPU() * 2
	}
	if c.MinWorkers > c.MaxWorkers {
		c.MinWorkers = c.MaxWorkers
	}
	if c.QueueSize <= 0 {
		c.QueueSize = c.MaxWorkers * 10
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 30 * time.Second
	}
	if c.ScaleInterval <= 0 {
		c.ScaleInterval = 100 * time.Millisecond
	}
}

// Pool is a generic, dynamically-scaling worker pool.
// T is the result type produced by every Job submitted to this pool.
type Pool[T any] struct {
	cfg Config

	// jobs is the buffered channel workers pull work from.
	jobs chan jobEnvelope[T]

	// ctx/cancel broadcast shutdown to all goroutines.
	ctx    context.Context
	cancel context.CancelFunc

	// wg tracks all worker goroutines and the scaler goroutine.
	wg sync.WaitGroup

	// mu guards activeWorkers for scaling decisions.
	mu            sync.Mutex
	activeWorkers int

	stats   stats
	started atomic.Bool
}

// jobEnvelope pairs a Job with its result channel so a worker can deliver
// the outcome directly to the original caller.
type jobEnvelope[T any] struct {
	job    Job[T]
	result chan<- Result[T]
}

// New creates a Pool configured by cfg.
// The pool does not start processing until Start() is called.
func New[T any](cfg Config) *Pool[T] {
	cfg.applyDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool[T]{
		cfg:    cfg,
		jobs:   make(chan jobEnvelope[T], cfg.QueueSize),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start launches MinWorkers goroutines and the scaler goroutine.
// Calling Start on an already-started pool is a no-op.
func (p *Pool[T]) Start() {
	if !p.started.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < p.cfg.MinWorkers; i++ {
		p.spawnWorker()
	}
	p.wg.Add(1)
	go p.scaler()
}

// Stop signals all workers to finish their current job, waits for them to
// exit, then drains any remaining queued jobs with ErrPoolStopped.
// Stop is safe to call multiple times; only the first call has effect.
func (p *Pool[T]) Stop() {
	if !p.started.Load() {
		return
	}
	p.cancel()   // unblocks idle workers and the scaler
	p.wg.Wait()  // waits for all goroutines to exit

	// Safe to close the channel now — no concurrent senders remain.
	close(p.jobs)

	// Drain jobs that were queued but never picked up.
	for env := range p.jobs {
		var zero T
		env.result <- Result[T]{Value: zero, Err: ErrPoolStopped}
		close(env.result)
	}
}

// Submit enqueues job for execution and returns a channel that will receive
// exactly one Result when the job completes (or the pool stops).
// The returned channel is buffered with capacity 1; callers may block-wait or
// use select.
//
// Submit returns ErrPoolStopped immediately if the pool is not running.
// Submit returns ErrQueueFull if the queue is at capacity (never blocks).
func (p *Pool[T]) Submit(job Job[T]) (<-chan Result[T], error) {
	if !p.started.Load() || p.ctx.Err() != nil {
		return nil, ErrPoolStopped
	}

	result := make(chan Result[T], 1)
	env := jobEnvelope[T]{job: job, result: result}

	select {
	case p.jobs <- env:
		return result, nil
	default:
		return nil, ErrQueueFull
	}
}
