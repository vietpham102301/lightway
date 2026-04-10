package kafka

import "sync/atomic"

// producerStats holds live atomic counters for a Producer.
type producerStats struct {
	sent   atomic.Int64
	errors atomic.Int64
}

// ProducerSnapshot is a point-in-time read of producer metrics.
type ProducerSnapshot struct {
	MessagesSent int64
	Errors       int64
}

// consumerStats holds live atomic counters for a Consumer.
type consumerStats struct {
	received      atomic.Int64
	processed     atomic.Int64
	handlerErrors atomic.Int64
	deserErrors   atomic.Int64
	dlqErrors     atomic.Int64
	panics        atomic.Int64
}

// ConsumerSnapshot is a point-in-time read of consumer metrics.
type ConsumerSnapshot struct {
	MessagesReceived      int64
	MessagesProcessed     int64
	HandlerErrors         int64
	DeserializationErrors int64
	DLQErrors             int64
	Panics                int64
}
