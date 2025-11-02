package cache

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type CacheObject struct {
	Connect          *redis.Client
	UserCreatedKey   string
	ExceededLimitKey string
	logger           *logger.Logger
}

func NewRedisConnection(cfg *config.Config, logger *logger.Logger) (*CacheObject, error) {
	redisobject := &CacheObject{UserCreatedKey: cfg.Cache.UserCreatedKey, ExceededLimitKey: cfg.Cache.ExceededLimitKey}
	err := redisobject.Open(cfg.Cache.URL, cfg.Cache.ConnectTimeout)
	if err != nil {
		logger.ZapLogger.Error("Failed to establish Redis-Client connection", zap.Error(err))
		return nil, err
	}
	err = redisobject.Ping()
	if err != nil {
		logger.ZapLogger.Error("Failed to ping Redis-Client connection", zap.Error(err))
		redisobject.Close()
		return nil, err
	}
	logger.ZapLogger.Info("Successful Redis-connect")
	return redisobject, nil
}
func (r *CacheObject) Open(url string, conn_timeout time.Duration) error {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return err
	}
	opts.DialTimeout = conn_timeout
	r.Connect = redis.NewClient(opts)
	return nil
}

func (r *CacheObject) Ping() error {
	_, err := r.Connect.Ping(context.Background()).Result()
	return err
}

func (r *CacheObject) Close() {
	r.Connect.Close()
	r.logger.ZapLogger.Info("Successful close Redis-connect")
}
