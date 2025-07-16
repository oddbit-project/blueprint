//go:build integration && pgsql
// +build integration,pgsql

package db

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryFactory tests repository factory methods
func TestRepositoryFactory(t *testing.T) {
	// Get database URL from environment
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping factory tests")
	}

	t.Run("NewRepository", func(t *testing.T) {
		t.Run("CreateRepositoryWithValidDB", func(t *testing.T) {
			// Connect to database
			client := pgClientFromUrl(dbURL)
			defer client.Disconnect()

			// Create repository
			repo := NewRepository(context.Background(), client, "test_table")
			require.NotNil(t, repo)

			// Verify repository properties
			assert.Equal(t, "test_table", repo.Name())
			assert.NotNil(t, repo.Db())
		})

		t.Run("CreateRepositoryWithContext", func(t *testing.T) {
			// Connect to database
			client := pgClientFromUrl(dbURL)
			defer client.Disconnect()

			// Create repository with custom context
			ctx := context.Background()
			repo := NewRepository(ctx, client, "context_test_table")
			require.NotNil(t, repo)

			// Verify repository properties
			assert.Equal(t, "context_test_table", repo.Name())
			assert.NotNil(t, repo.Db())
		})
	})

	t.Run("PostgreSQLRepositories", func(t *testing.T) {
		// Connect to database
		client := pgClientFromUrl(dbURL)
		defer client.Disconnect()

		t.Run("CreateRepositoryWithValidTableName", func(t *testing.T) {
			repo := NewRepository(context.Background(), client, "test_users")
			require.NotNil(t, repo)

			// Verify repository properties
			assert.Equal(t, "test_users", repo.Name())
			assert.NotNil(t, repo.Db())
		})

		t.Run("CreateRepositoryWithEmptyTableName", func(t *testing.T) {
			repo := NewRepository(context.Background(), client, "")
			require.NotNil(t, repo)

			// Repository should be created with empty table name
			assert.Equal(t, "", repo.Name())
		})

		t.Run("CreateMultipleRepositories", func(t *testing.T) {
			repo1 := NewRepository(context.Background(), client, "table1")
			repo2 := NewRepository(context.Background(), client, "table2")

			require.NotNil(t, repo1)
			require.NotNil(t, repo2)

			// Should be different repositories
			assert.NotEqual(t, repo1.Name(), repo2.Name())
			assert.Equal(t, "table1", repo1.Name())
			assert.Equal(t, "table2", repo2.Name())

			// But share same DB connection
			assert.Equal(t, repo1.Db(), repo2.Db())
		})

		t.Run("RepositoryOperations", func(t *testing.T) {
			// Create test table
			createTableSQL := `
				DROP TABLE IF EXISTS factory_test_users;
				CREATE TABLE factory_test_users (
					id SERIAL PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					email VARCHAR(255) UNIQUE NOT NULL
				);
			`
			_, err := client.Db().Exec(createTableSQL)
			require.NoError(t, err)
			defer client.Db().Exec("DROP TABLE IF EXISTS factory_test_users")

			// Create repository
			repo := NewRepository(context.Background(), client, "factory_test_users")
			require.NotNil(t, repo)

			// Test basic operations
			type FactoryTestUser struct {
				ID    int64  `db:"id,auto"`
				Name  string `db:"name"`
				Email string `db:"email"`
			}

			// Insert
			user := FactoryTestUser{
				Name:  "Factory Test User",
				Email: "factory@example.com",
			}
			err = repo.Insert(&user)
			require.NoError(t, err)

			// Fetch
			var result FactoryTestUser
			err = repo.FetchByKey("email", "factory@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "Factory Test User", result.Name)

			// Count
			count, err := repo.Count()
			require.NoError(t, err)
			assert.Equal(t, int64(1), count)
		})
	})

	t.Run("RepositoryWithClosedDB", func(t *testing.T) {
		// Create and immediately close DB
		client := pgClientFromUrl(dbURL)
		client.Disconnect()

		repo := NewRepository(context.Background(), client, "closed_db_table")
		require.NotNil(t, repo)

		// Operations should fail with closed DB
		count, err := repo.Count()
		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("RepositoryTransactions", func(t *testing.T) {
		client := pgClientFromUrl(dbURL)
		defer client.Disconnect()

		// Create test table
		createTableSQL := `
			DROP TABLE IF EXISTS factory_tx_test;
			CREATE TABLE factory_tx_test (
				id SERIAL PRIMARY KEY,
				value VARCHAR(255)
			);
		`
		_, err := client.Db().Exec(createTableSQL)
		require.NoError(t, err)
		defer client.Db().Exec("DROP TABLE IF EXISTS factory_tx_test")

		repo := NewRepository(context.Background(), client, "factory_tx_test")

		t.Run("CreateTransaction", func(t *testing.T) {
			tx, err := repo.NewTransaction(nil)
			require.NoError(t, err)
			require.NotNil(t, tx)

			// Transaction should have same table name
			assert.Equal(t, repo.Name(), tx.Name())

			// Rollback to clean up
			err = tx.Rollback()
			require.NoError(t, err)
		})

		t.Run("TransactionWithOptions", func(t *testing.T) {
			opts := &sql.TxOptions{
				Isolation: sql.LevelSerializable,
				ReadOnly:  false,
			}

			tx, err := repo.NewTransaction(opts)
			require.NoError(t, err)
			require.NotNil(t, tx)

			// Rollback to clean up
			err = tx.Rollback()
			require.NoError(t, err)
		})
	})
}

// TestRepositoryIdentifier tests the Identifier interface methods
func TestRepositoryIdentifier(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping identifier tests")
	}

	client := pgClientFromUrl(dbURL)
	defer client.Disconnect()

	t.Run("RepositoryDb", func(t *testing.T) {
		repo := NewRepository(context.Background(), client, "identifier_test")

		dbConn := repo.Db()
		require.NotNil(t, dbConn)

		// DB should be usable
		var result int
		err := dbConn.Get(&result, "SELECT 1")
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	})

	t.Run("RepositoryName", func(t *testing.T) {
		testCases := []struct {
			tableName string
			expected  string
		}{
			{"users", "users"},
			{"test_table", "test_table"},
			{"CamelCaseTable", "CamelCaseTable"},
			{"table-with-dash", "table-with-dash"},
			{"", ""},
			{"very_long_table_name_with_many_underscores", "very_long_table_name_with_many_underscores"},
		}

		for _, tc := range testCases {
			t.Run(tc.tableName, func(t *testing.T) {
				repo := NewRepository(context.Background(), client, tc.tableName)
				assert.Equal(t, tc.expected, repo.Name())
			})
		}
	})

	t.Run("TransactionDb", func(t *testing.T) {
		repo := NewRepository(context.Background(), client, "tx_identifier_test")

		tx, err := repo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		txDb := tx.Db()
		require.NotNil(t, txDb)

		// Should be usable as a database interface
		assert.NotNil(t, txDb, "Transaction Db() should return a valid database connection")

		// Tx should be usable
		var result int
		err = txDb.Get(&result, "SELECT 2")
		require.NoError(t, err)
		assert.Equal(t, 2, result)
	})

	t.Run("TransactionName", func(t *testing.T) {
		repo := NewRepository(context.Background(), client, "tx_name_test")

		tx, err := repo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Transaction should have same name as repository
		assert.Equal(t, repo.Name(), tx.Name())
		assert.Equal(t, "tx_name_test", tx.Name())
	})
}
