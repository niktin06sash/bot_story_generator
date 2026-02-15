package service_test

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/service"
	mock_service "bot_story_generator/internal/service/mocks"
	"bot_story_generator/internal/text_messages"
	"bot_story_generator/internal/tracing"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidatePreCheckout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := &config.Config{}
	mockCacheSett := mock_service.NewMockSettingCache(ctrl)
	mockDBSub := mock_service.NewMockSubscriptionDatabase(ctrl)
	mockCacheDL := mock_service.NewMockDailyLimitCache(ctrl)
	mockDBDL := mock_service.NewMockDailyLimitDatabase(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userId := int64(1)
	payload := fmt.Sprintf("%s_%s_%d_%d", text_messages.NameBasicSubscription, "XTR", userId, time.Now().Unix())
	serv := service.NewSubscriptionService(mockCacheSett, mockDBSub, mockCacheDL, mockDBDL, log)
	mockDBSub.EXPECT().GetActiveSubscriptions(gomock.Any(), userId).Return([]*models.Subscription{}, nil)
	mockDBSub.EXPECT().GetStatusSubscription(gomock.Any(), payload, userId).Return(&models.Subscription{
		Payload: payload,
		UserID:  userId,
		Status:  "pending",
	}, nil)
	mockCacheSett.EXPECT().GetSetting(gomock.Any(), models.SettingKeyPriceBasicSubscription).Return("1000", nil)
	pd := &models.PaymentData{
		QueryID:        "something",
		UserID:         userId,
		Currency:       "XTR",
		InvoicePayload: payload,
		TotalAmount:    1000,
		Trace:          tracing.NewTrace(),
	}
	err := serv.ValidatePreCheckout(ctx, pd)
	require.Nil(t, err)
}

func TestBuySubscription_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := &config.Config{}
	mockCacheSett := mock_service.NewMockSettingCache(ctrl)
	mockDBSub := mock_service.NewMockSubscriptionDatabase(ctrl)
	mockCacheDL := mock_service.NewMockDailyLimitCache(ctrl)
	mockDBDL := mock_service.NewMockDailyLimitDatabase(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userId := int64(1)
	payload := fmt.Sprintf("%s_%s_%d_%d", text_messages.NameBasicSubscription, "XTR", userId, time.Now().Unix())
	serv := service.NewSubscriptionService(mockCacheSett, mockDBSub, mockCacheDL, mockDBDL, log)
	mockDBSub.EXPECT().GetActiveSubscriptions(gomock.Any(), userId).Return([]*models.Subscription{}, nil)
	mockCacheSett.EXPECT().GetSetting(gomock.Any(), models.SettingKeyPriceBasicSubscription).Return("1000", nil)
	sub := models.NewSubscription(userId, text_messages.NameBasicSubscription, payload, "pending", "XTR", 1000)
	mockDBSub.EXPECT().AddSubscription(gomock.Any(), sub).Return(nil)
	sub, err := serv.BuySubscription(ctx, userId)
	require.Nil(t, err)
}
func TestCommitSubscription_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := &config.Config{}
	mockCacheSett := mock_service.NewMockSettingCache(ctrl)
	mockDBSub := mock_service.NewMockSubscriptionDatabase(ctrl)
	mockCacheDL := mock_service.NewMockDailyLimitCache(ctrl)
	mockDBDL := mock_service.NewMockDailyLimitDatabase(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userId := int64(1)
	payload := fmt.Sprintf("%s_%s_%d_%d", text_messages.NameBasicSubscription, "XTR", userId, time.Now().Unix())
	serv := service.NewSubscriptionService(mockCacheSett, mockDBSub, mockCacheDL, mockDBDL, log)
	mockDBSub.EXPECT().PayedPendingSubscription(gomock.Any(), payload, userId, gomock.Any(), gomock.Any(), "something").Return(nil)
	mockCacheSett.EXPECT().GetSetting(gomock.Any(), models.SettingKeyLimitPremiumDay).Return("100", nil)
	mockCacheDL.EXPECT().DeleteExceededLimit(gomock.Any(), userId).Return(nil)
	limit := models.NewDailyLimit(userId, 0, 100)
	mockDBDL.EXPECT().UpdateLimitCountDailyLimit(gomock.Any(), limit)
	pd := &models.PaymentData{
		QueryID:        "something",
		ChargeID:       "something",
		UserID:         userId,
		Currency:       "XTR",
		InvoicePayload: payload,
		TotalAmount:    1000,
		Trace:          tracing.NewTrace(),
	}
	err := serv.CommitSubscription(ctx, pd)
	require.Nil(t, err)
}
func TestGetSubscriptionStatus_SuccessActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := &config.Config{}
	mockCacheSett := mock_service.NewMockSettingCache(ctrl)
	mockDBSub := mock_service.NewMockSubscriptionDatabase(ctrl)
	mockCacheDL := mock_service.NewMockDailyLimitCache(ctrl)
	mockDBDL := mock_service.NewMockDailyLimitDatabase(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userId := int64(1)
	serv := service.NewSubscriptionService(mockCacheSett, mockDBSub, mockCacheDL, mockDBDL, log)
	startData := time.Now()
	endData := startData.AddDate(0, 0, 30)
	mockDBSub.EXPECT().GetActiveSubscriptions(gomock.Any(), userId).Return([]*models.Subscription{{
		UserID:    userId,
		Type:      text_messages.NameBasicSubscription,
		EndDate:   endData,
		StartDate: startData,
	}}, nil)
	resp, err := serv.GetSubscriptionStatus(ctx, userId)
	require.Nil(t, err)
	require.Equal(t, resp, text_messages.CreateSubscriptionStatusMessage(text_messages.NameBasicSubscription, startData, endData))
}
func TestGetSubscriptionStatus_SuccessNoActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := &config.Config{}
	mockCacheSett := mock_service.NewMockSettingCache(ctrl)
	mockDBSub := mock_service.NewMockSubscriptionDatabase(ctrl)
	mockCacheDL := mock_service.NewMockDailyLimitCache(ctrl)
	mockDBDL := mock_service.NewMockDailyLimitDatabase(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userId := int64(1)
	serv := service.NewSubscriptionService(mockCacheSett, mockDBSub, mockCacheDL, mockDBDL, log)
	mockDBSub.EXPECT().GetActiveSubscriptions(gomock.Any(), userId).Return([]*models.Subscription{}, nil)
	resp, err := serv.GetSubscriptionStatus(ctx, userId)
	require.Nil(t, err)
	require.Equal(t, resp, text_messages.CreateNoSubscriptionMessage())
}
