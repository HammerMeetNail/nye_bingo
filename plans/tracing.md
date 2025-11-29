# OpenTelemetry Tracing Implementation Plan

This plan details how to add distributed tracing to Year of Bingo using OpenTelemetry with Honeycomb's free tier as the backend.

## Overview

**Goal**: Add comprehensive observability to understand request flows, identify performance bottlenecks, and debug production issues.

**Backend**: Honeycomb Free Tier
- 20 events/second limit
- 20 million events/month
- 60-day retention
- Full query capabilities

**Estimated Implementation**: 5 phases, can be done incrementally

---

## Phase 1: Core Infrastructure

### 1.1 Add Dependencies

Add to `go.mod`:

```bash
go get go.opentelemetry.io/otel@v1.28.0
go get go.opentelemetry.io/otel/sdk@v1.28.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.28.0
go get go.opentelemetry.io/otel/semconv/v1.26.0@v1.28.0
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.53.0
```

### 1.2 Create Configuration

Add to `internal/config/config.go`:

```go
type TelemetryConfig struct {
    Enabled          bool    `json:"enabled"`
    ServiceName      string  `json:"service_name"`
    ServiceVersion   string  `json:"service_version"`
    Environment      string  `json:"environment"`
    ExporterEndpoint string  `json:"exporter_endpoint"`
    ExporterHeaders  string  `json:"exporter_headers"` // "key=value,key2=value2" format
    SamplingRate     float64 `json:"sampling_rate"`    // 0.0-1.0
}
```

Add to `Config` struct:

```go
type Config struct {
    // ... existing fields ...
    Telemetry TelemetryConfig
}
```

Add to `Load()` function:

```go
Telemetry: TelemetryConfig{
    Enabled:          getEnvBool("OTEL_ENABLED", true),
    ServiceName:      getEnv("OTEL_SERVICE_NAME", "yearofbingo"),
    ServiceVersion:   getEnv("OTEL_SERVICE_VERSION", "1.0.9"),
    Environment:      getEnv("OTEL_ENVIRONMENT", "development"),
    ExporterEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
    ExporterHeaders:  getEnv("OTEL_EXPORTER_OTLP_HEADERS", ""),
    SamplingRate:     getEnvFloat("OTEL_SAMPLING_RATE", 1.0),
},
```

Add helper function:

```go
func getEnvFloat(key string, defaultVal float64) float64 {
    val := os.Getenv(key)
    if val == "" {
        return defaultVal
    }
    f, err := strconv.ParseFloat(val, 64)
    if err != nil {
        return defaultVal
    }
    return f
}
```

### 1.3 Create Tracer Provider

Create `internal/telemetry/tracer.go`:

```go
package telemetry

import (
    "context"
    "fmt"
    "strings"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    "go.opentelemetry.io/otel/trace"
)

// TracerConfig holds configuration for the tracer provider
type TracerConfig struct {
    ServiceName      string
    ServiceVersion   string
    Environment      string
    ExporterEndpoint string
    ExporterHeaders  map[string]string
    SamplingRate     float64
}

// TracerProvider wraps the OpenTelemetry tracer provider with helper methods
type TracerProvider struct {
    provider *sdktrace.TracerProvider
    tracer   trace.Tracer
}

// NewTracerProvider creates and configures an OpenTelemetry tracer provider
func NewTracerProvider(ctx context.Context, cfg TracerConfig) (*TracerProvider, error) {
    // Build resource with service information
    res, err := resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(cfg.ServiceName),
            semconv.ServiceVersion(cfg.ServiceVersion),
            attribute.String("environment", cfg.Environment),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }

    // Configure exporter options
    opts := []otlptracehttp.Option{
        otlptracehttp.WithEndpoint(cfg.ExporterEndpoint),
    }

    // Add headers (for Honeycomb API key)
    if len(cfg.ExporterHeaders) > 0 {
        opts = append(opts, otlptracehttp.WithHeaders(cfg.ExporterHeaders))
    }

    // Use insecure for localhost development
    if strings.Contains(cfg.ExporterEndpoint, "localhost") ||
       strings.Contains(cfg.ExporterEndpoint, "127.0.0.1") {
        opts = append(opts, otlptracehttp.WithInsecure())
    }

    // Create OTLP exporter
    exporter, err := otlptracehttp.New(ctx, opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create exporter: %w", err)
    }

    // Configure sampler based on sampling rate
    var sampler sdktrace.Sampler
    if cfg.SamplingRate >= 1.0 {
        sampler = sdktrace.AlwaysSample()
    } else if cfg.SamplingRate <= 0.0 {
        sampler = sdktrace.NeverSample()
    } else {
        sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
    }

    // Create tracer provider
    provider := sdktrace.NewTracerProvider(
        sdktrace.WithResource(res),
        sdktrace.WithBatcher(exporter,
            sdktrace.WithBatchTimeout(5*time.Second),
            sdktrace.WithMaxExportBatchSize(512),
        ),
        sdktrace.WithSampler(sampler),
    )

    // Set global providers
    otel.SetTracerProvider(provider)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return &TracerProvider{
        provider: provider,
        tracer:   provider.Tracer(cfg.ServiceName),
    }, nil
}

// Tracer returns the tracer instance
func (tp *TracerProvider) Tracer() trace.Tracer {
    return tp.tracer
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
    return tp.provider.Shutdown(ctx)
}

// ParseHeaders converts "key=value,key2=value2" format to map
func ParseHeaders(headers string) map[string]string {
    result := make(map[string]string)
    if headers == "" {
        return result
    }
    pairs := strings.Split(headers, ",")
    for _, pair := range pairs {
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
        }
    }
    return result
}
```

### 1.4 Create Context Helpers

Create `internal/telemetry/context.go`:

```go
package telemetry

import (
    "context"

    "go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
    tracerContextKey contextKey = "tracer"
)

// SetTracerInContext stores the tracer in the context
func SetTracerInContext(ctx context.Context, tracer trace.Tracer) context.Context {
    return context.WithValue(ctx, tracerContextKey, tracer)
}

// GetTracerFromContext retrieves the tracer from the context
func GetTracerFromContext(ctx context.Context) trace.Tracer {
    tracer, _ := ctx.Value(tracerContextKey).(trace.Tracer)
    return tracer
}

// StartSpan creates a new span using the tracer from context
// Returns the updated context and span (span may be nil if no tracer in context)
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
    tracer := GetTracerFromContext(ctx)
    if tracer == nil {
        // Return noop span if no tracer configured
        return ctx, trace.SpanFromContext(ctx)
    }
    return tracer.Start(ctx, name, opts...)
}

// GetTraceID extracts the trace ID from context (for log correlation)
func GetTraceID(ctx context.Context) string {
    span := trace.SpanFromContext(ctx)
    if span == nil {
        return ""
    }
    sc := span.SpanContext()
    if !sc.IsValid() {
        return ""
    }
    return sc.TraceID().String()
}

// GetSpanID extracts the span ID from context (for log correlation)
func GetSpanID(ctx context.Context) string {
    span := trace.SpanFromContext(ctx)
    if span == nil {
        return ""
    }
    sc := span.SpanContext()
    if !sc.IsValid() {
        return ""
    }
    return sc.SpanID().String()
}
```

### 1.5 Create Tracing Middleware

Create `internal/middleware/tracing.go`:

```go
package middleware

import (
    "fmt"
    "net/http"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/propagation"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    "go.opentelemetry.io/otel/trace"

    "yearofbingo/internal/telemetry"
)

// TracingMiddleware adds distributed tracing to HTTP requests
type TracingMiddleware struct {
    tracer     trace.Tracer
    propagator propagation.TextMapPropagator
}

// NewTracingMiddleware creates a new tracing middleware instance
func NewTracingMiddleware(tracer trace.Tracer) *TracingMiddleware {
    return &TracingMiddleware{
        tracer: tracer,
        propagator: propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},
            propagation.Baggage{},
        ),
    }
}

// Apply wraps the handler with tracing
func (tm *TracingMiddleware) Apply(next http.Handler) http.Handler {
    if tm.tracer == nil {
        return next // No-op if tracer not configured
    }

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract trace context from incoming request headers
        ctx := tm.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

        // Create span name from HTTP method and route
        spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)

        // Start the span
        ctx, span := tm.tracer.Start(ctx, spanName,
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                semconv.HTTPMethod(r.Method),
                semconv.HTTPURL(r.URL.String()),
                semconv.HTTPScheme(getScheme(r)),
                semconv.NetHostName(r.Host),
                semconv.HTTPUserAgent(r.UserAgent()),
                semconv.NetPeerIP(getClientIP(r)),
            ),
        )
        defer span.End()

        // Store tracer in context for downstream use
        ctx = telemetry.SetTracerInContext(ctx, tm.tracer)

        // Wrap response writer to capture status code
        rw := &tracingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

        // Call the next handler
        next.ServeHTTP(rw, r.WithContext(ctx))

        // Record response attributes
        span.SetAttributes(
            semconv.HTTPStatusCode(rw.statusCode),
            attribute.Int("http.response_size", rw.size),
        )

        // Set span status based on HTTP status code
        if rw.statusCode >= 400 {
            span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
        } else {
            span.SetStatus(codes.Ok, "")
        }
    })
}

// tracingResponseWriter wraps http.ResponseWriter to capture status and size
type tracingResponseWriter struct {
    http.ResponseWriter
    statusCode int
    size       int
}

func (rw *tracingResponseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *tracingResponseWriter) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    rw.size += n
    return n, err
}

// Helper to get scheme from request
func getScheme(r *http.Request) string {
    if r.TLS != nil {
        return "https"
    }
    if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
        return proto
    }
    return "http"
}

// Helper to get client IP from request
func getClientIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return xff
    }
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    return r.RemoteAddr
}
```

### 1.6 Update Main Server

Update `cmd/server/main.go`:

Add import:
```go
import (
    // ... existing imports ...
    "yearofbingo/internal/telemetry"
)
```

In `run()` function, after loading config:

```go
// Initialize telemetry (after config load, before other services)
var tracerProvider *telemetry.TracerProvider
if cfg.Telemetry.Enabled && cfg.Telemetry.ExporterEndpoint != "" {
    tp, err := telemetry.NewTracerProvider(ctx, telemetry.TracerConfig{
        ServiceName:      cfg.Telemetry.ServiceName,
        ServiceVersion:   cfg.Telemetry.ServiceVersion,
        Environment:      cfg.Telemetry.Environment,
        ExporterEndpoint: cfg.Telemetry.ExporterEndpoint,
        ExporterHeaders:  telemetry.ParseHeaders(cfg.Telemetry.ExporterHeaders),
        SamplingRate:     cfg.Telemetry.SamplingRate,
    })
    if err != nil {
        logging.Warn("Failed to initialize tracing, continuing without it", map[string]interface{}{
            "error": err.Error(),
        })
    } else {
        tracerProvider = tp
        logging.Info("Tracing initialized", map[string]interface{}{
            "endpoint":      cfg.Telemetry.ExporterEndpoint,
            "sampling_rate": cfg.Telemetry.SamplingRate,
        })
    }
}
```

Create tracing middleware (after creating other middleware):

```go
// Create tracing middleware
var tracingMiddleware *middleware.TracingMiddleware
if tracerProvider != nil {
    tracingMiddleware = middleware.NewTracingMiddleware(tracerProvider.Tracer())
}
```

Update middleware chain (insert tracing after request logger):

```go
// Apply middleware chain (outermost to innermost)
var handler http.Handler = mux
handler = authMiddleware.Authenticate(handler)
handler = csrfMiddleware.Protect(handler)
handler = cacheControl.Apply(handler)
handler = compress.Apply(handler)
handler = securityHeaders.Apply(handler)
if tracingMiddleware != nil {
    handler = tracingMiddleware.Apply(handler)  // NEW: after request logger
}
handler = requestLogger.Apply(handler)
```

Add tracer shutdown in graceful shutdown section:

```go
// In the graceful shutdown section, before server.Shutdown:
if tracerProvider != nil {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
        logging.Error("Failed to shutdown tracer", map[string]interface{}{
            "error": err.Error(),
        })
    }
}
```

---

## Phase 2: Service-Level Instrumentation

### 2.1 Instrument CardService

Update `internal/services/card.go`. Add spans to key methods:

```go
import (
    // ... existing imports ...
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
    "yearofbingo/internal/telemetry"
)

func (s *CardService) Create(ctx context.Context, userID uuid.UUID, title string, year int) (*models.BingoCard, error) {
    ctx, span := telemetry.StartSpan(ctx, "CardService.Create")
    defer span.End()

    span.SetAttributes(
        attribute.String("user_id", userID.String()),
        attribute.Int("year", year),
    )

    // ... existing implementation ...

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    span.SetAttributes(attribute.String("card_id", card.ID.String()))
    return card, nil
}
```

Apply similar pattern to these high-value methods:
- `GetByID()` - Add `card_id` attribute
- `ListByUser()` - Add `user_id`, `count` attributes
- `AddItem()` - Add `card_id`, `position` attributes
- `SwapItems()` - Add `card_id`, `from_position`, `to_position` attributes
- `Shuffle()` - Add `card_id`, `items_shuffled` attributes
- `Finalize()` - Add `card_id`, `items_count` attributes
- `CompleteItem()` - Add `card_id`, `item_id`, `bingo_count` attributes
- `BulkDelete()` - Add `user_id`, `card_count` attributes
- `GetStats()` - Add `card_id`, `completion_rate` attributes
- `ExportCards()` - Add `user_id`, `export_count` attributes

### 2.2 Instrument AuthService

Update `internal/services/auth.go`:

```go
func (s *AuthService) CreateSession(ctx context.Context, user *models.User) (string, error) {
    ctx, span := telemetry.StartSpan(ctx, "AuthService.CreateSession")
    defer span.End()

    span.SetAttributes(attribute.String("user_id", user.ID.String()))

    // ... existing implementation ...

    // Add span event for Redis vs PostgreSQL fallback
    if usedRedis {
        span.AddEvent("session_stored_in_redis")
    } else {
        span.AddEvent("session_stored_in_postgres")
    }

    return token, nil
}
```

Apply to:
- `ValidateSession()` - Track cache hit/miss
- `DeleteSession()`
- `HashPassword()` - Track duration (bcrypt is slow)
- `VerifyPassword()` - Track success/failure

### 2.3 Instrument EmailService

Update `internal/services/email.go`:

```go
func (s *EmailService) SendVerificationEmail(ctx context.Context, user *models.User, token string) error {
    ctx, span := telemetry.StartSpan(ctx, "EmailService.SendVerificationEmail")
    defer span.End()

    span.SetAttributes(
        attribute.String("user_id", user.ID.String()),
        attribute.String("email_provider", s.provider),
    )

    // ... existing implementation ...

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "email_send_failed")
        return err
    }

    span.AddEvent("email_sent")
    return nil
}
```

Apply to:
- `SendMagicLinkEmail()`
- `SendPasswordResetEmail()`

### 2.4 Instrument FriendService

High-value methods to instrument:
- `SearchUsers()` - Add `query_length`, `results_count` attributes
- `SendRequest()` - Add `from_user_id`, `to_user_id` attributes
- `AcceptRequest()` - Track friendship creation
- `GetUserCard()` - Add `friend_id`, `card_year` attributes
- `GetUserCards()` - Add `friend_id`, `cards_count` attributes

### 2.5 Instrument ReactionService

Key methods:
- `AddReaction()` - Add `item_id`, `emoji`, `user_id` attributes
- `RemoveReaction()`
- `GetReactionsForItem()`
- `GetReactionsForCard()` - Add `card_id`, `reaction_count` attributes

---

## Phase 3: Database Instrumentation

### Option A: Manual Query Instrumentation (Recommended for simplicity)

Create a helper in `internal/telemetry/db.go`:

```go
package telemetry

import (
    "context"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    "go.opentelemetry.io/otel/trace"
)

// StartDBSpan creates a span for database operations
func StartDBSpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
    ctx, span := StartSpan(ctx, operation+" "+table,
        trace.WithSpanKind(trace.SpanKindClient),
    )
    span.SetAttributes(
        semconv.DBSystemPostgreSQL,
        semconv.DBOperationKey.String(operation),
        semconv.DBSQLTableKey.String(table),
    )
    return ctx, span
}

// RecordDBError records a database error on the span
func RecordDBError(span trace.Span, err error) {
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
}

// SetRowsAffected sets the number of rows affected
func SetRowsAffected(span trace.Span, count int64) {
    span.SetAttributes(attribute.Int64("db.rows_affected", count))
}
```

Usage in services:

```go
func (s *CardService) GetByID(ctx context.Context, id uuid.UUID) (*models.BingoCard, error) {
    ctx, span := telemetry.StartSpan(ctx, "CardService.GetByID")
    defer span.End()

    // Database query with its own span
    dbCtx, dbSpan := telemetry.StartDBSpan(ctx, "SELECT", "bingo_cards")
    card, err := s.queryCard(dbCtx, id)
    telemetry.RecordDBError(dbSpan, err)
    dbSpan.End()

    // ... rest of method ...
}
```

### Option B: pgx Tracer Hook (More comprehensive)

Use pgx's built-in tracer interface:

```go
// internal/database/postgres.go
import (
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "go.opentelemetry.io/otel"
)

type pgxTracer struct {
    tracer trace.Tracer
}

func (t *pgxTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
    ctx, _ = t.tracer.Start(ctx, "pgx.Query",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            semconv.DBSystemPostgreSQL,
            attribute.String("db.statement", truncateQuery(data.SQL, 500)),
        ),
    )
    return ctx
}

func (t *pgxTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
    span := trace.SpanFromContext(ctx)
    if data.Err != nil {
        span.RecordError(data.Err)
        span.SetStatus(codes.Error, data.Err.Error())
    }
    span.SetAttributes(attribute.String("db.command_tag", data.CommandTag.String()))
    span.End()
}
```

### Redis Instrumentation

go-redis/v9 supports OpenTelemetry via the `redisotel` package:

```bash
go get github.com/redis/go-redis/extra/redisotel/v9
```

Update `internal/database/redis.go`:

```go
import (
    "github.com/redis/go-redis/extra/redisotel/v9"
)

func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        // ... existing options ...
    })

    // Enable tracing
    if err := redisotel.InstrumentTracing(client); err != nil {
        return nil, fmt.Errorf("failed to instrument redis tracing: %w", err)
    }

    return client, nil
}
```

---

## Phase 4: Logging Correlation

### 4.1 Update Logger

Update `internal/logging/logger.go` to include trace context:

```go
import (
    "context"
    "yearofbingo/internal/telemetry"
)

type LogEntry struct {
    Timestamp string                 `json:"timestamp"`
    Level     string                 `json:"level"`
    Message   string                 `json:"message"`
    Fields    map[string]interface{} `json:"fields,omitempty"`
    TraceID   string                 `json:"trace_id,omitempty"`
    SpanID    string                 `json:"span_id,omitempty"`
}

// InfoCtx logs with trace context
func (l *Logger) InfoCtx(ctx context.Context, message string, fields map[string]interface{}) {
    l.logWithContext(ctx, INFO, message, fields)
}

// ErrorCtx logs with trace context
func (l *Logger) ErrorCtx(ctx context.Context, message string, fields map[string]interface{}) {
    l.logWithContext(ctx, ERROR, message, fields)
}

func (l *Logger) logWithContext(ctx context.Context, level Level, message string, fields map[string]interface{}) {
    entry := LogEntry{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Level:     level.String(),
        Message:   message,
        Fields:    fields,
        TraceID:   telemetry.GetTraceID(ctx),
        SpanID:    telemetry.GetSpanID(ctx),
    }
    l.writeEntry(entry)
}
```

### 4.2 Update Request Logger Middleware

Update `internal/middleware/logging.go` to include trace IDs:

```go
func (m *RequestLogger) Apply(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ... existing code ...

        // After calling next handler, add trace context to log
        fields := map[string]interface{}{
            "method":      r.Method,
            "path":        r.URL.Path,
            "status":      rw.statusCode,
            "size":        rw.size,
            "duration_ms": elapsed.Milliseconds(),
            "remote_addr": r.RemoteAddr,
            "user_agent":  r.UserAgent(),
        }

        // Add trace context if available
        if traceID := telemetry.GetTraceID(r.Context()); traceID != "" {
            fields["trace_id"] = traceID
        }
        if spanID := telemetry.GetSpanID(r.Context()); spanID != "" {
            fields["span_id"] = spanID
        }

        m.logger.Info("request", fields)
    })
}
```

---

## Phase 5: Environment & Deployment

### 5.1 Environment Variables

Add to `.env.example` and production environment:

```bash
# OpenTelemetry / Honeycomb Configuration
OTEL_ENABLED=true
OTEL_SERVICE_NAME=yearofbingo
OTEL_SERVICE_VERSION=1.0.9
OTEL_ENVIRONMENT=production
OTEL_EXPORTER_OTLP_ENDPOINT=api.honeycomb.io:443
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY_HERE
OTEL_SAMPLING_RATE=0.1
```

### 5.2 Honeycomb Setup

1. **Create Honeycomb Account**: https://ui.honeycomb.io/signup (free tier)

2. **Get API Key**:
   - Go to Team Settings > API Keys
   - Create new key with "Send Events" permission
   - Copy the key value

3. **Create Environment**:
   - Create "production" environment
   - Create "development" environment (optional, for local testing)

4. **Configure Dataset**:
   - Data will auto-create "yearofbingo" dataset on first trace
   - Or manually create it in Honeycomb UI

### 5.3 Local Development

For local development without sending to Honeycomb:

**Option A**: Disable tracing
```bash
OTEL_ENABLED=false
```

**Option B**: Use Jaeger locally
```bash
# Add to compose.yaml
jaeger:
  image: jaegertracing/all-in-one:latest
  ports:
    - "16686:16686"  # UI
    - "4318:4318"    # OTLP HTTP

# Environment
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
```

**Option C**: Use Honeycomb's free tier for dev
```bash
OTEL_ENVIRONMENT=development
OTEL_SAMPLING_RATE=1.0  # Full sampling OK for low dev traffic
```

### 5.4 Update compose.yaml

Add environment variables:

```yaml
services:
  app:
    environment:
      # ... existing vars ...
      - OTEL_ENABLED=${OTEL_ENABLED:-false}
      - OTEL_SERVICE_NAME=${OTEL_SERVICE_NAME:-yearofbingo}
      - OTEL_SERVICE_VERSION=${OTEL_SERVICE_VERSION:-dev}
      - OTEL_ENVIRONMENT=${OTEL_ENVIRONMENT:-development}
      - OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT:-}
      - OTEL_EXPORTER_OTLP_HEADERS=${OTEL_EXPORTER_OTLP_HEADERS:-}
      - OTEL_SAMPLING_RATE=${OTEL_SAMPLING_RATE:-1.0}
```

### 5.5 Update CI/CD

Add secrets to GitHub Actions:

```yaml
# In .github/workflows/ci.yaml deploy job
env:
  OTEL_EXPORTER_OTLP_HEADERS: x-honeycomb-team=${{ secrets.HONEYCOMB_API_KEY }}
```

Add `HONEYCOMB_API_KEY` to GitHub repository secrets.

### 5.6 Update CLAUDE.md

Add to Environment Variables section:

```markdown
Telemetry: `OTEL_ENABLED`, `OTEL_SERVICE_NAME`, `OTEL_SERVICE_VERSION`, `OTEL_ENVIRONMENT`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_SAMPLING_RATE`
```

---

## Sampling Strategy

### Free Tier Limits

Honeycomb free tier allows 20 events/second (1.2M events/month).

### Recommended Sampling Rates

| Traffic Level | Events/sec | Sampling Rate | Result |
|--------------|------------|---------------|--------|
| Low (<5 req/s) | ~15 | 1.0 (100%) | Full visibility |
| Medium (5-50 req/s) | ~150 | 0.1 (10%) | ~15 events/sec |
| High (50-200 req/s) | ~600 | 0.03 (3%) | ~18 events/sec |

### Smart Sampling (Future Enhancement)

For more sophisticated sampling, implement parent-based sampling with error priority:

```go
// Always sample errors and slow requests
type SmartSampler struct {
    baseSampler sdktrace.Sampler
}

func (s *SmartSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
    // Always sample if parent is sampled
    if p.ParentContext.IsSampled() {
        return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
    }

    // Use base sampler for new traces
    return s.baseSampler.ShouldSample(p)
}
```

---

## Honeycomb Queries & Dashboards

### Useful Queries

**Error Rate by Endpoint**:
```
VISUALIZE: COUNT
WHERE: http.status_code >= 400
GROUP BY: http.url, http.status_code
```

**P99 Latency by Service Method**:
```
VISUALIZE: P99(duration_ms)
GROUP BY: name
WHERE: name CONTAINS "Service"
```

**Database Query Performance**:
```
VISUALIZE: HEATMAP(duration_ms)
WHERE: db.system = "postgresql"
GROUP BY: db.operation
```

**User Activity**:
```
VISUALIZE: COUNT_DISTINCT(user_id)
GROUP BY: http.url
```

**Session Cache Hit Rate**:
```
VISUALIZE: COUNT
WHERE: name = "AuthService.ValidateSession"
GROUP BY: cache_hit
```

### Recommended Triggers

1. **High Error Rate**: Alert when error rate > 5% over 5 minutes
2. **Slow Requests**: Alert when P95 latency > 2s
3. **Database Issues**: Alert when db operations > 500ms

---

## Testing

### Unit Tests

Create `internal/telemetry/tracer_test.go`:

```go
func TestParseHeaders(t *testing.T) {
    tests := []struct {
        input    string
        expected map[string]string
    }{
        {"", map[string]string{}},
        {"key=value", map[string]string{"key": "value"}},
        {"k1=v1,k2=v2", map[string]string{"k1": "v1", "k2": "v2"}},
    }
    for _, tt := range tests {
        result := ParseHeaders(tt.input)
        // assert equality
    }
}
```

### Integration Test

```go
func TestTracingMiddleware(t *testing.T) {
    // Create test tracer provider with in-memory exporter
    exporter := tracetest.NewInMemoryExporter()
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSyncer(exporter),
    )

    // Create middleware
    tm := middleware.NewTracingMiddleware(tp.Tracer("test"))

    // Create test handler
    handler := tm.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))

    // Make request
    req := httptest.NewRequest("GET", "/test", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    // Verify span was created
    spans := exporter.GetSpans()
    require.Len(t, spans, 1)
    assert.Equal(t, "GET /test", spans[0].Name)
}
```

---

## Files to Create/Modify Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/telemetry/tracer.go` | Tracer provider initialization |
| `internal/telemetry/context.go` | Context helpers for spans |
| `internal/telemetry/db.go` | Database span helpers |
| `internal/middleware/tracing.go` | HTTP tracing middleware |
| `internal/telemetry/tracer_test.go` | Unit tests |

### Modified Files
| File | Changes |
|------|---------|
| `go.mod` | Add OTEL dependencies |
| `internal/config/config.go` | Add TelemetryConfig |
| `cmd/server/main.go` | Initialize tracer, add middleware |
| `internal/services/card.go` | Add spans to methods |
| `internal/services/auth.go` | Add spans to methods |
| `internal/services/email.go` | Add spans to methods |
| `internal/services/friend.go` | Add spans to methods |
| `internal/services/reaction.go` | Add spans to methods |
| `internal/logging/logger.go` | Add trace context to logs |
| `internal/middleware/logging.go` | Add trace IDs to request logs |
| `compose.yaml` | Add OTEL environment variables |
| `CLAUDE.md` | Document new environment variables |

---

## Implementation Order

1. **Phase 1** (Core): Must complete first - infrastructure setup
2. **Phase 4** (Logging): Do early - immediate value with log correlation
3. **Phase 5** (Deployment): Configure Honeycomb, set up secrets
4. **Phase 2** (Services): Incremental - start with CardService, AuthService
5. **Phase 3** (Database): Optional enhancement for deeper visibility

Estimated effort: 2-3 focused implementation sessions.
