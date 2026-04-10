package pool

import (
	"fmt"
	"time"

	"github.com/vietpham102301/lightway/pkg/logger"
)

// spawnWorker increments activeWorkers and launches a new worker goroutine.
func (p *Pool[T]) spawnWorker() {
	p.mu.Lock()
	p.activeWorkers++
	p.mu.Unlock()

	p.wg.Add(1)
	go p.worker()
}

// worker is the main goroutine loop. It pulls jobs from the channel and
// executes them. Workers above MinWorkers exit after IdleTimeout of
// inactivity, providing automatic scale-down.
func (p *Pool[T]) worker() {
	defer func() {
		p.mu.Lock()
		p.activeWorkers--
		p.mu.Unlock()
		p.wg.Done()
	}()

	idleTimer := time.NewTimer(p.cfg.IdleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return

		case env, ok := <-p.jobs:
			if !ok {
				return
			}
			// Reset idle timer each time work arrives.
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(p.cfg.IdleTimeout)

			p.executeJob(env)

		case <-idleTimer.C:
			p.mu.Lock()
			canShrink := p.activeWorkers > p.cfg.MinWorkers
			p.mu.Unlock()

			if canShrink {
				return
			}
			// Below minimum — keep the worker alive.
			idleTimer.Reset(p.cfg.IdleTimeout)
		}
	}
}

// executeJob runs a single job with panic recovery and delivers the result.
// Panic recovery lives here (not in worker) so a panicking job does not kill
// the worker goroutine — the worker continues processing future jobs.
func (p *Pool[T]) executeJob(env jobEnvelope[T]) {
	defer func() {
		if r := recover(); r != nil {
			p.stats.panics.Add(1)
			err := fmt.Errorf("pool: job panicked: %v", r)
			logger.Error("worker recovered from panic", "panic", r)
			var zero T
			env.result <- Result[T]{Value: zero, Err: err}
			close(env.result)
		}
	}()

	value, err := env.job.Execute(p.ctx)
	p.stats.processed.Add(1)
	if err != nil {
		p.stats.failed.Add(1)
	}

	env.result <- Result[T]{Value: value, Err: err}
	close(env.result)
}

// scaler periodically checks whether new workers should be spawned to handle
// queue pressure. Scale-down is handled naturally by the idle timer in each
// worker — no dedicated scale-down goroutine is needed.
func (p *Pool[T]) scaler() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.ScaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return

		case <-ticker.C:
			queueDepth := len(p.jobs)
			if queueDepth == 0 {
				continue
			}

			p.mu.Lock()
			current := p.activeWorkers
			max := p.cfg.MaxWorkers
			p.mu.Unlock()

			if current >= max {
				continue
			}

			// Spawn enough workers to drain the queue, up to MaxWorkers.
			toSpawn := max - current
			if queueDepth < toSpawn {
				toSpawn = queueDepth
			}

			spawned := 0
			for i := 0; i < toSpawn; i++ {
				p.mu.Lock()
				if p.activeWorkers >= p.cfg.MaxWorkers {
					p.mu.Unlock()
					break
				}
				p.mu.Unlock()
				p.spawnWorker()
				spawned++
			}

			if spawned > 0 {
				logger.Info("pool: scaled up workers",
					"added", spawned,
					"queue_depth", queueDepth,
				)
			}
		}
	}
}
