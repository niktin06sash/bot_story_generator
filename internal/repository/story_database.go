package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"errors"
	"fmt"
	"time"

	"context"

	"github.com/jackc/pgx/v5"
)

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewStoryDatabase(db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{
		databaseclient: db,
	}
}

// USERS
func (s *StoryDatabaseImpl) AddUser(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (ID)
        VALUES ($1)
        ON CONFLICT(ID) DO NOTHING 
        RETURNING ID
    `
	var insertedID int64
	err := s.databaseclient.Pool.QueryRow(ctx, query, user.ID).Scan(&insertedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("client: user is already registered")
		}
		return fmt.Errorf("server: database error: %w", err)
	}

	return nil
}

// STORIES
func (s *StoryDatabaseImpl) GetActiveStories(ctx context.Context, userID int64) ([]*models.Story, error) {
	query := `
    	SELECT ID, userID, data, createdAt
    	FROM stories
    	WHERE userID = $1 and isActive = TRUE
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()
	var stories []*models.Story
	for rows.Next() {
		story := &models.Story{}
		err := rows.Scan(&story.ID, &story.UserID, &story.Data, &story.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return stories, nil
}

func (s *StoryDatabaseImpl) AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error) {
	query := `
		INSERT INTO stories (userID, data)
    	VALUES ($1, $2)
		RETURNING ID
	`
	row := tx.QueryRow(ctx, query, story.UserID, story.Data)
	var storyID int
	err := row.Scan(&storyID)
	if err != nil {
		return 0, fmt.Errorf("server: database error: %w", err)
	}
	return storyID, nil
}

func (s *StoryDatabaseImpl) StopStory(ctx context.Context, userID int64) error {
	query := `
		UPDATE stories 
		SET isActive = FALSE 
		WHERE userID = $1 AND isActive = TRUE
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// VARIANTS
func (s *StoryDatabaseImpl) AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	query := `
		INSERT INTO storiesVariants (storyID, data, type)
    	VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(ctx, query, variant.StoryID, variant.Data, variant.Type)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	query := `
		UPDATE storiesVariants
		SET data = $2, type = $3
		WHERE storyID = $1
	`
	_, err := tx.Exec(ctx, query, variant.StoryID, variant.Data, variant.Type)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error) {
	query := `
		SELECT sv.storyid, sv.data, sv.type
		FROM storiesVariants sv
		INNER JOIN stories s ON sv.storyid = s.id
		WHERE s.userid = $1 and s.isactive = true
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()

	var variants []*models.StoryVariant
	for rows.Next() {
		variant := &models.StoryVariant{}
		err := rows.Scan(&variant.StoryID, &variant.Data, &variant.Type)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return variants, nil
}

// LIMITS
func (s *StoryDatabaseImpl) GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error) {
	query := `
		SELECT userID, date, count, limitCount FROM dailyLimits
		WHERE userID = $1 and date = CURRENT_DATE
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID)
	limit := &models.DailyLimit{}
	err := row.Scan(&limit.UserID, &limit.Date, &limit.Count, &limit.LimitCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return limit, nil
}

func (s *StoryDatabaseImpl) AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	query := `
        INSERT INTO dailyLimits (userID, count, limitCount)
        VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(ctx, query, dailyLimit.UserID, dailyLimit.Count, dailyLimit.LimitCount)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) UpdateDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	query := `
        UPDATE dailyLimits 
        SET count = $1
        WHERE userID = $2 AND date = CURRENT_DATE
    `
	_, err := tx.Exec(ctx, query, dailyLimit.Count, dailyLimit.UserID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// MESSAGES
func (s *StoryDatabaseImpl) AddStoryMessages(ctx context.Context, tx pgx.Tx, msgs []*models.StoryMessage) error {
	//делаем batch вместо двух запросов insert
	batch := &pgx.Batch{}
	query := `
		INSERT INTO storiesMessages (storyID, data, type)
		VALUES ($1, $2, $3)
	`
	for _, msg := range msgs {
		batch.Queue(query, msg.StoryID, msg.Data, msg.Type)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range msgs {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("server: database error: %w", err)
		}
	}
	return nil
}

func (s *StoryDatabaseImpl) GetAllStorySegments(ctx context.Context, storyID int) ([]*models.StoryMessage, error) {
	//TODO придумать, как не делать select + for по всем сообщениям истории(возможно, Redis пригодится)
	query := `
		SELECT data, type
		FROM storiesMessages WHERE storyID = $1
		ORDER BY createdAt ASC
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, storyID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()
	msgs := make([]*models.StoryMessage, 0)
	for rows.Next() {
		msg := &models.StoryMessage{}
		if err := rows.Scan(&msg.Data, &msg.Type); err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return msgs, nil
}

// AddSubscription добавляет новую подписку для пользователя
func (s *StoryDatabaseImpl) AddSubscription(ctx context.Context, subscription *models.Subscription) error {
	query := `
		INSERT INTO subscriptions (payload, chargeId, userID, type, status, startDate, endDate, isAutoRenewal, currency, price)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query,
		subscription.Payload,
		subscription.ChargeId,
		subscription.UserID,
		subscription.Type,
		subscription.Status,
		subscription.StartDate,
		subscription.EndDate,
		subscription.IsAutoRenewal,
		subscription.Currency,
		subscription.Price,
	)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
func (s *StoryDatabaseImpl) GetPendingSubscription(ctx context.Context, payload string, userID int64) (*models.Subscription, error) {
	query := `
		SELECT payload, chargeId, userID, type, status, startDate, endDate, isAutoRenewal, currency, price
		FROM subscriptions 
        WHERE userID = $1 AND payload = $2 AND status = 'pending'
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID, payload)
	sub := &models.Subscription{}
	err := row.Scan(
		&sub.Payload,
		&sub.ChargeId,
		&sub.UserID,
		&sub.Type,
		&sub.Status,
		&sub.StartDate,
		&sub.EndDate,
		&sub.IsAutoRenewal,
		&sub.Currency,
		&sub.Price,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("client: undefined payload")
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return sub, nil
}
func (s *StoryDatabaseImpl) UpdatePendingSubscription(ctx context.Context, payload string, userID int64, start time.Time, end time.Time, changeID string) error {
	query := `
		UPDATE subscriptions 
        SET 
            status = 'paid',
            startDate = $3,
            endDate = $4,
            chargeId = $5
        WHERE 
            userID = $1 
            AND payload = $2 
            AND status = 'pending'
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID, payload, start, end, changeID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// GetUserSubscription возвращает подписку пользователя по userID
func (s *StoryDatabaseImpl) GetActiveSubscriptions(ctx context.Context, userID int64) ([]*models.Subscription, error) {
	query := `
		SELECT userID, type, startDate, endDate, isAutoRenewal, chargeId, payload, status, currency, price
        FROM subscriptions 
        WHERE userID = $1
        AND endDate > NOW()
		AND status = 'paid'
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()

	var subs []*models.Subscription
	for rows.Next() {
		sub := &models.Subscription{}
		err := rows.Scan(&sub.UserID, &sub.Type, &sub.StartDate, &sub.EndDate, &sub.IsAutoRenewal, &sub.ChargeId, &sub.Payload, &sub.Status, &sub.Currency, &sub.Price)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return subs, nil
}

// SETTINGS
func (s *StoryDatabaseImpl) GetAllSettings(ctx context.Context) ([]*models.Setting, error) {
	query := `
		SELECT key, value, updated_at, updated_by
		FROM settings
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()

	var res []*models.Setting
	for rows.Next() {
		var st models.Setting
		if err := rows.Scan(&st.Key, &st.Value, &st.UpdatedAt, &st.UpdatedBy); err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		res = append(res, &st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return res, nil
}

func (s *StoryDatabaseImpl) GetSetting(ctx context.Context, key string) (*models.Setting, error) {
	query := `
		SELECT key, value, updated_at, updated_by
		FROM settings
		WHERE key = $1
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, key)
	st := &models.Setting{}
	if err := row.Scan(&st.Key, &st.Value, &st.UpdatedAt, &st.UpdatedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return st, nil
}

func (s *StoryDatabaseImpl) SetSetting(ctx context.Context, tx pgx.Tx, setting *models.Setting) error {
	query := `
		INSERT INTO settings (key, value, updated_at, updated_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (key) DO UPDATE
		SET value = $2,
			updated_at = now(),
			updated_by = $3
	`
	_, err := tx.Exec(ctx, query, setting.Key, setting.Value, setting.UpdatedBy)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
