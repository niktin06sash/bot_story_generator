package repository

import (
	"bot_story_generator/internal/cache"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type SettingCacheImpl struct {
	cacheclient *cache.CacheObject
}

func NewSettingCache(cache *cache.CacheObject) *SettingCacheImpl {
	return &SettingCacheImpl{
		cacheclient: cache,
	}
}
func (s *SettingCacheImpl) LoadCacheData(ctx context.Context, settings []*models.Setting) error {
	if settings == nil {
		return errors.New("settings is nil")
	}
	allName := models.NameSettingKeys()
	// Записываем каждую настройку в Redis с ключом settings:%s<key>
	for _, st := range settings {
		key := fmt.Sprintf(s.cacheclient.SettingsKey, st.Key)
		// Сохраняем значение как строку без TTL (пока)
		err := s.cacheclient.Connect.Set(ctx, key, st.Value, 0).Err()
		if err != nil {
			return err
		}
	}

	// Проверяем, что все ожидаемые ключи присутствуют в кэше
	var missing []string
	for _, name := range allName {
		key := fmt.Sprintf(s.cacheclient.SettingsKey, name)
		exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
		if err != nil {
			return err
		}
		if exists == 0 {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("some settings are missing in cache: %v", missing)
	}

	return nil
}

// SetSetting сохраняет отдельную настройку в кэше
func (s *SettingCacheImpl) SetSetting(ctx context.Context, key, value string) error {
	redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, key)
	if err := s.cacheclient.Connect.Set(ctx, redisKey, value, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (s *SettingCacheImpl) GetAllSettings(ctx context.Context) (map[string]string, error) {
	// Получаем все возможные ключи настроек
	keys := models.NameSettingKeys()
	res := make(map[string]string, len(keys))
	var missing []string

	for _, name := range keys {
		redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, name)
		val, err := s.cacheclient.Connect.Get(ctx, redisKey).Result()
		if err != nil {
			if err == redis.Nil {
				missing = append(missing, name)
				continue
			}
			return nil, err
		}
		res[name] = val
	}

	if len(missing) > 0 {
		return res, fmt.Errorf("missing settings in cache: %v", missing)
	}
	return res, nil
}

func (s *SettingCacheImpl) GetSetting(ctx context.Context, key string) (string, error) {
	redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, key)
	val, err := s.cacheclient.Connect.Get(ctx, redisKey).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}
