package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ===========================================================================
// ProducerConfig — applyDefaults
// ===========================================================================

func TestProducerConfig_Defaults(t *testing.T) {
	cfg := ProducerConfig[string]{}
	cfg.applyDefaults()

	if cfg.Serializer == nil {
		t.Error("Serializer should default to JSON")
	}
	if cfg.RecordRetries != 3 {
		t.Errorf("RecordRetries: want 3, got %d", cfg.RecordRetries)
	}
	if cfg.MaxBufferedRecords != 1000 {
		t.Errorf("MaxBufferedRecords: want 1000, got %d", cfg.MaxBufferedRecords)
	}
}

// ===========================================================================
// Producer — Send (closed guard)
// ===========================================================================

func TestProducer_Send_OnClosedProducer_ReturnsError(t *testing.T) {
	p := &Producer[string]{}
	p.closed.Store(true)

	err := p.Send(context.Background(), "hello")
	if !errors.Is(err, ErrProducerClosed) {
		t.Fatalf("expected ErrProducerClosed, got %v", err)
	}
}

func TestProducer_SendBatch_OnClosedProducer_ReturnsError(t *testing.T) {
	p := &Producer[string]{}
	p.closed.Store(true)

	err := p.SendBatch(context.Background(), []string{"a", "b"})
	if !errors.Is(err, ErrProducerClosed) {
		t.Fatalf("expected ErrProducerClosed, got %v", err)
	}
}

// ===========================================================================
// Producer — Serialization errors
// ===========================================================================

func TestProducer_Send_SerializationError(t *testing.T) {
	p := &Producer[string]{
		cfg: ProducerConfig[string]{
			Topic: "test-topic",
			Serializer: func(_ string) ([]byte, error) {
				return nil, errors.New("cannot serialize")
			},
		},
	}

	err := p.Send(context.Background(), "hello")
	if !errors.Is(err, ErrSerializationFailed) {
		t.Fatalf("expected ErrSerializationFailed, got %v", err)
	}
}

// ===========================================================================
// Producer — Stats
// ===========================================================================

func TestProducer_Stats_InitialZero(t *testing.T) {
	p := &Producer[string]{}
	snap := p.Stats()
	if snap.MessagesSent != 0 || snap.Errors != 0 {
		t.Errorf("expected zero stats, got %+v", snap)
	}
}

// ===========================================================================
// Producer — Send with fake cluster (integration)
// ===========================================================================

func TestProducer_Send_Success(t *testing.T) {
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, "test-topic"),
	)
	if err != nil {
		t.Fatalf("failed to start fake cluster: %v", err)
	}
	defer cluster.Close()

	kClient, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("failed to create kgo client: %v", err)
	}
	defer kClient.Close()

	p := &Producer[string]{
		cfg: ProducerConfig[string]{
			Topic:      "test-topic",
			Serializer: jsonSerializer[string](),
		},
		client: kClient,
	}

	if err := p.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Stats().MessagesSent != 1 {
		t.Errorf("expected MessagesSent=1, got %d", p.Stats().MessagesSent)
	}
}

func TestProducer_SendBatch_Success(t *testing.T) {
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, "test-topic"),
	)
	if err != nil {
		t.Fatalf("failed to start fake cluster: %v", err)
	}
	defer cluster.Close()

	kClient, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("failed to create kgo client: %v", err)
	}
	defer kClient.Close()

	p := &Producer[string]{
		cfg: ProducerConfig[string]{
			Topic:      "test-topic",
			Serializer: jsonSerializer[string](),
		},
		client: kClient,
	}

	msgs := []string{"a", "b", "c"}
	if err := p.SendBatch(context.Background(), msgs); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := p.Stats().MessagesSent; got != int64(len(msgs)) {
		t.Errorf("expected MessagesSent=%d, got %d", len(msgs), got)
	}
}

func TestProducer_Close_PreventsSubsequentSend(t *testing.T) {
	p := &Producer[string]{
		cfg: ProducerConfig[string]{
			Topic:      "test-topic",
			Serializer: jsonSerializer[string](),
		},
	}
	p.Close()

	err := p.Send(context.Background(), "hello")
	if !errors.Is(err, ErrProducerClosed) {
		t.Fatalf("expected ErrProducerClosed after Close(), got %v", err)
	}
}
