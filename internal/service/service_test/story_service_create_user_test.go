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

func TestCreateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockdb := mock_service.NewMockStoryDatabase(ctrl)
	mockai := mock_service.NewMockStoryAI(ctrl)
	mockcache := mock_service.NewMockStoryCache(ctrl)
	log, _ := logger.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serv := &service.StoryServiceImpl{
		DBStory: mockdb,
		AIStory: mockai,
		Logger:  log,
		CStory:  mockcache,
	}
	userID := int64(1)
	user := models.NewUser(userID)
	tests := []struct {
		name             string
		setupMocks       func()
		expectedResponse []string
		expectedError    error
	}{
		{
			name: "Success",
			setupMocks: func() {
				mockcache.EXPECT().CheckCreatedUser(ctx, userID).Return(false, nil)
				mockdb.EXPECT().AddUser(ctx, user).Return(nil)
			},
			expectedResponse: []string{text_messages.TextGreeting},
			expectedError:    nil,
		},
		{
			name: "DB client error",
			setupMocks: func() {
				mockcache.EXPECT().CheckCreatedUser(ctx, userID).Return(false, nil)
				mockdb.EXPECT().AddUser(ctx, user).Return(errors.New("client: user is already registered"))
				mockcache.EXPECT().AddCreatedUser(ctx, userID).Return(nil)
			},
			expectedResponse: nil,
			expectedError:    errors.New(text_messages.TextGreeting),
		},
		{
			name: "DB server error",
			setupMocks: func() {
				mockcache.EXPECT().CheckCreatedUser(ctx, userID).Return(false, nil)
				mockdb.EXPECT().AddUser(ctx, user).Return(errors.New("server: database error: internal error"))
			},
			expectedResponse: nil,
			expectedError:    errors.New(text_messages.TextErrorCreateTask),
		},
		{
			name: "Cache already has user",
			setupMocks: func() {
				mockcache.EXPECT().CheckCreatedUser(ctx, userID).Return(true, nil)
			},
			expectedResponse: nil,
			expectedError:    errors.New(text_messages.TextGreeting),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			response, err := serv.CreateUser(ctx, userID)
			require.Equal(t, tt.expectedError, err)
			require.Equal(t, tt.expectedResponse, response)
		})
	}
}
