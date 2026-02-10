package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sethgrid/helloworld/metrics"
)

// APIClient wraps an HTTP client with metrics collection
type APIClient struct {
	client  *http.Client
	service string
}

// NewAPIClient creates a new API client with metrics collection
func NewAPIClient(service string, timeout time.Duration) *APIClient {
	return &APIClient{
		client: &http.Client{
			Timeout: timeout,
		},
		service: service,
	}
}

// Do performs an HTTP request and records metrics
func (c *APIClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	start := time.Now()
	
	// Add context to request
	req = req.WithContext(ctx)
	
	resp, err := c.client.Do(req)
	duration := time.Since(start)
	
	// Record metrics
	endpoint := req.URL.Path
	method := req.Method
	status := "unknown"
	if resp != nil {
		status = fmt.Sprintf("%d", resp.StatusCode)
	} else if err != nil {
		status = "error"
	}
	
	metrics.APICallDuration.WithLabelValues(c.service, endpoint, method).Observe(duration.Seconds())
	metrics.APICallCount.WithLabelValues(c.service, endpoint, method, status).Inc()
	
	return resp, err
}

// Get performs a GET request
func (c *APIClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// Post performs a POST request
func (c *APIClient) Post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}
