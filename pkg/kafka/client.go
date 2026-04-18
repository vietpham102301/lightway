package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/vietpham102301/lightway/pkg/logger"
)

// ClientConfig holds broker-level connection settings.
// Zero values produce sensible defaults via applyDefaults().
type ClientConfig struct {
	// Brokers is the list of seed broker addresses (host:port). Required.
	Brokers []string

	// DialTimeout is the max time to wait for a broker TCP connection. Default: 10s
	DialTimeout time.Duration

	// RequestTimeout is the per-request timeout. Default: 30s
	RequestTimeout time.Duration

	// TLS optionally enables TLS. nil means plaintext.
	TLS *tls.Config

	// SASL optionally configures SASL authentication.
	SASL *SASLConfig
}

// SASLConfig holds SASL authentication settings.
type SASLConfig struct {
	// Mechanism must be one of: "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	Mechanism string
	Username  string
	Password  string
}

func (c *ClientConfig) applyDefaults() {
	if c.DialTimeout <= 0 {
		c.DialTimeout = 10 * time.Second
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 30 * time.Second
	}
}

// Client wraps a franz-go kgo.Client and is shared between Producer and Consumer instances.
type Client struct {
	kgo      *kgo.Client
	brokers  []string
	authOpts []kgo.Opt // SASL + TLS options, reused when creating per-consumer clients
}

// NewClient creates and validates a Kafka broker connection.
// Returns ErrBrokerUnavailable if no broker can be reached.
func NewClient(cfg ClientConfig) (*Client, error) {
	cfg.applyDefaults()

	var authOpts []kgo.Opt

	if cfg.TLS != nil {
		authOpts = append(authOpts, kgo.DialTLSConfig(cfg.TLS))
	}

	if cfg.SASL != nil {
		saslOpt, err := buildSASL(cfg.SASL)
		if err != nil {
			return nil, err
		}
		authOpts = append(authOpts, saslOpt)
	}

	opts := append([]kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.DialTimeout(cfg.DialTimeout),
		kgo.RequestTimeoutOverhead(cfg.RequestTimeout),
	}, authOpts...)

	kClient, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBrokerUnavailable, err)
	}

	// Ping broker to validate connectivity.
	if err := kClient.Ping(context.TODO()); err != nil {
		kClient.Close()
		return nil, fmt.Errorf("%w: %w", ErrBrokerUnavailable, err)
	}

	logger.Info("kafka: broker connection established", "brokers", cfg.Brokers)

	return &Client{kgo: kClient, brokers: cfg.Brokers, authOpts: authOpts}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() {
	c.kgo.Close()
}

func buildSASL(cfg *SASLConfig) (kgo.Opt, error) {
	switch cfg.Mechanism {
	case "PLAIN":
		auth := plain.Auth{User: cfg.Username, Pass: cfg.Password}
		return kgo.SASL(auth.AsMechanism()), nil
	case "SCRAM-SHA-256":
		auth := scram.Auth{User: cfg.Username, Pass: cfg.Password}
		return kgo.SASL(auth.AsSha256Mechanism()), nil
	case "SCRAM-SHA-512":
		auth := scram.Auth{User: cfg.Username, Pass: cfg.Password}
		return kgo.SASL(auth.AsSha512Mechanism()), nil
	default:
		return nil, fmt.Errorf("kafka: unsupported SASL mechanism %q", cfg.Mechanism)
	}
}
