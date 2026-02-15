package logger

import (
	"bot_story_generator/internal/config"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	ZapLogger *zap.Logger
}

const startServerLog = "-------Starting Server-------"
const stopServerLog = "-------Stopping Server-------"

func NewLogger(cfg *config.Config) (*Logger, error) {
	//TODO добавить больше данных конфигураций логгера
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	loggerConfig.EncoderConfig.TimeKey = "timestamp"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	var cores []zapcore.Core
	addCore := func(level zapcore.Level, path string) {
		if path == "" {
			return
		}
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create log directory %s: %v", dir, err)
			return
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open file %s: %v", dir, err)
			return
		}
		writer := zapcore.AddSync(file)
		levelFilter := zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l == level
		})

		encoder := zapcore.NewConsoleEncoder(loggerConfig.EncoderConfig)
		core := zapcore.NewCore(encoder, writer, levelFilter)
		cores = append(cores, core)
	}
	addCore(zapcore.InfoLevel, cfg.Logger.LogPaths.Info)
	addCore(zapcore.WarnLevel, cfg.Logger.LogPaths.Warn)
	addCore(zapcore.ErrorLevel, cfg.Logger.LogPaths.Error)
	addCore(zapcore.DebugLevel, cfg.Logger.LogPaths.Debug)
	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller())
	lg := &Logger{ZapLogger: logger}
	lg.logServerMsg(startServerLog)
	return lg, nil
}
func (l *Logger) logServerMsg(msg string) {
	fields := []zap.Field{
		zap.Time("timestamp", time.Now()),
	}

	l.ZapLogger.Debug(msg, fields...)
	l.ZapLogger.Info(msg, fields...)
	l.ZapLogger.Warn(msg, fields...)
	l.ZapLogger.Error(msg, fields...)
}

func (l *Logger) Sync() {
	l.logServerMsg(stopServerLog)
	l.ZapLogger.Sync()
}
