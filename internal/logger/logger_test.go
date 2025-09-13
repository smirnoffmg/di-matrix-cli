package logger_test

import (
	"di-matrix-cli/internal/logger"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestGetLogger(t *testing.T) {
	t.Parallel()

	// Test that GetLogger returns a valid logger
	log := logger.GetLogger()
	assert.NotNil(t, log)

	// Test that subsequent calls return the same instance (singleton behavior)
	log2 := logger.GetLogger()
	assert.Equal(t, log, log2)

	// Test that the logger can be used for logging
	log.Info("Test log message")
	log.Debug("Test debug message")
	log.Warn("Test warning message")
	log.Error("Test error message")
}

func TestSetLevel(t *testing.T) {
	t.Parallel()

	// Test setting different log levels
	logger.SetLevel(zapcore.DebugLevel)
	log := logger.GetLogger()
	assert.NotNil(t, log)

	logger.SetLevel(zapcore.InfoLevel)
	log2 := logger.GetLogger()
	assert.NotNil(t, log2)

	logger.SetLevel(zapcore.WarnLevel)
	log3 := logger.GetLogger()
	assert.NotNil(t, log3)

	logger.SetLevel(zapcore.ErrorLevel)
	log4 := logger.GetLogger()
	assert.NotNil(t, log4)

	// Test that all loggers are the same instance (singleton)
	assert.Equal(t, log, log2)
	assert.Equal(t, log2, log3)
	assert.Equal(t, log3, log4)
}

func TestLoggerConcurrency(t *testing.T) {
	t.Parallel()

	// Test concurrent access to logger
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			log := logger.GetLogger()
			assert.NotNil(t, log)

			// Test logging from different goroutines
			log.Info("Concurrent log message")
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLoggerLevelChanges(t *testing.T) {
	t.Parallel()

	// Test that level changes affect the logger
	logger.SetLevel(zapcore.DebugLevel)
	log := logger.GetLogger()

	// These should all work at debug level
	log.Debug("Debug message")
	log.Info("Info message")
	log.Warn("Warning message")
	log.Error("Error message")

	// Change to error level
	logger.SetLevel(zapcore.ErrorLevel)

	// Only error should work now (others will be filtered out)
	log.Debug("Debug message (should be filtered)")
	log.Info("Info message (should be filtered)")
	log.Warn("Warning message (should be filtered)")
	log.Error("Error message (should work)")
}
