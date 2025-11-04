package database

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDBObject(cfg *config.Config, logger *logger.Logger) (*DBObject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.ConnectTimeout)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &DBObject{Pool: pool, logger: logger}, nil
}

type DBObject struct {
	logger *logger.Logger
	Pool   *pgxpool.Pool
}

func (db *DBObject) Close() {
	db.Pool.Close()
	db.logger.ZapLogger.Debug("Successful close Database-connect")
}
