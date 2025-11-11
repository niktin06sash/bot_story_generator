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
	Type      string    `json:"type"`
}

// NewStoryMessage создает новое сообщение истории
func NewStoryMessage(storyID int, data string, t string) *StoryMessage {
	return &StoryMessage{
		StoryID: storyID,
		Data:    data,
		Type:    t,
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

// Subscription представляет подписку
type Subscription struct {
	Payload       string    `json:"payload"`
	ChargeId      string    `json:"charge_id"`
	UserID        int64     `json:"user_id"`
	Type          string    `json:"type"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	IsAutoRenewal bool      `json:"is_auto_renewal"`
	Status        string    `json:"status"`
	Currency      string    `json:"currency"`
	Price         int       `json:"price"`
}

func NewSubscription(userID int64, t string, payload string, status string, currency string, price int) *Subscription {
	return &Subscription{
		UserID:        userID,
		Type:          t,
		IsAutoRenewal: true,
		Payload:       payload,
		Status:        status,
		Currency:      currency,
		Price:         price,
	}
}

// Setting представляет запись из таблицы settings
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy int64     `json:"updated_by"`
}

// NewSetting создает новую настройку
func NewSetting(key string, value string, updatedBy int64) *Setting {
	return &Setting{
		Key:   key,
		Value: value,
		UpdatedBy: updatedBy,
	}
}

// начальные ключи настроек
const (
	SettingKeyPriceBasicSubscription = "sub.basic.price"
	SettingKeyLimitBaseDay = "limit.day.base"
	SettingKeyLimitPremiumDay = "limit.day.premium"
)

func NameSettingKeys() []string {
	return []string{
		SettingKeyPriceBasicSubscription,
		SettingKeyLimitBaseDay,
		SettingKeyLimitPremiumDay,
	}
}
