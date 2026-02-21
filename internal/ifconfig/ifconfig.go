// Package ifconfig detects the host's public IP via ifconfig.net.
package ifconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jsirianni/dzsa-sync/internal/metrics"
	"go.uber.org/zap"
)

const (
	endpoint = "https://ifconfig.net/json"
)

// Response is the response from the ifconfig.net service.
type Response struct {
	IP         string  `json:"ip"`
	IPDecimal  int     `json:"ip_decimal"`
	Country    string  `json:"country"`
	CountryIso string  `json:"country_iso"`
	CountryEu  bool    `json:"country_eu"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	TimeZone   string  `json:"time_zone"`
	Asn        string  `json:"asn"`
	AsnOrg     string  `json:"asn_org"`
	Hostname   string  `json:"hostname"`
	UserAgent  struct {
		Product  string `json:"product"`
		Version  string `json:"version"`
		Comment  string `json:"comment"`
		RawValue string `json:"raw_value"`
	} `json:"user_agent"`
}

// Client detects public IP using ifconfig.net.
type Client struct {
	client   *http.Client
	logger   *zap.Logger
	recorder metrics.HTTPRecorder
	address  string
	mu       sync.Mutex
	// BaseURL overrides the default endpoint when set (e.g. for tests).
	BaseURL string
}

// New creates a new ifconfig client. httpClient may be nil to use a default client.
func New(logger *zap.Logger, httpClient *http.Client, recorder metrics.HTTPRecorder) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		client:   httpClient,
		logger:   logger,
		recorder: recorder,
	}
}

// Get fetches the current public IP from ifconfig.net.
func (c *Client) Get(ctx context.Context) (*Response, error) {
	start := time.Now()
	host := "ifconfig"
	var statusCode int

	url := endpoint
	if c.BaseURL != "" {
		url = c.BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, 0, metrics.ClassifyError(err, 0), time.Since(start))
		}
		return nil, err
	}
	req.Header.Set("User-Agent", "dzsa-sync/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, 0, metrics.ClassifyError(err, 0), time.Since(start))
		}
		return nil, err
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ClassifyError(nil, statusCode), time.Since(start))
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		if c.recorder != nil {
			c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorDecode, time.Since(start))
		}
		return nil, err
	}
	if c.recorder != nil {
		c.recorder.RecordRequest(ctx, host, statusCode, metrics.ErrorNone, time.Since(start))
	}
	return &r, nil
}

// GetAddress returns the last successfully detected IP (updated by Run loop).
func (c *Client) GetAddress() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.address
}

// SetAddress updates the cached address (e.g. from config when DetectIP is false).
func (c *Client) SetAddress(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.address = ip
}

// Run runs the IP detection loop every 10 minutes. When the IP changes, onChanged is called.
// Run blocks until ctx is cancelled.
func (c *Client) Run(ctx context.Context, onChanged func(oldIP, newIP string)) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Initial fetch
	resp, err := c.Get(ctx)
	if err != nil {
		c.logger.Error("ifconfig initial get failed", zap.Error(err))
	} else if resp.IP != "" {
		c.mu.Lock()
		c.address = resp.IP
		c.mu.Unlock()
		c.logger.Info("ifconfig sync completed", zap.String("detected_ip", resp.IP))
	}

	for {
		select {
		case <-ticker.C:
			resp, err := c.Get(ctx)
			if err != nil {
				c.logger.Error("ifconfig get failed", zap.Error(err))
				continue
			}
			if resp.IP == "" {
				c.logger.Warn("ifconfig returned empty IP")
				continue
			}
			c.logger.Info("ifconfig sync completed", zap.String("detected_ip", resp.IP))
			c.mu.Lock()
			old := c.address
			c.address = resp.IP
			c.mu.Unlock()
			if old != "" && old != resp.IP && onChanged != nil {
				onChanged(old, resp.IP)
			}
		case <-ctx.Done():
			c.logger.Info("ifconfig loop shutting down")
			return
		}
	}
}
