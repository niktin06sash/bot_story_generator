package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func updateOrAddDailyLimit(ctx context.Context, tx pgx.Tx, limit *models.DailyLimit, step int, trace models.Trace, LogPlace string, daydb DailyLimitDatabase, txman TxManager, logger *logger.Logger) error {
	if limit == nil {
		rollbackErr := txman.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		}
		return errors.New(text_messages.TextErrorCreateTask)
	}
	if limit.Count == 0 {
		limit.Count += step
		err := daydb.AddDailyLimit(ctx, tx, limit)
		if err != nil {
			logger.ZapLogger.Error("AddDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			rollbackErr := txman.RollbackTx(context.Background(), tx)
			if rollbackErr != nil {
				logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	} else {
		limit.Count += step
		err := daydb.UpdateCountDailyLimit(ctx, tx, limit)
		if err != nil {
			logger.ZapLogger.Error("UpdateDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			rollbackErr := txman.RollbackTx(context.Background(), tx)
			if rollbackErr != nil {
				logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	}
	return nil
}
func checkDailyLimits(ctx context.Context, userID int64, trace models.Trace, LogPlace string, dcache DailyLimitCache, dbd DailyLimitDatabase, subdb SubscriptionDatabase, setcache SettingCache, logger *logger.Logger) (*models.DailyLimit, error) {
	//Проверяем превышение лимита в кэше
	isExist, err := dcache.CheckExceededLimit(ctx, userID)
	if err != nil {
		logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
	} else if isExist {
		logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(errors.New("cache: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	} else if !isExist {
		logger.ZapLogger.Info("CheckExceededLimit Exceeded Limits not in cache. Checking in database...", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
	}
	// Проверяем в базе данных, есть ли дневные ходы у пользователя для создания новой истории
	var limitv *models.DailyLimit
	var subb *models.Subscription
	g, ctxG := errgroup.WithContext(ctx)
	//параллельно делаем запросы для получения данных дневного лимита и подписки(если есть хоть одна ошибка - прекращаем выполнение)
	g.Go(func() error {
		limit, err := dbd.GetDailyLimit(ctxG, userID)
		if err != nil {
			logger.ZapLogger.Error("GetDailyLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = limit
		return nil
	})
	g.Go(func() error {
		subscriptions, err := subdb.GetActiveSubscriptions(ctxG, userID)
		if err != nil {
			logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		if len(subscriptions) > 1 {
			logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		if len(subscriptions) == 1 {
			subb = subscriptions[0]
		}
		return nil
	})
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	//сегодня ходов еще не было + подписка неактивна
	if limitv == nil && subb == nil {
		baseDayLimitStr, err := setcache.GetSetting(ctx, models.SettingKeyLimitBaseDay)
		if err != nil {
			logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		baseDayLimit, convErr := strconv.Atoi(baseDayLimitStr)
		if convErr != nil {
			logger.ZapLogger.Error("Atoi limit.day.base", zap.Error(convErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = models.NewDailyLimit(userID, 0, baseDayLimit)
	}
	//сегодня ходов еще не было + подписка активна
	if limitv == nil && subb != nil {
		premDayLimitStr, err := setcache.GetSetting(ctx, models.SettingKeyLimitPremiumDay)
		if err != nil {
			logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		premDayLimit, convErr := strconv.Atoi(premDayLimitStr)
		if convErr != nil {
			logger.ZapLogger.Error("Atoi limit.day.premium", zap.Error(convErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = models.NewDailyLimit(userID, 0, premDayLimit)
	}
	if limitv.LimitCount <= limitv.Count {
		logger.ZapLogger.Warn("GetDailyLimit", zap.Error(errors.New("client: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		//Добавляем превышение лимита в кэш
		err := dcache.AddExceededLimit(ctx, userID)
		if err != nil {
			logger.ZapLogger.Warn("AddExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		}
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	}
	return limitv, nil
}
func getSubPrice(ctx context.Context, userID int64, trace models.Trace, logPlace string, settCache SettingCache, logger *logger.Logger) (int, error) {
	price, err := settCache.GetSetting(ctx, models.SettingKeyPriceBasicSubscription)
	if err != nil {
		logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", logPlace))
		return 0, errors.New(text_messages.TextErrorProcessPayment)
	}

	priceInt, err := strconv.Atoi(price)
	if err != nil {
		logger.ZapLogger.Error("Atoi sub.basic.price", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", logPlace))
		return 0, errors.New(text_messages.TextErrorProcessPayment)
	}
	return priceInt, nil
}
func getTrace(ctx context.Context, logger *logger.Logger) models.Trace {
	trace, ok := ctx.Value(models.TraceKey).(models.Trace)
	if !ok {
		logger.ZapLogger.Warn("Context value for 'trace' is not a models.Trace", zap.Any("trace", trace))
		return models.Trace{}
	}
	return trace
}
func asyncRejectSub(pd *models.PaymentData, place string, sdb SubscriptionDatabase, logger *logger.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rejerr := sdb.RejectedPendingSubscription(ctx, pd.InvoicePayload, pd.UserID)
	if rejerr != nil {
		logger.ZapLogger.Error("RejectedPendingSubscription", zap.Error(rejerr), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", pd.Trace.ID), zap.Any("place", place))
	}
}
