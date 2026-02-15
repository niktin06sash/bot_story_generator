package service_test

import (
	"bot_story_generator/internal/config"
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
	cfg := &config.Config{}
	mockDBUser := mock_service.NewMockUserDatabase(ctrl)
	mockCacheUser := mock_service.NewMockUserCache(ctrl)
	log, _ := logger.NewLogger(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serv := service.NewUserService(mockDBUser, mockCacheUser, log)
	userID := int64(1)
	user := models.NewUser(userID)
	tests := []struct {
		name             string
		setupMocks       func()
		expectedResponse string
		expectedError    error
	}{
		{
			name: "Success",
			setupMocks: func() {
				mockCacheUser.EXPECT().CheckCreatedUser(gomock.Any(), userID).Return(false, nil)
				mockDBUser.EXPECT().AddUser(gomock.Any(), user).Return(nil)
			},
			expectedResponse: text_messages.TextGreeting,
			expectedError:    nil,
		},
		{
			name: "DB client error",
			setupMocks: func() {
				mockCacheUser.EXPECT().CheckCreatedUser(gomock.Any(), userID).Return(false, nil)
				mockDBUser.EXPECT().AddUser(gomock.Any(), user).Return(errors.New("client: user is already registered"))
				mockCacheUser.EXPECT().AddCreatedUser(gomock.Any(), userID).Return(nil)
			},
			expectedResponse: "",
			expectedError:    errors.New(text_messages.TextGreeting),
		},
		{
			name: "DB server error",
			setupMocks: func() {
				mockCacheUser.EXPECT().CheckCreatedUser(gomock.Any(), userID).Return(false, nil)
				mockDBUser.EXPECT().AddUser(gomock.Any(), user).Return(errors.New("server: database error: internal error"))
			},
			expectedResponse: "",
			expectedError:    errors.New(text_messages.TextErrorCreateTask),
		},
		{
			name: "Cache already has user",
			setupMocks: func() {
				mockCacheUser.EXPECT().CheckCreatedUser(gomock.Any(), userID).Return(true, nil)
			},
			expectedResponse: "",
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
