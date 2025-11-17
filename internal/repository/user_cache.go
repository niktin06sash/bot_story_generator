package repository

import (
	"bot_story_generator/internal/cache"
	"context"
	"fmt"
	"time"
)

type UserCacheImpl struct {
	cacheclient *cache.CacheObject
}

func NewUserCache(cache *cache.CacheObject) *UserCacheImpl {
	return &UserCacheImpl{
		cacheclient: cache,
	}
}
func (s *UserCacheImpl) AddCreatedUser(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.UserCreatedKey, userID)
	return s.cacheclient.Connect.Set(ctx, key, "true", 24*time.Hour).Err()
}
func (s *UserCacheImpl) CheckCreatedUser(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf(s.cacheclient.UserCreatedKey, userID)
	exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
