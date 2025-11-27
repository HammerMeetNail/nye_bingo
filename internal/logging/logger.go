package logging

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity levels.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured JSON logging.
type Logger struct {
	mu     sync.Mutex
	output io.Writer
	level  Level
	fields map[string]interface{}
}

// New creates a new Logger instance.
func New() *Logger {
	return &Logger{
		output: os.Stdout,
		level:  LevelInfo,
		fields: make(map[string]interface{}),
	}
}

// SetOutput sets the output writer for the logger.
func (l *Logger) SetOutput(w io.Writer) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
	return l
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	return l
}

// WithField returns a new logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(map[string]interface{}{key: value})
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newFields := make(map[string]interface{}, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{
		output: l.output,
		level:  l.level,
		fields: newFields,
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.log(LevelError, msg, fields...)
}

func (l *Logger) log(level Level, msg string, additionalFields ...map[string]interface{}) {
	if level < l.level {
		return
	}

	// Merge fields
	allFields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		allFields[k] = v
	}
	for _, f := range additionalFields {
		for k, v := range f {
			allFields[k] = v
		}
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
	}
	if len(allFields) > 0 {
		entry.Fields = allFields
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple format
		_, _ = l.output.Write([]byte(entry.Timestamp + " " + entry.Level + " " + msg + "\n"))
		return
	}
	_, _ = l.output.Write(data)
	_, _ = l.output.Write([]byte("\n"))
}

// Default is the default logger instance.
var Default = New()

// SetDefaultLevel sets the level for the default logger.
func SetDefaultLevel(level Level) {
	Default.SetLevel(level)
}

// Debug logs using the default logger.
func Debug(msg string, fields ...map[string]interface{}) {
	Default.Debug(msg, fields...)
}

// Info logs using the default logger.
func Info(msg string, fields ...map[string]interface{}) {
	Default.Info(msg, fields...)
}

// Warn logs using the default logger.
func Warn(msg string, fields ...map[string]interface{}) {
	Default.Warn(msg, fields...)
}

// Error logs using the default logger.
func Error(msg string, fields ...map[string]interface{}) {
	Default.Error(msg, fields...)
}
