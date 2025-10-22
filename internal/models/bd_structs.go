package models

import "time"

// User представляет пользователя в системе
type User struct {
	ID     int64 `json:"id" db:"id"`
	ChatID int64 `json:"chat_id" db:"chat_id"`
	IsSub  bool  `json:"is_sub" db:"is_sub"`
}

// NewUser создает нового пользователя
func NewUser(chatID int64, isSub bool) User {
	return User{
		ChatID: chatID,
		IsSub:  isSub,
	}
}

// Story представляет историю в системе
type Story struct {
	ID        int       `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	//! ХЗ ПРО DATA - вроде другой тип данных должен быть
	Data      string    `json:"data" db:"data"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	IsActive  bool      `json:"is_active" db:"is_active"`
}

// NewStory создает новую историю
func NewStory(userID int64, data string) Story {
	return Story{
		UserID:   userID,
		Data:     data,
		IsActive: true,
	}
}

// StoryMessage представляет сообщение в истории
type StoryMessage struct {
	ID        int       `json:"id" db:"id"`
	StoryID   int       `json:"story_id" db:"story_id"`
	Data      string    `json:"data" db:"data"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewStoryMessage создает новое сообщение истории
func NewStoryMessage(storyID int, data string) StoryMessage {
	return StoryMessage{
		StoryID: storyID,
		Data:    data,
	}
}

// StoryVariant представляет варианты развития истории
type StoryVariant struct {
	StoryID int    `json:"story_id" db:"story_id"`
	//! ХЗ ПРО DATA - вроде другой тип данных должен быть
	Data    string `json:"data" db:"data"`
}

// NewStoryVariant создает новый вариант истории
func NewStoryVariant(storyID int, data string) StoryVariant {
	return StoryVariant{
		StoryID: storyID,
		Data:    data,
	}
}

// DailyLimit представляет дневные лимиты пользователя
type DailyLimit struct {
	UserID     int64     `json:"user_id" db:"user_id"`
	Date       time.Time `json:"date" db:"date"`
	MsgCount   int       `json:"msg_count" db:"msg_count"`
	DailyLimit int       `json:"daily_limit" db:"daily_limit"`
}

// NewDailyLimit создает новый дневной лимит
func NewDailyLimit(userID int64, msgCount, dailyLimit int) DailyLimit {
	return DailyLimit{
		UserID:     userID,
		MsgCount:   msgCount,
		DailyLimit: dailyLimit,
	}
}
