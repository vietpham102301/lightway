package kafka

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/vietpham102301/lightway/pkg/logger"
)

// PartitionKeyFunc extracts a string partition key from a value.
// Returning "" uses franz-go's default partitioner (uniform bytes).
type PartitionKeyFunc[T any] func(v T) string

// ProducerConfig holds all settings for a Producer[T].
type ProducerConfig[T any] struct {
	// Topic is the default topic to publish to. Required.
	Topic string

	// Serializer converts T → []byte. Default: JSON encoding.
	Serializer Serializer[T]

	// PartitionKey extracts a routing key from the value. Optional.
	PartitionKey PartitionKeyFunc[T]

	// Async controls whether Send returns before broker acknowledgement.
	// Async=true gives higher throughput; errors are counted in Stats().
	// Default: false (sync, waits for broker ack).
	Async bool

	// Linger is the time to wait before flushing a batch (async mode). Default: 100ms
	Linger time.Duration

	// MaxBufferedRecords caps the number of records buffered before blocking (async). Default: 1000
	MaxBufferedRecords int

	// RecordRetries is the number of retries for transient produce errors. Default: 3
	RecordRetries int

	// DeliveryTimeout is the max time to wait for a record to be delivered. Default: 30s
	DeliveryTimeout time.Duration
}

func (c *ProducerConfig[T]) applyDefaults() {
	if c.Serializer == nil {
		c.Serializer = jsonSerializer[T]()
	}
	if c.Linger <= 0 {
		c.Linger = 100 * time.Millisecond
	}
	if c.MaxBufferedRecords <= 0 {
		c.MaxBufferedRecords = 1000
	}
	if c.RecordRetries <= 0 {
		c.RecordRetries = 3
	}
	if c.DeliveryTimeout <= 0 {
		c.DeliveryTimeout = 30 * time.Second
	}
}

// Producer[T] serializes values of type T and publishes them to Kafka.
// It wraps the shared Client's kgo.Client for produce operations.
type Producer[T any] struct {
	cfg    ProducerConfig[T]
	client *kgo.Client
	closed atomic.Bool
	stats  producerStats
}

// NewProducer creates a Producer[T] backed by the given Client.
func NewProducer[T any](client *Client, cfg ProducerConfig[T]) *Producer[T] {
	cfg.applyDefaults()
	return &Producer[T]{
		cfg:    cfg,
		client: client.kgo,
	}
}

// Send serializes v and publishes it to the configured topic.
// In sync mode (default) it blocks until the broker acknowledges.
// In async mode it enqueues the record and returns immediately;
// errors are reflected in Stats().Errors.
func (p *Producer[T]) Send(ctx context.Context, v T) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	data, err := p.cfg.Serializer(v)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSerializationFailed, err)
	}

	record := &kgo.Record{
		Topic: p.cfg.Topic,
		Value: data,
	}
	if p.cfg.PartitionKey != nil {
		record.Key = []byte(p.cfg.PartitionKey(v))
	}

	if p.cfg.Async {
		p.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
			if err != nil {
				p.stats.errors.Add(1)
				logger.Error("kafka: async produce error", "topic", p.cfg.Topic, "err", err)
				return
			}
			p.stats.sent.Add(1)
		})
		return nil
	}

	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		p.stats.errors.Add(1)
		return fmt.Errorf("kafka: failed to produce record: %w", err)
	}
	p.stats.sent.Add(1)
	return nil
}

// SendBatch serializes and sends multiple values.
// In sync mode it sends all records in a single ProduceSync call for efficiency.
// In async mode it enqueues each record individually.
func (p *Producer[T]) SendBatch(ctx context.Context, values []T) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	if p.cfg.Async {
		for _, v := range values {
			if err := p.Send(ctx, v); err != nil {
				return err
			}
		}
		return nil
	}

	records := make([]*kgo.Record, 0, len(values))
	for _, v := range values {
		data, err := p.cfg.Serializer(v)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSerializationFailed, err)
		}
		record := &kgo.Record{Topic: p.cfg.Topic, Value: data}
		if p.cfg.PartitionKey != nil {
			record.Key = []byte(p.cfg.PartitionKey(v))
		}
		records = append(records, record)
	}

	results := p.client.ProduceSync(ctx, records...)
	if err := results.FirstErr(); err != nil {
		p.stats.errors.Add(1)
		return fmt.Errorf("kafka: failed to produce batch: %w", err)
	}
	p.stats.sent.Add(int64(len(records)))
	return nil
}

// Flush waits until all buffered async records have been delivered.
// No-op in sync mode.
func (p *Producer[T]) Flush(ctx context.Context) error {
	if !p.cfg.Async {
		return nil
	}
	return p.client.Flush(ctx)
}

// Close marks the producer as closed. Subsequent Send calls return ErrProducerClosed.
// The underlying kgo.Client is managed by the parent Client — call Client.Close() to
// release broker connections.
func (p *Producer[T]) Close() {
	p.closed.Store(true)
}

// Stats returns a point-in-time snapshot of producer metrics.
func (p *Producer[T]) Stats() ProducerSnapshot {
	return ProducerSnapshot{
		MessagesSent: p.stats.sent.Load(),
		Errors:       p.stats.errors.Load(),
	}
}
