package cache

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type CacheObject struct {
	Connect          *redis.Client
	UserCreatedKey   string
	ExceededLimitKey string
	logger           *logger.Logger
}

func NewCacheConnection(cfg *config.Config, logger *logger.Logger) (*CacheObject, error) {
	cacheobject := &CacheObject{UserCreatedKey: cfg.Cache.UserCreatedKey, ExceededLimitKey: cfg.Cache.ExceededLimitKey, logger: logger}
	err := cacheobject.Open(cfg.Cache.URL, cfg.Cache.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	err = cacheobject.Ping()
	if err != nil {
		return nil, err
	}
	return cacheobject, nil
}
func (r *CacheObject) Open(url string, conn_timeout time.Duration) error {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return err
	}
	opts.DialTimeout = conn_timeout
	opts.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	r.Connect = redis.NewClient(opts)
	return nil
}

func (r *CacheObject) Ping() error {
	_, err := r.Connect.Ping(context.Background()).Result()
	return err
}

func (r *CacheObject) Close() {
	r.Connect.Close()
	r.logger.ZapLogger.Debug("Successful close Cache-connect")
}
