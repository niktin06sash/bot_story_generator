package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Создание нового логгера
func New() (*zap.Logger, error) {
	// Создаем конфиг для разработки
	loggerConfig := zap.NewDevelopmentConfig()

	// Уровень логирования: debug и выше
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

	// Настройка отображения времени
	loggerConfig.EncoderConfig.TimeKey = "timestamp"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Отключаем stacktrace для уровней ниже error
	loggerConfig.DisableStacktrace = true
	logger, err := loggerConfig.Build(
		zap.AddStacktrace(zap.ErrorLevel), // stacktrace только для error и выше
	)
	return logger, err
}
