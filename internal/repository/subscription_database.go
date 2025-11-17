package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type SubscriptionDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewSubscriptionDatabase(db *database.DBObject) *SubscriptionDatabaseImpl {
	return &SubscriptionDatabaseImpl{
		databaseclient: db,
	}
}
func (s *SubscriptionDatabaseImpl) AddSubscription(ctx context.Context, subscription *models.Subscription) error {
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

// UpdateSubscription обновляет данные подписки пользователя
func (s *SubscriptionDatabaseImpl) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	query := `
		UPDATE subscriptions
		SET
			type = $1,
			status = $2,
			startDate = $3,
			endDate = $4,
			isAutoRenewal = $5,
			currency = $6,
			price = $7,
			chargeId = $8
		WHERE
			payload = $9 AND userID = $10
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query,
		subscription.Type,
		subscription.Status,
		subscription.StartDate,
		subscription.EndDate,
		subscription.IsAutoRenewal,
		subscription.Currency,
		subscription.Price,
		subscription.ChargeId,
		subscription.Payload,
		subscription.UserID,
	)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *SubscriptionDatabaseImpl) GetStatusSubscription(ctx context.Context, payload string, userID int64) (*models.Subscription, error) {
	query := `
		SELECT payload, userID, status
		FROM subscriptions 
        WHERE userID = $1 AND payload = $2
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID, payload)
	sub := &models.Subscription{}
	err := row.Scan(
		&sub.Payload,
		&sub.UserID,
		&sub.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("client: undefined payload")
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return sub, nil
}

func (s *SubscriptionDatabaseImpl) PayedPendingSubscription(ctx context.Context, payload string, userID int64, start time.Time, end time.Time, changeID string) error {
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
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID, payload, start, end, changeID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
func (s *SubscriptionDatabaseImpl) RejectedPendingSubscription(ctx context.Context, payload string, userID int64) error {
	query := `
		UPDATE subscriptions 
        SET 
            status = 'rejected',
        WHERE 
            userID = $1 
            AND payload = $2 
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID, payload)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// GetUserSubscription возвращает подписку пользователя по userID
func (s *SubscriptionDatabaseImpl) GetActiveSubscriptions(ctx context.Context, userID int64) ([]*models.Subscription, error) {
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
