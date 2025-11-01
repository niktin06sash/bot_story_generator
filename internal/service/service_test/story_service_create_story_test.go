package service_test

import (
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
	mockcache := mock_service.NewMockStoryCache(ctrl)
	mockdb := mock_service.NewMockStoryDatabase(ctrl)
	mockai := mock_service.NewMockStoryAI(ctrl)
	log, _ := logger.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serv := &service.StoryServiceImpl{
		DBStory: mockdb,
		AIStory: mockai,
		Logger:  log,
		CStory:  mockcache,
	}
	userID := int64(64)
	mockcache.EXPECT().CheckExceededLimit(ctx, userID).Return(false, nil)
	newLimit := models.NewDailyLimit(userID, 0, 20)
	mockdb.EXPECT().GetDailyLimit(ctx, userID).Return(newLimit, nil)
	mockdb.EXPECT().GetActiveStories(ctx, userID).Return([]*models.Story{}, nil)
	heroes := &models.FantasyCharacters{Characters: []models.Hero{
		{Name: "something....."},
	}}
	mockai.EXPECT().GetStructuredHeroes(ctx).Return(heroes, nil)
	data, _ := json.Marshal(heroes)
	var tx pgx.Tx
	mockdb.EXPECT().BeginTx(ctx).Return(tx, nil)
	story := models.NewStory(userID, nil)
	storyID := 1
	mockdb.EXPECT().AddStory(ctx, tx, story).Return(storyID, nil)
	variant := models.NewStoryVariant(storyID, "characters", data)
	mockdb.EXPECT().AddVariant(ctx, tx, variant).Return(nil)
	mockdb.EXPECT().AddDailyLimit(ctx, tx, newLimit)
	mockdb.EXPECT().CommitTx(ctx, tx).Return(nil)
	response, err := serv.CreateStory(ctx, userID)
	require.Nil(t, err)
	require.Equal(t, response, text_messages.NewChouseHero(heroes))
}
