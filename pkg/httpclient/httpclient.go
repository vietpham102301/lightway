package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vietpham102301/hermes/pkg/logger"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	t := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: t,
			Timeout:   60 * time.Second,
		},
	}
}

func (c *Client) RequestBytes(ctx context.Context, method, url string, body any, headers map[string]string) ([]byte, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	reqBody := bytes.NewBuffer(jsonBytes)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request %w", err)
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
		return nil, fmt.Errorf("failed to execute request %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body %w", err)
	}

	if resp.StatusCode >= 400 {
		logger.Warn("request failed",
			"url", url,
			"status", resp.StatusCode,
			"body", string(respBody),
		)
		return respBody, fmt.Errorf("api error status %d", resp.StatusCode)
	}

	return respBody, nil
}

// Do sends the request and returns the response. Caller must close resp.Body.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
