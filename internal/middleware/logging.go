package middleware

import (
	"net/http"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

// responseRecorder wraps http.ResponseWriter to capture status code and size.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

// RequestLogger logs HTTP requests with timing information.
type RequestLogger struct {
	logger *logging.Logger
}

// NewRequestLogger creates a new request logging middleware.
func NewRequestLogger(logger *logging.Logger) *RequestLogger {
	if logger == nil {
		logger = logging.Default
	}
	return &RequestLogger{logger: logger}
}

// Apply wraps the handler to log requests.
func (l *RequestLogger) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status and size
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process request
		next.ServeHTTP(recorder, r)

		// Calculate duration
		duration := time.Since(start)

		// Log request
		fields := map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.statusCode,
			"size":        recorder.size,
			"duration_ms": duration.Milliseconds(),
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}

		// Add query string if present
		if r.URL.RawQuery != "" {
			fields["query"] = r.URL.RawQuery
		}

		// Choose log level based on status code
		switch {
		case recorder.statusCode >= 500:
			l.logger.Error("HTTP request", fields)
		case recorder.statusCode >= 400:
			l.logger.Warn("HTTP request", fields)
		default:
			l.logger.Info("HTTP request", fields)
		}
	})
}
