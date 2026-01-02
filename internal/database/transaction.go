package database

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTransaction executes function within a transaction
// Automatically commits on success, rolls back on error
func WithTransaction(db *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
			if err != nil {
				err = fmt.Errorf("failed to commit transaction: %w", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

type IsolationLevel string

// TransactionIsolationLevel constants for transaction isolation
const (
	IsolationReadUncommitted IsolationLevel = "READ UNCOMMITTED"
	IsolationReadCommitted   IsolationLevel = "READ COMMITTED"
	IsolationRepeatableRead  IsolationLevel = "REPEATABLE READ" // MySQL default
	IsolationSerializable    IsolationLevel = "SERIALIZABLE"
)

func toSQLIsolation(level IsolationLevel) (sql.IsolationLevel, error) {
	switch level {
	case IsolationReadUncommitted:
		return sql.LevelReadUncommitted, nil
	case IsolationReadCommitted:
		return sql.LevelReadCommitted, nil
	case IsolationRepeatableRead:
		return sql.LevelRepeatableRead, nil
	case IsolationSerializable:
		return sql.LevelSerializable, nil
	default:
		return 0, fmt.Errorf("invalid isolation level: %q", level)
	}
}

// WithTransactionAndIsolation starts transaction with specific isolation level
func WithTransactionAndIsolation(ctx context.Context, db *sql.DB, level IsolationLevel, fn func(*sql.Tx) error) (err error) {
	iso, err := toSQLIsolation(level)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: iso})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	err = fn(tx)
	return err
}
