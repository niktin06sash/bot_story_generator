package service_test

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/service"
	mock_service "bot_story_generator/internal/service/mocks"
	"bot_story_generator/internal/text_messages"
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateStory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	setcache := mock_service.NewMockSettingCache(ctrl)
	subdb := mock_service.NewMockSubscriptionDatabase(ctrl)
	dcache := mock_service.NewMockDailyLimitCache(ctrl)
	ddb := mock_service.NewMockDailyLimitDatabase(ctrl)
	stdb := mock_service.NewMockStoryDatabase(ctrl)
	stai := mock_service.NewMockStoryAI(ctrl)
	tman := mock_service.NewMockTxManager(ctrl)
	vardb := mock_service.NewMockVariantDatabase(ctrl)
	msgdb := mock_service.NewMockMessageDatabase(ctrl)
	cfg := &config.Config{}
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serv := service.NewStoryService(setcache, subdb, dcache, ddb, stdb, stai, tman, vardb, msgdb, log)
	userID := int64(64)
	stdb.EXPECT().GetActiveStories(gomock.Any(), userID).Return([]*models.Story{}, nil)
	heroes := &models.FantasyCharacters{Characters: []models.Hero{
		{Name: "something....."},
	}}
	dcache.EXPECT().CheckExceededLimit(gomock.Any(), userID).Return(false, nil)
	ddb.EXPECT().GetDailyLimit(gomock.Any(), userID).Return(nil, nil)
	subdb.EXPECT().GetActiveSubscriptions(gomock.Any(), userID).Return([]*models.Subscription{}, nil)
	setcache.EXPECT().GetSetting(gomock.Any(), models.SettingKeyLimitBaseDay).Return("20", nil)
	stai.EXPECT().GetStructuredHeroes(gomock.Any()).Return(heroes, nil)
	newLimit := models.NewDailyLimit(userID, 0, 20)
	data, _ := json.Marshal(heroes)
	var tx pgx.Tx
	tman.EXPECT().BeginTx(gomock.Any()).Return(tx, nil)
	story := models.NewStory(userID, nil)
	storyID := 1
	stdb.EXPECT().AddStory(gomock.Any(), tx, story).Return(storyID, nil)
	variant := models.NewStoryVariant(storyID, "characters", data)
	vardb.EXPECT().AddVariant(gomock.Any(), tx, variant).Return(nil)
	newLimit.Count += 2
	ddb.EXPECT().AddDailyLimit(gomock.Any(), tx, newLimit)
	tman.EXPECT().CommitTx(gomock.Any(), tx).Return(nil)
	response, err := serv.CreateStory(ctx, userID)
	require.Nil(t, err)
	require.Equal(t, response, text_messages.NewChouseHero(heroes))
}
