// Package logging provides structured logging capabilities for the Fern Platform
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Logger with additional context and functionality
type Logger struct {
	*logrus.Logger
	config *config.LoggingConfig
}

// NewLogger creates a new logger instance based on the provided configuration
func NewLogger(cfg *config.LoggingConfig) (*Logger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", cfg.Level, err)
	}
	logger.SetLevel(level)

	// Set output
	var output io.Writer = os.Stdout
	if cfg.Output != "" && cfg.Output != "stdout" {
		if cfg.Output == "stderr" {
			output = os.Stderr
		} else {
			file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file %s: %w", cfg.Output, err)
			}
			output = file
		}
	}
	logger.SetOutput(output)

	// Set formatter
	if cfg.Structured || strings.ToLower(cfg.Format) == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z",
		})
	}

	return &Logger{
		Logger: logger,
		config: cfg,
	}, nil
}

// WithContext adds contextual fields to the logger
func (l *Logger) WithContext(fields map[string]interface{}) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithFields adds contextual fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithService adds service name context to the logger
func (l *Logger) WithService(serviceName string) *logrus.Entry {
	return l.Logger.WithField("service", serviceName)
}

// WithRequest adds request context to the logger
func (l *Logger) WithRequest(requestID, method, path string) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"method":     method,
		"path":       path,
	})
}

// WithError adds error context to the logger
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// WithUser adds user context to the logger
func (l *Logger) WithUser(userID string) *logrus.Entry {
	return l.Logger.WithField("user_id", userID)
}

// WithUser adds user context to a log entry
func WithUser(entry *logrus.Entry, userID string) *logrus.Entry {
	return entry.WithField("user_id", userID)
}

// WithTestRun adds test run context to the logger
func (l *Logger) WithTestRun(runID, projectID string) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"run_id":     runID,
		"project_id": projectID,
	})
}

// Middleware provides logging middleware functionality
type Middleware struct {
	logger *Logger
}

// NewMiddleware creates a new logging middleware
func NewMiddleware(logger *Logger) *Middleware {
	return &Middleware{logger: logger}
}

// RequestLogger creates a request logging entry
func (m *Middleware) RequestLogger(requestID, method, path, userAgent, remoteAddr string) *logrus.Entry {
	return m.logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"method":      method,
		"path":        path,
		"user_agent":  userAgent,
		"remote_addr": remoteAddr,
		"type":        "request",
	})
}

// Global logger instance
var globalLogger *Logger

// Initialize sets up the global logger
func Initialize(cfg *config.LoggingConfig) error {
	logger, err := NewLogger(cfg)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// Fallback to default logger if not initialized
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.JSONFormatter{})
		return &Logger{Logger: logger}
	}
	return globalLogger
}

// Info logs an info message
func Info(msg string, fields ...map[string]interface{}) {
	logger := GetLogger()
	if len(fields) > 0 {
		logger.Logger.WithFields(fields[0]).Info(msg)
	} else {
		logger.Logger.Info(msg)
	}
}

// Error logs an error message
func Error(msg string, err error, fields ...map[string]interface{}) {
	logger := GetLogger()
	entry := logger.Logger.WithError(err)
	if len(fields) > 0 {
		entry.WithFields(fields[0]).Error(msg)
	} else {
		entry.Error(msg)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...map[string]interface{}) {
	logger := GetLogger()
	if len(fields) > 0 {
		logger.Logger.WithFields(fields[0]).Warn(msg)
	} else {
		logger.Logger.Warn(msg)
	}
}

// Debug logs a debug message
func Debug(msg string, fields ...map[string]interface{}) {
	logger := GetLogger()
	if len(fields) > 0 {
		logger.Logger.WithFields(fields[0]).Debug(msg)
	} else {
		logger.Logger.Debug(msg)
	}
}
