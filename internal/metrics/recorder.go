// Package metrics provides OpenTelemetry metrics for dzsa-sync.
package metrics // revive:disable-line:var-naming

import (
	"context"
	"time"
)

// HTTPRecorder records HTTP request metrics (count and latency).
// Implementations are used by the DZSA client and ifconfig client.
type HTTPRecorder interface {
	RecordRequest(ctx context.Context, host string, statusCode int, errType string, duration time.Duration)
}
