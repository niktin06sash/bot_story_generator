package repository

import (
	"bot_story_generator/internal/database"
	"context"

	"github.com/jackc/pgx/v5"
)

type TxManagerImpl struct {
	databaseclient *database.DBObject
}

func NewTxManager(db *database.DBObject) *TxManagerImpl {
	return &TxManagerImpl{
		databaseclient: db,
	}
}
func (r *TxManagerImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.databaseclient.Pool.Begin(ctx)
}

func (r *TxManagerImpl) RollbackTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Rollback(ctx)
}

func (r *TxManagerImpl) CommitTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Commit(ctx)
}
