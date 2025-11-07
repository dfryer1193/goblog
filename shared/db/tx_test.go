package db

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	return db
}

func TestRunInTransaction_NewTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := RunInTransaction(ctx, db, func(txCtx context.Context) error {
		// Verify transaction is in context
		if _, ok := GetTx(txCtx); !ok {
			t.Error("Expected transaction in context")
		}

		executor := GetExecutor(txCtx, db)
		_, err := executor.ExecContext(txCtx, "INSERT INTO test_table (value) VALUES (?)", "test")
		return err
	})

	if err != nil {
		t.Fatalf("RunInTransaction failed: %v", err)
	}

	// Verify data was committed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

func TestRunInTransaction_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := RunInTransaction(ctx, db, func(txCtx context.Context) error {
		executor := GetExecutor(txCtx, db)
		_, err := executor.ExecContext(txCtx, "INSERT INTO test_table (value) VALUES (?)", "test")
		if err != nil {
			return err
		}
		// Return error to trigger rollback
		return sql.ErrTxDone
	})

	if err == nil {
		t.Fatal("Expected error from RunInTransaction")
	}

	// Verify data was rolled back
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 rows (rollback), got %d", count)
	}
}

func TestRunInTransaction_NestedTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := RunInTransaction(ctx, db, func(outerCtx context.Context) error {
		// First insert in outer transaction
		executor := GetExecutor(outerCtx, db)
		_, err := executor.ExecContext(outerCtx, "INSERT INTO test_table (value) VALUES (?)", "outer")
		if err != nil {
			return err
		}

		// Nested RunInTransaction should reuse the same transaction
		return RunInTransaction(outerCtx, db, func(innerCtx context.Context) error {
			// Verify we're using the same transaction
			outerTx, _ := GetTx(outerCtx)
			innerTx, _ := GetTx(innerCtx)
			
			if outerTx != innerTx {
				t.Error("Expected nested transaction to reuse outer transaction")
			}

			// Second insert in nested "transaction"
			executor := GetExecutor(innerCtx, db)
			_, err := executor.ExecContext(innerCtx, "INSERT INTO test_table (value) VALUES (?)", "inner")
			return err
		})
	})

	if err != nil {
		t.Fatalf("RunInTransaction failed: %v", err)
	}

	// Verify both inserts were committed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestRunInTransaction_NestedRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := RunInTransaction(ctx, db, func(outerCtx context.Context) error {
		// First insert in outer transaction
		executor := GetExecutor(outerCtx, db)
		_, err := executor.ExecContext(outerCtx, "INSERT INTO test_table (value) VALUES (?)", "outer")
		if err != nil {
			return err
		}

		// Nested transaction that fails
		err = RunInTransaction(outerCtx, db, func(innerCtx context.Context) error {
			executor := GetExecutor(innerCtx, db)
			_, err := executor.ExecContext(innerCtx, "INSERT INTO test_table (value) VALUES (?)", "inner")
			if err != nil {
				return err
			}
			// Return error - should rollback entire outer transaction
			return sql.ErrTxDone
		})

		return err // Propagate error to outer transaction
	})

	if err == nil {
		t.Fatal("Expected error from RunInTransaction")
	}

	// Verify all data was rolled back (including outer insert)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 rows (complete rollback), got %d", count)
	}
}

func TestGetExecutor_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	txCtx := WithTx(ctx, tx)
	executor := GetExecutor(txCtx, db)

	// Verify executor is the transaction
	if executor != tx {
		t.Error("Expected executor to be the transaction")
	}
}

func TestGetExecutor_WithoutTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	executor := GetExecutor(ctx, db)

	// Verify executor is the database
	if executor != db {
		t.Error("Expected executor to be the database")
	}
}
