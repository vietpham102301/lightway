package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ===========================================================================
// Test helpers — minimal Job[int] implementations
// ===========================================================================

type successJob struct{ val int }

func (j successJob) Execute(_ context.Context) (int, error) { return j.val, nil }

type errorJob struct{ err error }

func (j errorJob) Execute(_ context.Context) (int, error) { return 0, j.err }

type panicJob struct{}

func (j panicJob) Execute(_ context.Context) (int, error) { panic("boom") }

type slowJob struct{}

func (j slowJob) Execute(ctx context.Context) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

// fastCfg returns a Config suitable for tests — short timeouts so tests finish quickly.
func fastCfg() Config {
	return Config{
		MinWorkers:    2,
		MaxWorkers:    8,
		QueueSize:     100,
		IdleTimeout:   50 * time.Millisecond,
		ScaleInterval: 10 * time.Millisecond,
	}
}

// ===========================================================================
// Config — applyDefaults
// ===========================================================================

func TestConfig_Defaults(t *testing.T) {
	var cfg Config
	cfg.applyDefaults()

	if cfg.MinWorkers <= 0 {
		t.Errorf("MinWorkers: want > 0, got %d", cfg.MinWorkers)
	}
	if cfg.MaxWorkers <= 0 {
		t.Errorf("MaxWorkers: want > 0, got %d", cfg.MaxWorkers)
	}
	if cfg.QueueSize <= 0 {
		t.Errorf("QueueSize: want > 0, got %d", cfg.QueueSize)
	}
	if cfg.IdleTimeout <= 0 {
		t.Errorf("IdleTimeout: want > 0, got %v", cfg.IdleTimeout)
	}
	if cfg.ScaleInterval <= 0 {
		t.Errorf("ScaleInterval: want > 0, got %v", cfg.ScaleInterval)
	}
}

func TestConfig_MinWorkersClampedToMax(t *testing.T) {
	cfg := Config{MinWorkers: 10, MaxWorkers: 3}
	cfg.applyDefaults()
	if cfg.MinWorkers > cfg.MaxWorkers {
		t.Errorf("MinWorkers (%d) must not exceed MaxWorkers (%d)", cfg.MinWorkers, cfg.MaxWorkers)
	}
}

// ===========================================================================
// New
// ===========================================================================

func TestNew_ReturnsNonNil(t *testing.T) {
	p := New[int](Config{})
	if p == nil {
		t.Fatal("New returned nil")
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	p := New[int](Config{})
	if p.cfg.MinWorkers <= 0 {
		t.Errorf("cfg.MinWorkers not defaulted, got %d", p.cfg.MinWorkers)
	}
}

// ===========================================================================
// Start / Stop
// ===========================================================================

func TestStart_IsIdempotent(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	p.Start() // second call must not panic or deadlock
	p.Stop()
}

func TestStop_BeforeStart_IsNoop(t *testing.T) {
	p := New[int](fastCfg())
	p.Stop() // must not panic
}

func TestStop_DrainsQueuedJobs(t *testing.T) {
	cfg := fastCfg()
	cfg.MinWorkers = 0
	cfg.MaxWorkers = 0
	cfg.QueueSize = 10
	// We can't set MaxWorkers=0 after applyDefaults clamps it, so use a pool
	// where workers are blocked to ensure jobs sit in queue during Stop.
	p := New[int](Config{
		MinWorkers:    1,
		MaxWorkers:    1,
		QueueSize:     10,
		IdleTimeout:   time.Second,
		ScaleInterval: time.Second,
	})
	p.Start()

	// Block the single worker.
	blocker := make(chan struct{})
	type blockJob struct{}
	// We need a Job[int] that blocks.
	type blockingJob struct{ ch chan struct{} }
	// Reuse slowJob concept inline.

	// Submit a slow job to occupy the worker.
	slowCh, _ := p.Submit(slowJob{})
	_ = slowCh

	// Give worker time to pick up the slow job.
	time.Sleep(20 * time.Millisecond)
	close(blocker)

	// Submit jobs that will sit in queue.
	const queued = 3
	resultChans := make([]<-chan Result[int], 0, queued)
	for i := 0; i < queued; i++ {
		ch, err := p.Submit(successJob{val: i})
		if err == nil {
			resultChans = append(resultChans, ch)
		}
	}

	p.Stop()

	// All queued jobs (and the slow job) must get a result (ErrPoolStopped or normal).
	for _, ch := range resultChans {
		select {
		case res, ok := <-ch:
			if !ok {
				t.Error("result channel closed without a value")
			}
			_ = res // may be ErrPoolStopped or a successful result
		case <-time.After(time.Second):
			t.Error("timeout waiting for drained job result")
		}
	}
}

// ===========================================================================
// Submit
// ===========================================================================

func TestSubmit_ReturnsResult(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	ch, err := p.Submit(successJob{val: 42})
	if err != nil {
		t.Fatalf("Submit: unexpected error: %v", err)
	}

	select {
	case res := <-ch:
		if res.Err != nil {
			t.Fatalf("expected nil error, got %v", res.Err)
		}
		if res.Value != 42 {
			t.Fatalf("expected value 42, got %d", res.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestSubmit_OnStoppedPool_ReturnsError(t *testing.T) {
	p := New[int](fastCfg())
	// Don't start — pool is inactive.
	_, err := p.Submit(successJob{})
	if !errors.Is(err, ErrPoolStopped) {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}
}

func TestSubmit_QueueFull_ReturnsErrQueueFull(t *testing.T) {
	p := New[int](Config{
		MinWorkers:    1,
		MaxWorkers:    1,
		QueueSize:     1,
		IdleTimeout:   time.Second,
		ScaleInterval: time.Second,
	})
	p.Start()
	defer p.Stop()

	// Block the worker.
	p.Submit(slowJob{}) //nolint — intentionally not checking error here

	// Give the worker time to pick up the slow job.
	time.Sleep(20 * time.Millisecond)

	// Fill the queue.
	p.Submit(successJob{}) //nolint

	// Now the queue should be full.
	_, err := p.Submit(successJob{})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestSubmit_Concurrent(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	const goroutines = 100
	var wg sync.WaitGroup
	var successCount atomic.Int32

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(val int) {
			defer wg.Done()
			ch, err := p.Submit(successJob{val: val})
			if err != nil {
				return // queue may be transiently full
			}
			res := <-ch
			if res.Err == nil {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if count := successCount.Load(); count == 0 {
		t.Error("expected at least some successful concurrent submissions")
	}
}

// ===========================================================================
// Error handling
// ===========================================================================

func TestJob_ReturnsError_CountedAsFailed(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	sentinel := errors.New("job error")
	ch, err := p.Submit(errorJob{err: sentinel})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	res := <-ch
	if !errors.Is(res.Err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", res.Err)
	}

	snap := p.Stats()
	if snap.Failed < 1 {
		t.Errorf("expected Failed >= 1, got %d", snap.Failed)
	}
}

func TestJob_Panic_RecoveredAndCountedAsPanic(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	ch, err := p.Submit(panicJob{})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	res := <-ch
	if res.Err == nil {
		t.Fatal("expected error from panicking job")
	}

	snap := p.Stats()
	if snap.Panics < 1 {
		t.Errorf("expected Panics >= 1, got %d", snap.Panics)
	}
}

func TestJob_Panic_WorkerSurvives(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	// Submit a panic job.
	ch, _ := p.Submit(panicJob{})
	<-ch

	// The pool must still process subsequent jobs.
	ch2, err := p.Submit(successJob{val: 7})
	if err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	res := <-ch2
	if res.Err != nil || res.Value != 7 {
		t.Fatalf("expected (7, nil) after panic recovery, got (%d, %v)", res.Value, res.Err)
	}
}

// ===========================================================================
// Dynamic scaling
// ===========================================================================

func TestScaling_SpawnsWorkersUnderLoad(t *testing.T) {
	cfg := Config{
		MinWorkers:    1,
		MaxWorkers:    8,
		QueueSize:     200,
		IdleTimeout:   500 * time.Millisecond,
		ScaleInterval: 10 * time.Millisecond,
	}
	p := New[int](cfg)
	p.Start()
	defer p.Stop()

	initialSnap := p.Stats()
	if initialSnap.ActiveWorkers != 1 {
		t.Fatalf("expected 1 initial worker, got %d", initialSnap.ActiveWorkers)
	}

	// Submit a burst of slow jobs to trigger scale-up.
	for i := 0; i < 50; i++ {
		p.Submit(slowJob{}) //nolint
	}

	// Allow scaler to react.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := p.Stats()
		if snap.ActiveWorkers > 1 {
			return // scaled up
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("expected ActiveWorkers to grow above 1, got %d", p.Stats().ActiveWorkers)
}

func TestScaling_ShrinksWorkersWhenIdle(t *testing.T) {
	cfg := Config{
		MinWorkers:    1,
		MaxWorkers:    4,
		QueueSize:     100,
		IdleTimeout:   50 * time.Millisecond,
		ScaleInterval: 10 * time.Millisecond,
	}
	p := New[int](cfg)
	p.Start()
	defer p.Stop()

	// Submit burst of quick jobs to scale up.
	var chans []<-chan Result[int]
	for i := 0; i < 20; i++ {
		ch, err := p.Submit(successJob{val: i})
		if err == nil {
			chans = append(chans, ch)
		}
	}
	// Drain results.
	for _, ch := range chans {
		<-ch
	}

	// After IdleTimeout, workers above MinWorkers should exit.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := p.Stats()
		if snap.ActiveWorkers <= cfg.MinWorkers {
			return // scaled down
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("expected ActiveWorkers to shrink to %d, got %d", cfg.MinWorkers, p.Stats().ActiveWorkers)
}

// ===========================================================================
// Stats / Snapshot
// ===========================================================================

func TestStats_ZeroedBeforeStart(t *testing.T) {
	p := New[int](fastCfg())
	snap := p.Stats()
	if snap.Processed != 0 || snap.Failed != 0 || snap.Panics != 0 {
		t.Errorf("expected all counters zero before Start, got %+v", snap)
	}
}

func TestStats_ProcessedAndFailed_Updated(t *testing.T) {
	p := New[int](fastCfg())
	p.Start()
	defer p.Stop()

	ch1, _ := p.Submit(successJob{val: 1})
	ch2, _ := p.Submit(errorJob{err: errors.New("e")})
	<-ch1
	<-ch2

	snap := p.Stats()
	if snap.Processed < 2 {
		t.Errorf("expected Processed >= 2, got %d", snap.Processed)
	}
	if snap.Failed < 1 {
		t.Errorf("expected Failed >= 1, got %d", snap.Failed)
	}
}

func TestStats_QueueDepth_ReflectsPending(t *testing.T) {
	p := New[int](Config{
		MinWorkers:    1,
		MaxWorkers:    1,
		QueueSize:     10,
		IdleTimeout:   time.Second,
		ScaleInterval: time.Second,
	})
	p.Start()
	defer p.Stop()

	// Block the worker.
	p.Submit(slowJob{}) //nolint
	time.Sleep(20 * time.Millisecond)

	// Queue some jobs.
	p.Submit(successJob{}) //nolint
	p.Submit(successJob{}) //nolint

	snap := p.Stats()
	if snap.QueueDepth < 1 {
		t.Errorf("expected QueueDepth >= 1, got %d", snap.QueueDepth)
	}
}

func TestStats_ActiveWorkers_Accurate(t *testing.T) {
	cfg := fastCfg()
	p := New[int](cfg)
	p.Start()
	defer p.Stop()

	snap := p.Stats()
	if snap.ActiveWorkers != cfg.MinWorkers {
		t.Errorf("expected ActiveWorkers == MinWorkers (%d), got %d", cfg.MinWorkers, snap.ActiveWorkers)
	}
}
