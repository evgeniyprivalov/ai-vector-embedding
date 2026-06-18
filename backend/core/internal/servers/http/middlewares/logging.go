package server

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
)

// LoggingMiddleware logs information about each request.
func LoggingMiddleware(logger *log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			logRequestDetails(r.Context(), logger, r)

			ww := httpsnoop.CaptureMetrics(next, w, r)

			logger.WithCtx(r.Context()).Info("Response",
				"timestamp", time.Now().Format(time.RFC3339),
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", extractContextValue(r.Context(), RequestIDKeyString, "none"),
				"status_code", ww.Code,
				"latency_ms", time.Since(startTime).Milliseconds(),
				"bytes_written", ww.Written,
			)
		})
	}
}

// logRequestDetails logs detailed information about the incoming request
//
//nolint:errchkjson
func logRequestDetails(ctx context.Context, logger *log.Logger, r *http.Request) {
	// Gather header information
	requestID := extractContextValue(ctx, RequestIDKeyString, "none")
	clientIP := getClientIP(r)

	// Get operation name if available from context
	operation := extractContextValue(ctx, OperationKey{}, "unknown")

	// Log with separate fields
	logger.WithCtx(ctx).Info("Request",
		"timestamp", time.Now().Format(time.RFC3339),
		"method", r.Method,
		"path", r.URL.Path,
		"operation", operation,
		"request_id", requestID,
		"client_ip", clientIP,
		"user_agent", r.UserAgent(),
		"query_params", r.URL.Query().Encode(),
		"headers", r.Header,
		"request_body", getRequestBodyForLog(r),
	)
}

// Helper function to extract values from context.
func extractContextValue(ctx context.Context, key interface{}, defaultValue string) string {
	if value, ok := ctx.Value(key).(string); ok {
		return value
	}
	return defaultValue
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first (common for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try X-Real-IP header next (used by some proxies)
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Strip port if present
	if i := strings.LastIndex(ip, ":"); i != -1 {
		ip = ip[:i]
	}
	return ip
}

func getRequestBodyForLog(r *http.Request) string {
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		return "[multipart body omitted]"

	case strings.HasPrefix(contentType, "application/octet-stream"):
		return "[binary body omitted]"

	case strings.HasPrefix(contentType, "application/pdf"):
		return "[pdf body omitted]"
	}

	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		return ""
	}

	return string(dump)
}
