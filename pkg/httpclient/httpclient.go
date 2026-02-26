package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/vietpham102301/lightway/pkg/logger"
)

// Config holds the configuration for the HTTP client.
// Zero values for fields will use sensible defaults.
type Config struct {
	MaxIdleConns        int           // default: 100
	MaxIdleConnsPerHost int           // default: 10
	IdleConnTimeout     time.Duration // default: 90s
	Timeout             time.Duration // default: 60s
}

// RetryConfig holds the configuration for retry behavior.
// Zero values for fields will use sensible defaults.
type RetryConfig struct {
	MaxRetries  int                                       // default: 3
	BaseDelay   time.Duration                             // default: 500ms (doubles each retry)
	MaxDelay    time.Duration                             // default: 10s
	RetryOn     []int                                     // HTTP status codes to retry on; default: 429, 502, 503, 504
	ShouldRetry func(resp *http.Response, err error) bool // custom retry decision; overrides RetryOn if set
}

func (c *Config) applyDefaults() {
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 100
	}
	if c.MaxIdleConnsPerHost <= 0 {
		c.MaxIdleConnsPerHost = 10
	}
	if c.IdleConnTimeout <= 0 {
		c.IdleConnTimeout = 90 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 60 * time.Second
	}
}

func (r *RetryConfig) applyDefaults() {
	if r.MaxRetries <= 0 {
		r.MaxRetries = 3
	}
	if r.BaseDelay <= 0 {
		r.BaseDelay = 500 * time.Millisecond
	}
	if r.MaxDelay <= 0 {
		r.MaxDelay = 10 * time.Second
	}
	if len(r.RetryOn) == 0 && r.ShouldRetry == nil {
		r.RetryOn = []int{
			http.StatusTooManyRequests,    // 429
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
		}
	}
}

// backoff returns the delay for the given attempt using exponential backoff.
// attempt is 0-indexed: attempt 0 = BaseDelay, attempt 1 = BaseDelay*2, etc.
func (r *RetryConfig) backoff(attempt int) time.Duration {
	delay := time.Duration(float64(r.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > r.MaxDelay {
		delay = r.MaxDelay
	}
	return delay
}

// isRetryable checks whether a response/error should be retried.
func (r *RetryConfig) isRetryable(resp *http.Response, err error) bool {
	if r.ShouldRetry != nil {
		return r.ShouldRetry(resp, err)
	}
	// Network errors are retryable
	if err != nil {
		return true
	}
	for _, code := range r.RetryOn {
		if resp.StatusCode == code {
			return true
		}
	}
	return false
}

type Client struct {
	httpClient  *http.Client
	retryConfig *RetryConfig
}

// NewClient creates a new HTTP client with default configuration and no retry.
func NewClient() *Client {
	return NewClientWithConfig(Config{})
}

// NewClientWithConfig creates a new HTTP client with the provided configuration.
func NewClientWithConfig(cfg Config) *Client {
	cfg.applyDefaults()

	t := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: t,
			Timeout:   cfg.Timeout,
		},
	}
}

// WithRetry returns a new Client with retry enabled using the given configuration.
func (c *Client) WithRetry(cfg RetryConfig) *Client {
	cfg.applyDefaults()
	return &Client{
		httpClient:  c.httpClient,
		retryConfig: &cfg,
	}
}

func (c *Client) RequestBytes(ctx context.Context, method, url string, body any, headers map[string]string) ([]byte, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	maxAttempts := 1
	var retryCfg RetryConfig
	if c.retryConfig != nil {
		retryCfg = *c.retryConfig
		maxAttempts = 1 + retryCfg.MaxRetries
	}

	var lastErr error
	var lastBody []byte

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := retryCfg.backoff(attempt - 1)
			logger.Warn("retrying request",
				"url", url,
				"attempt", attempt+1,
				"max_attempts", maxAttempts,
				"delay", delay.String(),
			)
			select {
			case <-ctx.Done():
				return lastBody, ctx.Err()
			case <-time.After(delay):
			}
		}

		reqBody := bytes.NewBuffer(jsonBytes)
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			if strings.EqualFold(k, "Host") {
				req.Host = v
			} else {
				req.Header.Set(k, v)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute request: %w", err)
			if c.retryConfig != nil && retryCfg.isRetryable(nil, err) {
				continue
			}
			return nil, lastErr
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}

		if resp.StatusCode >= 400 {
			lastBody = respBody
			lastErr = fmt.Errorf("api error status %d", resp.StatusCode)

			if c.retryConfig != nil && retryCfg.isRetryable(resp, nil) {
				logger.Warn("retryable error",
					"url", url,
					"status", resp.StatusCode,
				)
				continue
			}

			logger.Warn("request failed",
				"url", url,
				"status", resp.StatusCode,
				"body", string(respBody),
			)
			return respBody, lastErr
		}

		return respBody, nil
	}

	logger.Error("all retry attempts exhausted",
		"url", url,
		"attempts", maxAttempts,
	)
	return lastBody, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// Do sends the request and returns the response. Caller must close resp.Body.
// Note: Do does not apply retry logic. Use RequestBytes for automatic retries.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
