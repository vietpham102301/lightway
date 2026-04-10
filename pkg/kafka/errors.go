package kafka

import "errors"

var (
	ErrBrokerUnavailable     = errors.New("kafka: broker unavailable")
	ErrConsumerClosed        = errors.New("kafka: consumer is closed")
	ErrProducerClosed        = errors.New("kafka: producer is closed")
	ErrSerializationFailed   = errors.New("kafka: serialization failed")
	ErrDeserializationFailed = errors.New("kafka: deserialization failed")
)
