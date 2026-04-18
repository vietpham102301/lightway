package kafka

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/vietpham102301/lightway/pkg/logger"
)

// DLQConfig enables dead letter queue publishing for messages that exhaust retries.
type DLQConfig struct {
	// Topic is the DLQ topic name. Required.
	Topic string
}

// ConsumerConfig holds all settings for a Consumer[T].
type ConsumerConfig[T any] struct {
	// GroupID is the Kafka consumer group ID. Required.
	GroupID string

	// Topics is the list of topics to subscribe to. Required.
	Topics []string

	// Deserializer converts []byte → T. Default: JSON decoding.
	Deserializer Deserializer[T]

	// DisableAutoCommit disables automatic offset committing.
	// When false (default), offsets are committed after each successful batch.
	DisableAutoCommit bool

	// MaxRetries is the number of handler retry attempts before DLQ or drop. Default: 3
	MaxRetries int

	// RetryBaseDelay is the initial retry backoff. Default: 500ms
	RetryBaseDelay time.Duration

	// RetryMaxDelay caps exponential backoff. Default: 10s
	RetryMaxDelay time.Duration

	// DLQ optionally configures a dead letter queue.
	// When nil, failed messages are logged and dropped after MaxRetries.
	DLQ *DLQConfig

	// SessionTimeout is the Kafka consumer group session timeout. Default: 10s
	SessionTimeout time.Duration

	// RebalanceTimeout is the max time allowed for a rebalance. Default: 30s
	RebalanceTimeout time.Duration
}

func (c *ConsumerConfig[T]) applyDefaults() {
	if c.Deserializer == nil {
		c.Deserializer = jsonDeserializer[T]()
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = 500 * time.Millisecond
	}
	if c.RetryMaxDelay <= 0 {
		c.RetryMaxDelay = 10 * time.Second
	}
	if c.SessionTimeout <= 0 {
		c.SessionTimeout = 10 * time.Second
	}
	if c.RebalanceTimeout <= 0 {
		c.RebalanceTimeout = 30 * time.Second
	}
}

// Consumer[T] subscribes to Kafka topics and dispatches messages to a Handler[T].
type Consumer[T any] struct {
	cfg     ConsumerConfig[T]
	handler Handler[T]
	client  *kgo.Client
	closed  atomic.Bool
	stats   consumerStats
}

// NewConsumer creates a Consumer[T] backed by the given Client.
// The consumer does not begin reading until Start() is called.
func NewConsumer[T any](client *Client, cfg ConsumerConfig[T], handler Handler[T]) (*Consumer[T], error) {
	cfg.applyDefaults()

	opts := append([]kgo.Opt{
		kgo.SeedBrokers(client.brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.SessionTimeout(cfg.SessionTimeout),
		kgo.RebalanceTimeout(cfg.RebalanceTimeout),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
	}, client.authOpts...)

	if !cfg.DisableAutoCommit {
		opts = append(opts, kgo.AutoCommitMarks())
	}

	kClient, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("kafka: failed to create consumer client: %w", err)
	}

	return &Consumer[T]{
		cfg:     cfg,
		handler: handler,
		client:  kClient,
	}, nil
}

// Start begins consuming messages from the configured topics.
// It blocks until ctx is cancelled, then drains in-flight messages and returns.
func (c *Consumer[T]) Start(ctx context.Context) error {
	defer func() {
		c.closed.Store(true)
		c.client.Close()
	}()

	logger.Info("kafka: consumer started",
		"group", c.cfg.GroupID,
		"topics", c.cfg.Topics,
	)

	for {
		fetches := c.client.PollFetches(ctx)

		if ctx.Err() != nil {
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				logger.Error("kafka: fetch error",
					"topic", fe.Topic,
					"partition", fe.Partition,
					"err", fe.Err,
				)
			}
		}

		fetches.EachRecord(func(r *kgo.Record) {
			c.processRecord(ctx, r)
		})

		if !c.cfg.DisableAutoCommit {
			if err := c.client.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
				logger.Error("kafka: failed to commit offsets", "err", err)
			}
		}
	}
}

// Close stops the consumer. It is safe to call if Start has not been called.
func (c *Consumer[T]) Close() {
	if c.closed.CompareAndSwap(false, true) {
		c.client.Close()
	}
}

// Stats returns a point-in-time snapshot of consumer metrics.
func (c *Consumer[T]) Stats() ConsumerSnapshot {
	return ConsumerSnapshot{
		MessagesReceived:      c.stats.received.Load(),
		MessagesProcessed:     c.stats.processed.Load(),
		HandlerErrors:         c.stats.handlerErrors.Load(),
		DeserializationErrors: c.stats.deserErrors.Load(),
		DLQErrors:             c.stats.dlqErrors.Load(),
		Panics:                c.stats.panics.Load(),
	}
}

// processRecord deserializes one Kafka record and dispatches it to the handler with retry.
func (c *Consumer[T]) processRecord(ctx context.Context, r *kgo.Record) {
	defer func() {
		if rec := recover(); rec != nil {
			c.stats.panics.Add(1)
			logger.Error("kafka: handler panicked",
				"topic", r.Topic,
				"partition", r.Partition,
				"offset", r.Offset,
				"panic", rec,
			)
		}
	}()

	c.stats.received.Add(1)

	payload, err := c.cfg.Deserializer(r.Value)
	if err != nil {
		c.stats.deserErrors.Add(1)
		logger.Error("kafka: deserialization failed",
			"topic", r.Topic,
			"partition", r.Partition,
			"offset", r.Offset,
			"err", err,
		)
		c.sendToDLQ(ctx, r, fmt.Errorf("%w: %w", ErrDeserializationFailed, err))
		return
	}

	msg := Message[T]{
		Topic:     r.Topic,
		Partition: r.Partition,
		Offset:    r.Offset,
		Key:       r.Key,
		Payload:   payload,
		Timestamp: r.Timestamp,
	}
	if len(r.Headers) > 0 {
		msg.Headers = make(map[string]string, len(r.Headers))
		for _, h := range r.Headers {
			msg.Headers[string(h.Key)] = string(h.Value)
		}
	}

	maxAttempts := 1 + c.cfg.MaxRetries
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := retryBackoff(c.cfg.RetryBaseDelay, c.cfg.RetryMaxDelay, attempt-1)
			logger.Warn("kafka: retrying message handler",
				"topic", r.Topic,
				"offset", r.Offset,
				"attempt", attempt+1,
				"delay", delay.String(),
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		if err := c.handler.Handle(ctx, msg); err != nil {
			lastErr = err
			c.stats.handlerErrors.Add(1)
			continue
		}

		c.stats.processed.Add(1)
		return
	}

	logger.Error("kafka: message handler failed after all retries",
		"topic", r.Topic,
		"offset", r.Offset,
		"attempts", maxAttempts,
		"err", lastErr,
	)
	c.sendToDLQ(ctx, r, lastErr)
}

// sendToDLQ publishes the raw record to the dead letter queue topic if configured.
func (c *Consumer[T]) sendToDLQ(ctx context.Context, r *kgo.Record, cause error) {
	if c.cfg.DLQ == nil {
		return
	}

	dlqRecord := &kgo.Record{
		Topic: c.cfg.DLQ.Topic,
		Key:   r.Key,
		Value: r.Value,
		Headers: []kgo.RecordHeader{
			{Key: "x-original-topic", Value: []byte(r.Topic)},
			{Key: "x-original-partition", Value: []byte(fmt.Sprintf("%d", r.Partition))},
			{Key: "x-original-offset", Value: []byte(fmt.Sprintf("%d", r.Offset))},
			{Key: "x-error", Value: []byte(cause.Error())},
		},
	}

	results := c.client.ProduceSync(ctx, dlqRecord)
	if err := results.FirstErr(); err != nil {
		c.stats.dlqErrors.Add(1)
		logger.Error("kafka: DLQ publish failed",
			"dlq_topic", c.cfg.DLQ.Topic,
			"err", err,
		)
	}
}

// retryBackoff computes exponential backoff capped at maxDelay.
func retryBackoff(base, max time.Duration, attempt int) time.Duration {
	d := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if d > max {
		d = max
	}
	return d
}
