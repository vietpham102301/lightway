package pool

import "sync/atomic"

// stats holds live counters updated atomically by worker goroutines.
type stats struct {
	processed atomic.Int64
	failed    atomic.Int64
	panics    atomic.Int64
}

// Snapshot is a point-in-time read of pool metrics.
// It is safe to call concurrently at any time, including during shutdown.
type Snapshot struct {
	ActiveWorkers int   // current live worker goroutines
	QueueDepth    int   // jobs currently waiting in the channel
	QueueCapacity int   // total queue capacity (static)
	Processed     int64 // jobs completed (success or error, excluding panics)
	Failed        int64 // jobs that returned a non-nil error
	Panics        int64 // jobs recovered from panic
}

// Stats returns a point-in-time snapshot of pool metrics.
func (p *Pool[T]) Stats() Snapshot {
	p.mu.Lock()
	active := p.activeWorkers
	p.mu.Unlock()

	return Snapshot{
		ActiveWorkers: active,
		QueueDepth:    len(p.jobs),
		QueueCapacity: cap(p.jobs),
		Processed:     p.stats.processed.Load(),
		Failed:        p.stats.failed.Load(),
		Panics:        p.stats.panics.Load(),
	}
}
