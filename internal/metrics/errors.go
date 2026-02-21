// Package metrics provides OpenTelemetry metrics for dzsa-sync.
package metrics

import (
	"context"
	"errors"
	"net"
	"strings"
)

// Error type attribute values for HTTP request metrics.
const (
	ErrorNone             = "none"
	ErrorTimeout          = "timeout"
	ErrorConnectionRefused = "connection_refused"
	ErrorStatus4xx        = "status_4xx"
	ErrorStatus5xx        = "status_5xx"
	ErrorDecode           = "decode_error"
	ErrorUnknown          = "unknown"
)

// ClassifyError returns the error type for metrics from err and statusCode.
// statusCode is 0 if the request failed before receiving a response.
func ClassifyError(err error, statusCode int) string {
	if err == nil {
		if statusCode >= 200 && statusCode < 300 {
			return ErrorNone
		}
		if statusCode >= 400 && statusCode < 500 {
			return ErrorStatus4xx
		}
		if statusCode >= 500 {
			return ErrorStatus5xx
		}
		return ErrorUnknown
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrorTimeout
		}
		if strings.Contains(err.Error(), "connection refused") {
			return ErrorConnectionRefused
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTimeout
	}
	if statusCode >= 400 && statusCode < 500 {
		return ErrorStatus4xx
	}
	if statusCode >= 500 {
		return ErrorStatus5xx
	}
	return ErrorUnknown
}
