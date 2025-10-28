package models

import "time"

// User представляет пользователя в системе
type User struct {
	ID    int64 `json:"id"`
	IsSub bool  `json:"is_sub"`
}

// NewUser создает нового пользователя
func NewUser(userID int64) *User {
	return &User{
		ID:    userID,
		IsSub: false,
	}
}

// Story представляет историю в системе
type Story struct {
	ID        int       `json:"id"`
	UserID    int64     `json:"user_id"`
	Data      []byte    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
}

// NewStory создает новую историю
func NewStory(userID int64, data []byte) *Story {
	return &Story{
		UserID:   userID,
		Data:     data,
		IsActive: true,
	}
}

// StoryMessage представляет сообщение в истории
type StoryMessage struct {
	ID        int       `json:"id"`
	StoryID   int       `json:"story_id"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

// NewStoryMessage создает новое сообщение истории
func NewStoryMessage(storyID int, data string) *StoryMessage {
	return &StoryMessage{
		StoryID: storyID,
		Data:    data,
	}
}

// StoryVariant представляет варианты развития истории
type StoryVariant struct {
	StoryID int    `json:"story_id"`
	Type    string `json:"type"`
	Data    []byte `json:"data"`
}

// NewStoryVariant создает новый вариант истории
func NewStoryVariant(storyID int, t string, data []byte) *StoryVariant {
	return &StoryVariant{
		StoryID: storyID,
		Data:    data,
		Type:    t,
	}
}

// DailyLimit представляет дневные лимиты пользователя
type DailyLimit struct {
	UserID     int64     `json:"user_id"`
	Date       time.Time `json:"date"`
	Count      int       `json:"msg_count"`
	LimitCount int       `json:"daily_limit"`
}

// NewDailyLimit создает новый дневной лимит
func NewDailyLimit(userID int64, msgCount int, dailyLimit int) *DailyLimit {
	return &DailyLimit{
		UserID:     userID,
		Count:      msgCount,
		LimitCount: dailyLimit,
	}
}
