package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	atomicLevel zap.AtomicLevel
	logger      *zap.Logger
	mu          sync.RWMutex
}

var (
	instance *Logger   //nolint:gochecknoglobals // Singleton pattern for logger
	once     sync.Once //nolint:gochecknoglobals // Singleton pattern for logger
)

func GetLogger() *zap.Logger {
	once.Do(func() {
		instance = &Logger{
			atomicLevel: zap.NewAtomicLevelAt(zap.InfoLevel),
		}

		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.TimeKey = "timestamp"
		encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000") // HH:MM:SS.mmm format
		encoderCfg.CallerKey = ""                                           // remove caller
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			instance.atomicLevel,
		)

		instance.logger = zap.New(core)
	})

	instance.mu.RLock()
	defer instance.mu.RUnlock()
	return instance.logger
}

func SetLevel(level zapcore.Level) {
	once.Do(func() {
		instance = &Logger{
			atomicLevel: zap.NewAtomicLevelAt(zap.InfoLevel),
		}
	})

	instance.mu.Lock()
	defer instance.mu.Unlock()
	instance.atomicLevel.SetLevel(level)
}
