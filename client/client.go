// Package client provides a client for interacting with the DZSA API.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jsirianni/dzsa-sync/internal/metrics"
	"github.com/jsirianni/dzsa-sync/model"
)

const (
	baseURL = "https://dayzsalauncher.com/api/v1/query"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 15 * time.Second

// Client interacts with the DZSA API.
type Client interface {
	Query(ctx context.Context, ip string, port int) (*model.QueryResponse, error)
}

// Options configures the client.
type Options struct {
	HTTPClient *http.Client
	Recorder   metrics.HTTPRecorder
}

// New creates a new DZSA client.
func New(opts Options) Client {
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	return &defaultClient{
		baseURL:  baseURL,
		client:   hc,
		recorder: opts.Recorder,
	}
}

type defaultClient struct {
	baseURL  string
	client   *http.Client
	recorder metrics.HTTPRecorder
}

var _ Client = (*defaultClient)(nil)

// Query registers/query the server at ip:port with DZSA and returns the response.
func (c *defaultClient) Query(ctx context.Context, ip string, port int) (*model.QueryResponse, error) {
	start := time.Now()
	host := "dzsa"
	var statusCode int

	endpoint, err := buildEndpoint(c.baseURL, ip, port)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, 0, metrics.ClassifyError(err, 0), time.Since(start))
		}
		return nil, fmt.Errorf("build endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, 0, metrics.ClassifyError(err, 0), time.Since(start))
		}
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "dzsa-sync/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, 0, metrics.ClassifyError(err, 0), time.Since(start))
		}
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ClassifyError(nil, statusCode), time.Since(start))
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorDecode, time.Since(start))
		}
		return nil, fmt.Errorf("read response: %w", err)
	}

	rawReq := make(map[string]any)
	if err := json.Unmarshal(b, &rawReq); err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorDecode, time.Since(start))
		}
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if _, ok := rawReq["error"]; ok {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorStatus4xx, time.Since(start))
		}
		return nil, fmt.Errorf("api error: %v", rawReq["error"])
	}

	queryResponse := &model.QueryResponse{}
	if err := json.Unmarshal(b, queryResponse); err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorDecode, time.Since(start))
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if c.recorder != nil {
		c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorNone, time.Since(start))
	}
	return queryResponse, nil
}

func buildEndpoint(base, ip string, port int) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse base: %w", err)
	}
	ipPort := net.JoinHostPort(ip, strconv.Itoa(port))
	path, err := url.JoinPath(u.Path, ipPort)
	if err != nil {
		return "", fmt.Errorf("join path %s: %w", ipPort, err)
	}
	u.Path = path
	return u.String(), nil
}
