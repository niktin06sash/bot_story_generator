package repository

import (
	"bot_story_generator/internal/cache"
	"context"
	"fmt"
	"time"
)

type DailyLimitCacheImpl struct {
	cacheclient *cache.CacheObject
}

func NewDailyLimitCache(cache *cache.CacheObject) *DailyLimitCacheImpl {
	return &DailyLimitCacheImpl{
		cacheclient: cache,
	}
}
func (s *DailyLimitCacheImpl) AddExceededLimit(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	now := time.Now().UTC()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)
	return s.cacheclient.Connect.Set(ctx, key, "true", endOfDay.Sub(now)).Err()
}
func (s *DailyLimitCacheImpl) CheckExceededLimit(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
func (s *DailyLimitCacheImpl) DeleteExceededLimit(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	_, err := s.cacheclient.Connect.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	return err
}
