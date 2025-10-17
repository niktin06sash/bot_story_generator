package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	ZapLogger *zap.Logger
}

func NewLogger() (*Logger, error) {
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	loggerConfig.EncoderConfig.TimeKey = "timestamp"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.DisableStacktrace = true
	logger, err := loggerConfig.Build(
		zap.AddStacktrace(zap.ErrorLevel),
	)
	return &Logger{ZapLogger: logger}, err
}
func (l *Logger) Sync() {
	l.ZapLogger.Sync()
}
