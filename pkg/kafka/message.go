package kafka

import (
	"context"
	"encoding/json"
	"time"
)

// Serializer converts a value of type T into bytes for the Kafka record value.
type Serializer[T any] func(v T) ([]byte, error)

// Deserializer converts raw Kafka record bytes into a value of type T.
type Deserializer[T any] func(data []byte) (T, error)

// Message wraps Kafka record metadata with a typed, deserialized payload.
type Message[T any] struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Payload   T
	Headers   map[string]string
	Timestamp time.Time
}

// Handler is the interface consumers call for each received message.
// Returning a non-nil error triggers the consumer's retry/DLQ logic.
type Handler[T any] interface {
	Handle(ctx context.Context, msg Message[T]) error
}

// HandlerFunc is a function adapter for Handler[T], analogous to http.HandlerFunc.
type HandlerFunc[T any] func(ctx context.Context, msg Message[T]) error

func (f HandlerFunc[T]) Handle(ctx context.Context, msg Message[T]) error {
	return f(ctx, msg)
}

// jsonSerializer returns a Serializer[T] that encodes values as JSON.
// Used as the default when ProducerConfig.Serializer is nil.
func jsonSerializer[T any]() Serializer[T] {
	return func(v T) ([]byte, error) {
		return json.Marshal(v)
	}
}

// jsonDeserializer returns a Deserializer[T] that decodes JSON bytes.
// Used as the default when ConsumerConfig.Deserializer is nil.
func jsonDeserializer[T any]() Deserializer[T] {
	return func(data []byte) (T, error) {
		var v T
		if err := json.Unmarshal(data, &v); err != nil {
			var zero T
			return zero, err
		}
		return v, nil
	}
}
