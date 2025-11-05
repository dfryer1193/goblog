package db

import (
	"context"
	"database/sql"
	"fmt"
)

// txKey is the key type for storing transaction in context
type txKey struct{}

// WithTx returns a new context with the transaction attached
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// GetTx retrieves the transaction from context if it exists
func GetTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

// GetExecutor returns either a transaction from context or the base db connection
// This allows repositories to execute queries within a transaction if one exists
func GetExecutor(ctx context.Context, db *sql.DB) interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return db
}

// RunInTransaction executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func RunInTransaction(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new context with the transaction
	txCtx := WithTx(ctx, tx)

	// Execute the function
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction after error %v: %w", err, rbErr)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

