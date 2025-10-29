package cache

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type CacheObject struct {
	Connect *redis.Client
	logger  *logger.Logger
}

func NewRedisConnection(cfg *config.Config, logger *logger.Logger) (*CacheObject, error) {
	redisobject := &CacheObject{}
	redisobject.Open(cfg.Cache.Host, cfg.Cache.Port, cfg.Cache.Password, cfg.Cache.DB)
	err := redisobject.Ping()
	if err != nil {
		redisobject.Close()
		logger.ZapLogger.Error("Failed to establish Redis-Client connection", zap.Error(err))
		return nil, err
	}
	logger.ZapLogger.Info("Successful Redis-connect")
	return redisobject, nil
}
func (r *CacheObject) Open(host string, port int, password string, db int) {
	r.Connect = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})
}

func (r *CacheObject) Ping() error {
	_, err := r.Connect.Ping(context.Background()).Result()
	return err
}

func (r *CacheObject) Close() {
	r.Connect.Close()
	r.logger.ZapLogger.Info("Successful close Redis-connect")
}
