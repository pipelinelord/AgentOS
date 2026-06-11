package io

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LocalNetDriver struct {
	client *http.Client
}

func NewLocalNetDriver() *LocalNetDriver {
	return &LocalNetDriver{
		client: &http.Client{
			Timeout: 10 * time.Second, // Security: timeout to prevent hanging
		},
	}
}

func (d *LocalNetDriver) Fetch(ctx context.Context, url string) (string, error) {
	// Security: Only allow GET requests
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("invalid request: %v", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Security: limit body size to prevent memory exhaustion (e.g., 5MB)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(bodyBytes), nil
}
