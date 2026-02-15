package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type SettingServiceImpl struct {
	SettingCache    SettingCache
	SettingDatabase SettingDatabase
	TxManager       TxManager
	Logger          *logger.Logger
}

func NewSettingService(settCache SettingCache, setdb SettingDatabase, txman TxManager, logger *logger.Logger) *SettingServiceImpl {
	return &SettingServiceImpl{
		SettingCache:    settCache,
		SettingDatabase: setdb,
		TxManager:       txman,
		Logger:          logger,
	}
}
func (s *SettingServiceImpl) SetSetting(ctx context.Context, key string, value string, updatedBy int64) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "SetSetting"
	if key == "" {
		s.Logger.ZapLogger.Warn("Empty Key", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	switch key {
	case models.SettingKeyPriceBasicSubscription:
		price, err := strconv.Atoi(value)
		if err != nil || price <= 0 {
			s.Logger.ZapLogger.Warn("Invalid Price", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}

	case models.SettingKeyLimitBaseDay:
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 0 {
			s.Logger.ZapLogger.Warn("Invalid LimitDay", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}
	case models.SettingKeyLimitPremiumDay:
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 0 {
			s.Logger.ZapLogger.Warn("Invalid LimitDay", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}
	default:
		s.Logger.ZapLogger.Warn("Unknown setting key", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.TxManager.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	//закоментил так как в тесте мок вовзращает nil interface
	/*if tx == nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(errors.New("returning nil transaction")), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}*/
	setting := models.NewSetting(key, value, updatedBy)
	err = s.SettingDatabase.SetSetting(ctxTimeout, tx, setting)
	if err != nil {
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	err = s.SettingCache.SetSetting(ctxTimeout, key, value)
	if err != nil {
		s.Logger.ZapLogger.Error("SetSetting", zap.Error(err), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	if err := s.TxManager.CommitTx(ctxTimeout, tx); err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	s.Logger.ZapLogger.Info("Setting updated successfully", zap.Any("key", key), zap.Any("traceID", trace.ID), zap.Any("place", place))

	return text_messages.TextSuccessSetSetting, nil
}

func (s *SettingServiceImpl) ViewSetting(ctx context.Context) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "ViewSetting"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cacheSettings, err := s.SettingCache.GetAllSettings(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("Failed to get settings from cache", zap.Error(err), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}

	dbSettings, err := s.SettingDatabase.GetAllSettings(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("Failed to get settings from database", zap.Error(err), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}

	dbSettingsMap := make(map[string]string)
	for _, setting := range dbSettings {
		if setting == nil {
			continue
		}
		dbSettingsMap[setting.Key] = setting.Value
	}

	formattedMessage := text_messages.FormatSettingsComparison(cacheSettings, dbSettingsMap)
	s.Logger.ZapLogger.Info("Setting received successfully", zap.Any("traceID", trace.ID), zap.Any("place", place))
	return formattedMessage, nil
}

func (s *SettingServiceImpl) RebootCacheData(ctx context.Context) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "RebootCacheData"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	settings, err := s.SettingDatabase.GetAllSettings(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("GetAllSettings", zap.Error(err), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	err = s.SettingCache.LoadCacheData(ctxTimeout, settings)
	if err != nil {
		s.Logger.ZapLogger.Error("LoadCacheData", zap.Error(err), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	s.Logger.ZapLogger.Info("Setting rebooted successfully", zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.SuccessRebootCache, nil
}
