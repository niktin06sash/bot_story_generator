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
	mockdb := mock_service.NewMockStoryDatabase(ctrl)
	mockai := mock_service.NewMockStoryAI(ctrl)
	log, _ := logger.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serv := &service.StoryServiceImpl{
		DBStory: mockdb,
		AIStory: mockai,
		Logger:  log,
	}
	heroes := &models.FantasyCharacters{Characters: []models.Hero{
		{Name: "something"},
	}}
	mockdb.EXPECT().CheckActiveStories(ctx, int64(1)).Return(nil)
	mockai.EXPECT().GetStructuredHeroes(ctx).Return(heroes, nil)
	data, _ := json.Marshal(heroes)
	var tx pgx.Tx
	mockdb.EXPECT().BeginTx(ctx).Return(tx, nil)
	story := models.NewStory(1, nil)
	mockdb.EXPECT().AddStory(ctx, tx, story).Return(1, nil)
	variant := models.NewStoryVariant(1, data)
	mockdb.EXPECT().AddVariant(ctx, tx, variant).Return(nil)
	mockdb.EXPECT().CommitTx(ctx, tx).Return(nil)
	response, err := serv.CreateStory(ctx, 1, 1)
	require.Nil(t, err)
	//! ЗАМЕНИ НА НОВЫЙ ВЫВОД ГЕРОЕВ
	require.Equal(t, response, text_messages.TextChooseHero(heroes)) //! ЗАМЕНИ НА НОВЫЙ ВЫВОД ГЕРОЕВ
	//! ЗАМЕНИ НА НОВЫЙ ВЫВОД ГЕРОЕВ
}
