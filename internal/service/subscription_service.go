package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type SubscriptionServiceImpl struct {
	SettingCache       SettingCache
	SubDatabase        SubscriptionDatabase
	DailyLimitCache    DailyLimitCache
	DailyLimitDatabase DailyLimitDatabase
	Logger             *logger.Logger
}

func NewSubscriptionService(settCache SettingCache, subdb SubscriptionDatabase, dcache DailyLimitCache, ddb DailyLimitDatabase, logger *logger.Logger) *SubscriptionServiceImpl {
	return &SubscriptionServiceImpl{
		SubDatabase:        subdb,
		SettingCache:       settCache,
		DailyLimitCache:    dcache,
		DailyLimitDatabase: ddb,
		Logger:             logger,
	}
}
func (s *SubscriptionServiceImpl) ValidatePreCheckout(ctx context.Context, pd *models.PaymentData) error {
	trace := getTrace(ctx, s.Logger)
	place := "ValidatePreCheckout"
	ctxTimeout, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	g, ctxTimeoutG := errgroup.WithContext(ctxTimeout)
	var subb *models.Subscription
	//параллельно делаем запросы для получения данных транзакции(если есть хоть одна ошибка - помечаем статус транзакции на rejected)
	g.Go(func() error {
		subscriptions, err := s.SubDatabase.GetActiveSubscriptions(ctxTimeoutG, pd.UserID)
		if err != nil {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		if len(subscriptions) > 1 {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		if len(subscriptions) > 0 {
			s.Logger.ZapLogger.Warn("GetActiveSubscriptions", zap.Error(fmt.Errorf("client: user already has active subscription")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.TextAlreadyActiveSubscription)
		}
		return nil
	})
	g.Go(func() error {
		sub, err := s.SubDatabase.GetStatusSubscription(ctxTimeoutG, pd.InvoicePayload, pd.UserID)
		if err != nil {
			if strings.HasPrefix(err.Error(), "client: ") {
				s.Logger.ZapLogger.Warn("GetStatusSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
				return errors.New(text_messages.InvalidPaymentData)
			}
			s.Logger.ZapLogger.Error("GetStatusSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		subb = sub
		if sub != nil && sub.Status == "rejected" {
			s.Logger.ZapLogger.Warn("GetStatusSubscription", zap.Error(errors.New("attempt to send a rejected transaction")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.InvalidPaymentData)
		}
		if sub != nil && sub.Status == "paid" {
			s.Logger.ZapLogger.Warn("GetStatusSubscription", zap.Error(errors.New("attempt to send a payed transaction")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return errors.New(text_messages.InvalidPaymentData)
		}
		return nil
	})
	//делаем reject транзакции ассинхронно, не блокируя основной поток
	err := g.Wait()
	if err != nil {
		if subb != nil && subb.Status == "pending" {
			go asyncRejectSub(pd, place, s.SubDatabase, s.Logger)
		}
		return err
	}
	price, err := getSubPrice(ctxTimeout, pd.UserID, trace, place, s.SettingCache, s.Logger)
	if err != nil {
		go asyncRejectSub(pd, place, s.SubDatabase, s.Logger)
		return err
	}
	//сверяем цены
	if pd != nil && price != pd.TotalAmount {
		s.Logger.ZapLogger.Warn("Check Subscription Price", zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		go asyncRejectSub(pd, place, s.SubDatabase, s.Logger)
		return errors.New(text_messages.InvalidPaymentData)
	}
	s.Logger.ZapLogger.Info("PreCheckout validated successfully", zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return nil
}

// Обработка команды покупки подписки
// Проверяем, что нет активной подписки + добавляем в бд pending у подписки
func (s *SubscriptionServiceImpl) BuySubscription(ctx context.Context, userID int64) (*models.Subscription, error) {
	trace := getTrace(ctx, s.Logger)
	place := "BuySubscription"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	subscriptions, err := s.SubDatabase.GetActiveSubscriptions(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	if len(subscriptions) > 1 {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	if len(subscriptions) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveSubscriptions", zap.Error(fmt.Errorf("client: user already has active subscription")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextAlreadyActiveSubscription)
	}
	status := "pending"

	// Есил будем добавлять другие типы подписок, то тут нужно будет менять currency и price в зависимости от типа
	// Например, можно будет передавать тип подписки в аргументах функции
	// и в зависимости от этого выбирать нужные параметры
	// Но пока у нас только один тип подписки, поэтому оставляем так
	currencySubscription := "XTR"

	nameSub := text_messages.NameBasicSubscription

	payload := fmt.Sprintf("%s_%s_%d_%d", nameSub, currencySubscription, userID, time.Now().Unix())
	price, err := getSubPrice(ctxTimeout, userID, trace, place, s.SettingCache, s.Logger)
	if err != nil {
		return nil, err
	}
	sub := models.NewSubscription(userID, nameSub, payload, status, currencySubscription, price)
	err = s.SubDatabase.AddSubscription(ctxTimeout, sub)
	if err != nil {
		s.Logger.ZapLogger.Error("AddSubscription", zap.Error(err), zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	s.Logger.ZapLogger.Info("Subscription pending successfully", zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return sub, nil
}
func (s *SubscriptionServiceImpl) CommitSubscription(ctx context.Context, pd *models.PaymentData) error {
	trace := getTrace(ctx, s.Logger)
	place := "CommitSubscription"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Подписка на 30 дней
	start := time.Now()
	end := start.AddDate(0, 0, 30)
	err := s.SubDatabase.PayedPendingSubscription(ctxTimeout, pd.InvoicePayload, pd.UserID, start, end, pd.ChargeID)
	if err != nil {
		s.Logger.ZapLogger.Error("PayedPendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	premiumDayLimitStr, err := s.SettingCache.GetSetting(ctx, models.SettingKeyLimitPremiumDay)
	if err != nil {
		s.Logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	premiumDayLimit, convErr := strconv.Atoi(premiumDayLimitStr)
	if convErr != nil {
		s.Logger.ZapLogger.Error("Atoi limit.day.premium", zap.Error(convErr), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	err = s.DailyLimitCache.DeleteExceededLimit(ctxTimeout, pd.UserID)
	if err != nil {
		s.Logger.ZapLogger.Error("DeleteExceededLimit", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	limit := models.NewDailyLimit(pd.UserID, 0, premiumDayLimit)
	err = s.DailyLimitDatabase.UpdateLimitCountDailyLimit(ctxTimeout, limit)
	if err != nil {
		s.Logger.ZapLogger.Error("UpdateLimitCountDailyLimit", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	s.Logger.ZapLogger.Info("Subscription commited successfully", zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return nil
}

func (s *SubscriptionServiceImpl) GetSubscriptionStatus(ctx context.Context, userID int64) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "GetSubscriptionStatus"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// TODO добавить обновление лимита на обычный, когда подписка закончилась

	subscriptions, err := s.SubDatabase.GetActiveSubscriptions(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorGetSubscriptionStatus)
	}
	if len(subscriptions) > 1 {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorGetSubscriptionStatus)
	}
	if len(subscriptions) == 0 {
		return text_messages.CreateNoSubscriptionMessage(), nil
	}

	sub := subscriptions[0]
	if sub != nil {
		typeSub, startData, endData := sub.Type, sub.StartDate, sub.EndDate
		s.Logger.ZapLogger.Info("Subscription received successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return text_messages.CreateSubscriptionStatusMessage(typeSub, startData, endData), nil
	}
	return "", errors.New(text_messages.TextErrorGetSubscriptionStatus)
}
