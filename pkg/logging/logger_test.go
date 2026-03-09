package logging

import (
	"bytes"
	"errors"
	"testing"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_ValidConfig(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		Structured: true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	assert.Equal(t, logrus.InfoLevel, logger.Logger.Level)
}

func TestNewLogger_TextFormat(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	assert.Equal(t, logrus.DebugLevel, logger.Logger.Level)
}

func TestNewLogger_InvalidLogLevel(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "invalid",
		Format: "json",
		Output: "stdout",
	}

	logger, err := NewLogger(cfg)
	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestNewLogger_StderrOutput(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "warn",
		Format: "json",
		Output: "stderr",
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	assert.Equal(t, logrus.WarnLevel, logger.Logger.Level)
}

func TestNewLogger_AllLogLevels(t *testing.T) {
	levels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := &config.LoggingConfig{
				Level:  level,
				Format: "json",
				Output: "stdout",
			}
			logger, err := NewLogger(cfg)
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestNewLogger_StructuredFlag(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:      "info",
		Structured: true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	_, isJSON := logger.Logger.Formatter.(*logrus.JSONFormatter)
	assert.True(t, isJSON, "structured flag should produce JSON formatter")
}

func TestGetLogger_ReturnsDefault(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	logger := GetLogger()
	require.NotNil(t, logger)
	assert.Equal(t, logrus.InfoLevel, logger.Logger.Level)
}

func TestInitialize(t *testing.T) {
	// Reset global logger before and after
	globalLogger = nil
	defer func() { globalLogger = nil }()

	cfg := &config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	err := Initialize(cfg)
	require.NoError(t, err)

	logger := GetLogger()
	require.NotNil(t, logger)
	assert.Equal(t, logrus.DebugLevel, logger.Logger.Level)
}

func TestInitialize_InvalidLevel(t *testing.T) {
	globalLogger = nil
	defer func() { globalLogger = nil }()

	cfg := &config.LoggingConfig{
		Level: "invalid",
	}

	err := Initialize(cfg)
	assert.Error(t, err)
}

func TestLoggerMethods_DontPanic(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "trace",
		Format: "json",
		Output: "stdout",
	}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Redirect output to discard
	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	assert.NotPanics(t, func() { logger.Info("test info") })
	assert.NotPanics(t, func() { logger.Warn("test warn") })
	assert.NotPanics(t, func() { logger.Debug("test debug") })
	assert.NotPanics(t, func() { logger.Logger.Error("test error") })
}

func TestWithError(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithError(errors.New("test error"))
	require.NotNil(t, entry)
	entry.Error("something failed")
	assert.Contains(t, buf.String(), "test error")
}

func TestWithFields(t *testing.T) {
	cfg := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithFields(map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	})
	require.NotNil(t, entry)
	entry.Info("fields test")
	assert.Contains(t, buf.String(), "key1")
	assert.Contains(t, buf.String(), "value1")
}

func TestWithContext(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	entry := logger.WithContext(map[string]interface{}{"ctx_key": "ctx_val"})
	require.NotNil(t, entry)
}

func TestWithService(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithService("test-svc")
	require.NotNil(t, entry)
	entry.Info("service test")
	assert.Contains(t, buf.String(), "test-svc")
}

func TestWithRequest(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithRequest("req-123", "GET", "/api/test")
	require.NotNil(t, entry)
	entry.Info("request test")
	assert.Contains(t, buf.String(), "req-123")
	assert.Contains(t, buf.String(), "GET")
}

func TestWithUser(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithUser("user-456")
	require.NotNil(t, entry)
	entry.Info("user test")
	assert.Contains(t, buf.String(), "user-456")
}

func TestWithUserPackageLevel(t *testing.T) {
	entry := logrus.NewEntry(logrus.New())
	result := WithUser(entry, "user-789")
	require.NotNil(t, result)
}

func TestWithTestRun(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)

	entry := logger.WithTestRun("run-1", "proj-2")
	require.NotNil(t, entry)
	entry.Info("test run")
	assert.Contains(t, buf.String(), "run-1")
	assert.Contains(t, buf.String(), "proj-2")
}

func TestMiddleware_RequestLogger(t *testing.T) {
	cfg := &config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	mw := NewMiddleware(logger)
	require.NotNil(t, mw)

	entry := mw.RequestLogger("req-1", "POST", "/api/data", "TestAgent/1.0", "127.0.0.1")
	require.NotNil(t, entry)
}

func TestPackageLevelFunctions_DontPanic(t *testing.T) {
	globalLogger = nil
	defer func() { globalLogger = nil }()

	assert.NotPanics(t, func() { Info("test") })
	assert.NotPanics(t, func() { Warn("test") })
	assert.NotPanics(t, func() { Debug("test") })
	assert.NotPanics(t, func() { Error("test", errors.New("err")) })
}

func TestPackageLevelFunctions_WithFields(t *testing.T) {
	globalLogger = nil
	defer func() { globalLogger = nil }()

	fields := map[string]interface{}{"key": "val"}
	assert.NotPanics(t, func() { Info("test", fields) })
	assert.NotPanics(t, func() { Warn("test", fields) })
	assert.NotPanics(t, func() { Debug("test", fields) })
	assert.NotPanics(t, func() { Error("test", errors.New("err"), fields) })
}
