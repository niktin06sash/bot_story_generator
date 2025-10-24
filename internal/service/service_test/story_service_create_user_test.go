package service_test

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/service"
	mock_service "bot_story_generator/internal/service/mocks"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateUser_Success(t *testing.T) {
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
	user := models.NewUser(1, 1)
	mockdb.EXPECT().AddUser(ctx, user).Return(nil)
	response, err := serv.CreateUser(ctx, 1, 1)
	require.Nil(t, err)
	require.Equal(t, response, text_messages.TextGreeting)
}
func TestCreateUser_ClientError(t *testing.T) {
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
	user := models.NewUser(1, 1)
	mockdb.EXPECT().AddUser(ctx, user).Return(errors.New("client: user with user_id=1 is already registered"))
	response, err := serv.CreateUser(ctx, 1, 1)
	require.NotNil(t, err)
	require.Equal(t, response, text_messages.TextHelp())
}
func TestCreateUser_ServerError(t *testing.T) {
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
	user := models.NewUser(1, 1)
	mockdb.EXPECT().AddUser(ctx, user).Return(errors.New("server: database error: internal error"))
	response, err := serv.CreateUser(ctx, 1, 1)
	require.NotNil(t, err)
	require.Equal(t, response, text_messages.TextErrorCreateTask)
}
