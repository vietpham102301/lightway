package kafka

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ===========================================================================
// ConsumerConfig — applyDefaults
// ===========================================================================

func TestConsumerConfig_Defaults(t *testing.T) {
	cfg := ConsumerConfig[string]{}
	cfg.applyDefaults()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: want 3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBaseDelay != 500*time.Millisecond {
		t.Errorf("RetryBaseDelay: want 500ms, got %v", cfg.RetryBaseDelay)
	}
	if cfg.RetryMaxDelay != 10*time.Second {
		t.Errorf("RetryMaxDelay: want 10s, got %v", cfg.RetryMaxDelay)
	}
	if cfg.SessionTimeout != 10*time.Second {
		t.Errorf("SessionTimeout: want 10s, got %v", cfg.SessionTimeout)
	}
	if cfg.Deserializer == nil {
		t.Error("Deserializer should default to JSON")
	}
}

// ===========================================================================
// retryBackoff
// ===========================================================================

func TestRetryBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		base     time.Duration
		max      time.Duration
		expected time.Duration
	}{
		{0, 100 * time.Millisecond, 1 * time.Second, 100 * time.Millisecond},
		{1, 100 * time.Millisecond, 1 * time.Second, 200 * time.Millisecond},
		{2, 100 * time.Millisecond, 1 * time.Second, 400 * time.Millisecond},
		{3, 100 * time.Millisecond, 1 * time.Second, 800 * time.Millisecond},
		{5, 100 * time.Millisecond, 1 * time.Second, 1 * time.Second}, // capped
	}
	for _, tt := range tests {
		got := retryBackoff(tt.base, tt.max, tt.attempt)
		if got != tt.expected {
			t.Errorf("retryBackoff(attempt=%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

// ===========================================================================
// Handler — mockHandler
// ===========================================================================

type mockHandler[T any] struct {
	calls   atomic.Int32
	failFor int // fail the first N calls
	lastMsg Message[T]
}

func (h *mockHandler[T]) Handle(_ context.Context, msg Message[T]) error {
	n := int(h.calls.Add(1))
	h.lastMsg = msg
	if n <= h.failFor {
		return errors.New("handler error")
	}
	return nil
}

// ===========================================================================
// processRecord — unit tests (drives handler directly without kafka)
// ===========================================================================

func TestProcessRecord_HandlerSuccess(t *testing.T) {
	handler := &mockHandler[string]{}
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:     3,
			RetryBaseDelay: 1 * time.Millisecond,
			RetryMaxDelay:  10 * time.Millisecond,
			Deserializer:   jsonDeserializer[string](),
		},
		handler: handler,
	}

	payload, _ := jsonSerializer[string]()("hello")
	r := &kgo.Record{Topic: "t", Partition: 0, Offset: 1, Value: payload}

	c.processRecord(context.Background(), r)

	if handler.calls.Load() != 1 {
		t.Errorf("expected handler called 1 time, got %d", handler.calls.Load())
	}
	if c.Stats().MessagesProcessed != 1 {
		t.Errorf("expected MessagesProcessed=1, got %d", c.Stats().MessagesProcessed)
	}
}

func TestProcessRecord_HandlerSucceedsOnRetry(t *testing.T) {
	handler := &mockHandler[string]{failFor: 2}
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:     3,
			RetryBaseDelay: 1 * time.Millisecond,
			RetryMaxDelay:  10 * time.Millisecond,
			Deserializer:   jsonDeserializer[string](),
		},
		handler: handler,
	}

	payload, _ := jsonSerializer[string]()("hello")
	r := &kgo.Record{Topic: "t", Partition: 0, Offset: 1, Value: payload}

	c.processRecord(context.Background(), r)

	if handler.calls.Load() != 3 { // 2 failures + 1 success
		t.Errorf("expected 3 handler calls, got %d", handler.calls.Load())
	}
	if c.Stats().MessagesProcessed != 1 {
		t.Errorf("expected MessagesProcessed=1, got %d", c.Stats().MessagesProcessed)
	}
	if c.Stats().HandlerErrors != 2 {
		t.Errorf("expected HandlerErrors=2, got %d", c.Stats().HandlerErrors)
	}
}

func TestProcessRecord_ExhaustedRetries_NoDLQ(t *testing.T) {
	handler := &mockHandler[string]{failFor: 100}
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:     2,
			RetryBaseDelay: 1 * time.Millisecond,
			RetryMaxDelay:  5 * time.Millisecond,
			Deserializer:   jsonDeserializer[string](),
		},
		handler: handler,
	}

	payload, _ := jsonSerializer[string]()("hello")
	r := &kgo.Record{Topic: "t", Partition: 0, Offset: 1, Value: payload}

	c.processRecord(context.Background(), r)

	if c.Stats().MessagesProcessed != 0 {
		t.Errorf("expected MessagesProcessed=0, got %d", c.Stats().MessagesProcessed)
	}
	if c.Stats().HandlerErrors != 3 { // 1 initial + 2 retries
		t.Errorf("expected HandlerErrors=3, got %d", c.Stats().HandlerErrors)
	}
}

func TestProcessRecord_DeserializationError(t *testing.T) {
	handler := &mockHandler[string]{}
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:   3,
			Deserializer: jsonDeserializer[string](),
		},
		handler: handler,
	}

	r := &kgo.Record{Topic: "t", Partition: 0, Offset: 1, Value: []byte("not valid json")}

	c.processRecord(context.Background(), r)

	if handler.calls.Load() != 0 {
		t.Error("handler should not be called when deserialization fails")
	}
	if c.Stats().DeserializationErrors != 1 {
		t.Errorf("expected DeserializationErrors=1, got %d", c.Stats().DeserializationErrors)
	}
}

func TestProcessRecord_PanicRecovery(t *testing.T) {
	panicHandler := HandlerFunc[string](func(_ context.Context, _ Message[string]) error {
		panic("boom")
	})
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:   0,
			Deserializer: jsonDeserializer[string](),
		},
		handler: panicHandler,
	}

	payload, _ := jsonSerializer[string]()("hello")
	r := &kgo.Record{Topic: "t", Partition: 0, Offset: 1, Value: payload}

	// Should not panic the test goroutine.
	c.processRecord(context.Background(), r)

	if c.Stats().Panics != 1 {
		t.Errorf("expected Panics=1, got %d", c.Stats().Panics)
	}
}

func TestProcessRecord_Headers(t *testing.T) {
	var capturedMsg Message[string]
	handler := HandlerFunc[string](func(_ context.Context, msg Message[string]) error {
		capturedMsg = msg
		return nil
	})
	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			MaxRetries:   0,
			Deserializer: jsonDeserializer[string](),
		},
		handler: handler,
	}

	payload, _ := jsonSerializer[string]()("hello")
	r := &kgo.Record{
		Topic:     "t",
		Partition: 0,
		Offset:    1,
		Value:     payload,
		Headers: []kgo.RecordHeader{
			{Key: "trace-id", Value: []byte("abc123")},
		},
	}

	c.processRecord(context.Background(), r)

	if capturedMsg.Headers["trace-id"] != "abc123" {
		t.Errorf("expected header trace-id=abc123, got %q", capturedMsg.Headers["trace-id"])
	}
}

// ===========================================================================
// Consumer — integration with fake cluster
// ===========================================================================

func TestConsumer_Start_ReceivesMessages(t *testing.T) {
	const topic = "test-consume"

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, topic),
	)
	if err != nil {
		t.Fatalf("failed to start fake cluster: %v", err)
	}
	defer cluster.Close()

	// Produce a message first.
	producer, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("failed to create producer client: %v", err)
	}
	defer producer.Close()

	payload, _ := jsonSerializer[string]()("world")
	results := producer.ProduceSync(context.Background(), &kgo.Record{
		Topic: topic,
		Value: payload,
	})
	if err := results.FirstErr(); err != nil {
		t.Fatalf("failed to produce: %v", err)
	}

	// Now consume with our Consumer.
	var received atomic.Int32
	handler := HandlerFunc[string](func(_ context.Context, msg Message[string]) error {
		received.Add(1)
		return nil
	})

	kConsumer, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ConsumerGroup("test-group"),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.AutoCommitMarks(),
	)
	if err != nil {
		t.Fatalf("failed to create consumer client: %v", err)
	}

	c := &Consumer[string]{
		cfg: ConsumerConfig[string]{
			GroupID:        "test-group",
			Topics:         []string{topic},
			MaxRetries:     0,
			RetryBaseDelay: 1 * time.Millisecond,
			RetryMaxDelay:  5 * time.Millisecond,
			Deserializer:   jsonDeserializer[string](),
		},
		handler: handler,
		client:  kConsumer,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go c.Start(ctx) //nolint:errcheck

	// Poll until message is received or timeout.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Errorf("expected at least 1 message received, got %d", received.Load())
}
