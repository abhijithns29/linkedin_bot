package logger

import (
	"log/slog"
	"os"
)

// Logger defines the interface for logging throughout the application
type Logger interface {
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	Debug(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
}

// SlogAdapter adapts the standard library slog to our Logger interface
type SlogAdapter struct {
	logger *slog.Logger
}

// New creates a new structured logger
// Defaults to generic text handler (time=... level=INFO msg=... key=val)
func New() Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Default to debug for development
	}
	// Use TextHandler for structured but human-readable output
	handler := slog.NewTextHandler(os.Stdout, opts)

	return &SlogAdapter{
		logger: slog.New(handler),
	}
}

// NewJSON creates a new JSON logger (useful for production/parsing)
func NewJSON() Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &SlogAdapter{
		logger: slog.New(handler),
	}
}

func (l *SlogAdapter) Info(msg string, keyvals ...interface{}) {
	l.logger.Info(msg, keyvals...)
}

func (l *SlogAdapter) Error(msg string, keyvals ...interface{}) {
	l.logger.Error(msg, keyvals...)
}

func (l *SlogAdapter) Debug(msg string, keyvals ...interface{}) {
	l.logger.Debug(msg, keyvals...)
}

func (l *SlogAdapter) Warn(msg string, keyvals ...interface{}) {
	l.logger.Warn(msg, keyvals...)
}
