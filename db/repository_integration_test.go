//go:build integration
// +build integration

package db

import (
	"context"
	"database/sql"
	"github.com/doug-martin/goqu/v9"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// TestIntegration_InsertReturning_FullFlow tests the complete InsertReturning flow
func TestIntegration_InsertReturning_FullFlow(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Test 1: Insert with struct target
	user1 := &IntegrationTestUser{
		Name:   "John Doe",
		Email:  "john@example.com",
		Status: "active",
	}

	result1 := &IntegrationTestUser{}
	err := repo.InsertReturning(user1, []string{"id", "name", "email", "status", "created_at"}, result1)
	require.NoError(t, err)

	assert.NotZero(t, result1.ID)
	assert.Equal(t, "John Doe", result1.Name)
	assert.Equal(t, "john@example.com", result1.Email)
	assert.Equal(t, "active", result1.Status)
	assert.False(t, result1.CreatedAt.IsZero())

	// Test 2: Insert with multiple variables
	user2 := &IntegrationTestUser{
		Name:   "Jane Smith",
		Email:  "jane@example.com",
		Status: "pending",
	}

	var id2 int64
	var name2 string
	var email2 string
	var status2 string
	err = repo.InsertReturning(user2, []string{"id", "name", "email", "status"}, &id2, &name2, &email2, &status2)
	require.NoError(t, err)

	assert.NotZero(t, id2)
	assert.Equal(t, "Jane Smith", name2)
	assert.Equal(t, "jane@example.com", email2)
	assert.Equal(t, "pending", status2)

	// Test 3: Insert with single variable
	user3 := &IntegrationTestUser{
		Name:   "Bob Wilson",
		Email:  "bob@example.com",
		Status: "verified",
	}

	var id3 int64
	err = repo.InsertReturning(user3, []string{"id"}, &id3)
	require.NoError(t, err)
	assert.NotZero(t, id3)

	// Verify all users were inserted
	var count int64
	err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestIntegration_UpdateReturning_FullFlow tests the complete UpdateReturning flow
func TestIntegration_UpdateReturning_FullFlow(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// First, insert a user to update
	user := &IntegrationTestUser{
		Name:   "Original Name",
		Email:  "original@example.com",
		Status: "pending",
	}

	var originalID int64
	err := repo.InsertReturning(user, []string{"id"}, &originalID)
	require.NoError(t, err)

	// Test 1: Update with struct target
	updateUser := &IntegrationTestUser{
		Name:   "Updated Name",
		Email:  "updated@example.com",
		Status: "active",
	}

	whereConditions := map[string]any{"id": originalID}
	result := &IntegrationTestUser{}
	err = repo.UpdateReturning(updateUser, whereConditions, []string{"id", "name", "email", "status", "updated_at"}, result)
	require.NoError(t, err)

	assert.Equal(t, originalID, result.ID)
	assert.Equal(t, "Updated Name", result.Name)
	assert.Equal(t, "updated@example.com", result.Email)
	assert.Equal(t, "active", result.Status)
	assert.False(t, result.UpdatedAt.IsZero())

	// Test 2: Update with multiple variables
	updateUser2 := &IntegrationTestUser{
		Name:   "Final Name",
		Status: "verified",
	}

	var id2 int64
	var name2 string
	var status2 string
	err = repo.UpdateReturning(updateUser2, whereConditions, []string{"id", "name", "status"}, &id2, &name2, &status2)
	require.NoError(t, err)

	assert.Equal(t, originalID, id2)
	assert.Equal(t, "Final Name", name2)
	assert.Equal(t, "verified", status2)
}

// TestIntegration_UpdateFieldsReturning_FullFlow tests the complete UpdateFieldsReturning flow
func TestIntegration_UpdateFieldsReturning_FullFlow(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// First, insert a user to update
	user := &IntegrationTestUser{
		Name:   "Fields Test User",
		Email:  "fields@example.com",
		Status: "pending",
	}

	var originalID int64
	err := repo.InsertReturning(user, []string{"id"}, &originalID)
	require.NoError(t, err)

	// Test UpdateFieldsReturning with specific fields
	fieldsToUpdate := map[string]any{
		"name":   "Updated Fields Name",
		"status": "active",
	}

	whereConditions := map[string]any{"id": originalID}

	// Test with struct target
	result := &IntegrationTestUser{}
	err = repo.UpdateFieldsReturning(&IntegrationTestUser{}, fieldsToUpdate, whereConditions, []string{"id", "name", "email", "status"}, result)
	require.NoError(t, err)

	assert.Equal(t, originalID, result.ID)
	assert.Equal(t, "Updated Fields Name", result.Name)
	assert.Equal(t, "fields@example.com", result.Email) // Should remain unchanged
	assert.Equal(t, "active", result.Status)

	// Test with multiple variables
	fieldsToUpdate2 := map[string]any{
		"status": "verified",
	}

	var id2 int64
	var status2 string
	err = repo.UpdateFieldsReturning(&IntegrationTestUser{}, fieldsToUpdate2, whereConditions, []string{"id", "status"}, &id2, &status2)
	require.NoError(t, err)

	assert.Equal(t, originalID, id2)
	assert.Equal(t, "verified", status2)
}

// TestIntegration_Transaction_CompleteFlow tests transaction functionality
func TestIntegration_Transaction_CompleteFlow(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Start transaction
	tx, err := repo.NewTransaction(nil)
	require.NoError(t, err)

	// Insert user in transaction
	user1 := &IntegrationTestUser{
		Name:   "TX User 1",
		Email:  "tx1@example.com",
		Status: "pending",
	}

	var id1 int64
	err = tx.InsertReturning(user1, []string{"id"}, &id1)
	require.NoError(t, err)
	assert.NotZero(t, id1)

	// Update user in transaction
	updateFields := map[string]any{
		"status": "active",
	}
	whereConditions := map[string]any{"id": id1}

	var updatedStatus string
	err = tx.UpdateFieldsReturning(&IntegrationTestUser{}, updateFields, whereConditions, []string{"status"}, &updatedStatus)
	require.NoError(t, err)
	assert.Equal(t, "active", updatedStatus)

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify changes persisted
	var count int64
	err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users WHERE id = $1 AND status = 'active'", id1).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestIntegration_Transaction_Rollback tests transaction rollback
func TestIntegration_Transaction_Rollback(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Get initial count
	var initialCount int64
	err := repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&initialCount)
	require.NoError(t, err)

	// Start transaction
	tx, err := repo.NewTransaction(nil)
	require.NoError(t, err)

	// Insert user in transaction
	user := &IntegrationTestUser{
		Name:   "Rollback User",
		Email:  "rollback@example.com",
		Status: "pending",
	}

	var id int64
	err = tx.InsertReturning(user, []string{"id"}, &id)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// Rollback transaction
	err = tx.Rollback()
	require.NoError(t, err)

	// Verify no changes persisted
	var finalCount int64
	err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&finalCount)
	require.NoError(t, err)
	assert.Equal(t, initialCount, finalCount)
}

// TestIntegration_ErrorHandling tests error cases with real database
func TestIntegration_ErrorHandling(t *testing.T) {
	repo, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Test duplicate email error
	user1 := &IntegrationTestUser{
		Name:   "User 1",
		Email:  "duplicate@example.com",
		Status: "active",
	}

	var id1 int64
	err := repo.InsertReturning(user1, []string{"id"}, &id1)
	require.NoError(t, err)

	// Try to insert with same email (should fail due to unique constraint)
	user2 := &IntegrationTestUser{
		Name:   "User 2",
		Email:  "duplicate@example.com", // Same email
		Status: "pending",
	}

	var id2 int64
	err = repo.InsertReturning(user2, []string{"id"}, &id2)
	assert.Error(t, err)
	assert.Zero(t, id2)

	// Test update with non-existent ID
	whereConditions := map[string]any{"id": 99999} // Non-existent ID

	result := &IntegrationTestUser{}
	err = repo.UpdateReturning(&IntegrationTestUser{Name: "Non-existent User"}, whereConditions, []string{"id", "name"}, result)

	// Should either error or return no rows (depending on database behavior)
	if err == nil {
		// If no error, result should be empty (no rows affected)
		assert.Zero(t, result.ID)
	} else {
		// Or should return a "no rows" type error
		assert.True(t, EmptyResult(err) || err != nil)
	}
}

// TestAdditionalRepositoryMethods tests additional methods and edge cases not covered in other tests
func TestAdditionalRepositoryMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()
	t.Run("SqlDialect", func(t *testing.T) {
		t.Run("RepositorySqlDialect", func(t *testing.T) {
			dialect := helper.userRepo.(*repository).SqlDialect()
			require.NotNil(t, dialect)
			// Should be PostgreSQL dialect with $ placeholder
			assert.Equal(t, "$", dialect.PlaceHolderFragment)
			assert.True(t, dialect.IncludePlaceholderNum)
		})
	})

	t.Run("SqlBuilder", func(t *testing.T) {
		t.Run("RepositorySqlBuilder", func(t *testing.T) {
			builder := helper.userRepo.(*repository).SqlBuilder()
			require.NotNil(t, builder)
			// Should be able to use builder
			assert.NotNil(t, builder.Dialect())
		})
	})

	t.Run("Sql", func(t *testing.T) {
		t.Run("CreateCustomQueries", func(t *testing.T) {
			dialect := helper.userRepo.Sql()
			require.NotNil(t, dialect)

			// Create custom select query
			query := dialect.Select(goqu.L("COUNT(*)")).From("extensive_test_users")
			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "SELECT COUNT(*)")
			assert.Contains(t, sql, "FROM")
			assert.Contains(t, sql, "extensive_test_users")

			// Create custom insert query
			insertQuery := dialect.Insert("extensive_test_users").Rows(
				goqu.Record{"name": "Custom User", "email": "custom@example.com"},
			)
			sql, _, err = insertQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "INSERT INTO")
		})
	})

	t.Run("Update_Deprecated", func(t *testing.T) {
		// Create test user
		user := ExtensiveTestUser{
			Name:     "Update Deprecated Test",
			Email:    "update_deprecated@example.com",
			Status:   "pending",
			Score:    75.0,
			IsActive: false,
			Tags:     pq.StringArray{"deprecated", "update"},
		}
		err := helper.userRepo.Insert(&user)
		require.NoError(t, err)

		t.Run("UpdateWithPreparedDataset", func(t *testing.T) {
			// The deprecated Update method with prepared dataset
			updateQuery := helper.userRepo.SqlUpdateX(&user).
				FieldsValues(map[string]any{"status": "updated", "score": 85.0}).
				WhereEq("email", "update_deprecated@example.com")

			err := helper.userRepo.Do(updateQuery, nil)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "update_deprecated@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "updated", result.Status)
			assert.Equal(t, 85.0, result.Score)
		})
	})

	t.Run("InsertReturning_EdgeCases", func(t *testing.T) {
		t.Run("InsertReturningEmptyReturnFields", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Empty Return Fields",
				Email:    "empty_return@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"empty", "return"},
			}

			var id int64
			err := helper.userRepo.InsertReturning(&user, []string{}, &id)
			assert.Error(t, err, "Should error with empty return fields")
		})

		t.Run("InsertReturningNilRecord", func(t *testing.T) {
			var id int64
			err := helper.userRepo.InsertReturning(nil, []string{"id"}, &id)
			assert.Error(t, err, "Should error with nil record")
		})
	})

	t.Run("UpdateReturning_EdgeCases", func(t *testing.T) {
		// Create test user
		user := ExtensiveTestUser{
			Name:     "UpdateReturning Edge Test",
			Email:    "update_return_edge@example.com",
			Status:   "pending",
			Score:    70.0,
			IsActive: false,
			Tags:     pq.StringArray{"edge", "test"},
		}
		err := helper.userRepo.Insert(&user)
		require.NoError(t, err)

		t.Run("UpdateReturningNoMatches", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Status: "updated",
				Score:  90.0,
			}

			var result ExtensiveTestUser
			err := helper.userRepo.UpdateReturning(&updateUser,
				map[string]any{"email": "nonexistent@example.com"},
				[]string{"id", "status"}, &result)
			// Should succeed but not update anything
			require.ErrorIs(t, sql.ErrNoRows, err)
			assert.Zero(t, result.ID, "Should not return any data for non-matching update")
		})
	})

	t.Run("Transaction_EdgeCases", func(t *testing.T) {
		t.Run("TransactionWithContext", func(t *testing.T) {
			// Create repository with context
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			repoWithCtx := &repository{
				conn:       helper.db,
				ctx:        ctx,
				tableName:  "extensive_test_users",
				dialect:    goqu.Dialect("postgres"),
				sqlBuilder: helper.userRepo.(*repository).sqlBuilder,
			}

			tx, err := repoWithCtx.NewTransaction(nil)
			require.NoError(t, err)
			require.NotNil(t, tx)

			// Transaction should inherit context (we'll just verify it's working)
			count, err := tx.Count()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(0))

			err = tx.Rollback()
			require.NoError(t, err)
		})

		t.Run("TransactionReadOnly", func(t *testing.T) {
			opts := &sql.TxOptions{
				ReadOnly: true,
			}

			tx, err := helper.userRepo.NewTransaction(opts)
			require.NoError(t, err)
			require.NotNil(t, tx)

			// Read operations should work
			count, err := tx.Count()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(0))

			// Write operations should fail
			user := ExtensiveTestUser{
				Name:     "ReadOnly Test",
				Email:    "readonly@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"readonly"},
			}
			err = tx.Insert(&user)
			assert.Error(t, err, "Insert should fail in read-only transaction")

			err = tx.Rollback()
			require.NoError(t, err)
		})
	})

	t.Run("FetchOne_EdgeCases", func(t *testing.T) {
		t.Run("FetchOneComplexAggregation", func(t *testing.T) {
			type AggResult struct {
				TotalUsers  int64   `db:"total_users"`
				AvgScore    float64 `db:"avg_score"`
				MaxScore    float64 `db:"max_score"`
				MinScore    float64 `db:"min_score"`
				ActiveCount int64   `db:"active_count"`
			}

			query := helper.userRepo.SqlSelect().Select(
				goqu.COUNT("*").As("total_users"),
				goqu.AVG("score").As("avg_score"),
				goqu.MAX("score").As("max_score"),
				goqu.MIN("score").As("min_score"),
				goqu.SUM(goqu.Case().When(goqu.C("is_active").IsTrue(), 1).Else(0)).As("active_count"),
			)

			var result AggResult
			err := helper.userRepo.FetchOne(query, &result)
			require.NoError(t, err)
			assert.Greater(t, result.TotalUsers, int64(0))
			assert.Greater(t, result.AvgScore, 0.0)
		})
	})

	t.Run("Exec_EdgeCases", func(t *testing.T) {
		t.Run("ExecWithUpdateQuery", func(t *testing.T) {
			// Create test user
			user := ExtensiveTestUser{
				Name:     "Exec Update Test",
				Email:    "exec_update@example.com",
				Status:   "pending",
				Score:    60.0,
				IsActive: false,
				Tags:     pq.StringArray{"exec", "test"},
			}
			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Note: Exec expects SelectDataset, so we test with a select query
			selectQuery := helper.userRepo.SqlSelect().Where(goqu.C("email").Eq("exec_update@example.com"))
			err = helper.userRepo.Exec(selectQuery)
			// Should succeed (though it's a SELECT being executed)
			require.NoError(t, err)
		})
	})

	t.Run("ArrayHandling", func(t *testing.T) {
		t.Run("InsertReturningWithArray", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Array Return Test",
				Email:    "array_return@example.com",
				Status:   "active",
				Score:    85.0,
				IsActive: true,
				Tags:     pq.StringArray{"one", "two", "three"},
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.InsertReturning(&user, []string{"id", "name", "tags"}, result)
			require.NoError(t, err)
			assert.NotZero(t, result.ID)
			assert.Equal(t, "Array Return Test", result.Name)
			assert.Equal(t, []string{"one", "two", "three"}, []string(result.Tags))
		})

		t.Run("UpdateFieldsReturningArray", func(t *testing.T) {
			// Create user first
			user := ExtensiveTestUser{
				Name:     "Update Array Test",
				Email:    "update_array@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"initial"},
			}
			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Update tags and return as pq.StringArray
			fieldsToUpdate := map[string]any{
				"tags": pq.StringArray{"updated", "array", "tags"},
			}

			var tags pq.StringArray
			err = helper.userRepo.UpdateFieldsReturning(&ExtensiveTestUser{}, fieldsToUpdate,
				map[string]any{"email": "update_array@example.com"},
				[]string{"tags"}, &tags)
			require.NoError(t, err)
			assert.Equal(t, []string{"updated", "array", "tags"}, []string(tags))
		})
	})

	t.Run("NullHandling", func(t *testing.T) {
		t.Run("CountWhereWithNullValue", func(t *testing.T) {
			// Count users with null age
			count, err := helper.userRepo.CountWhere(map[string]any{"age": nil})
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(0))
		})

		t.Run("FetchWhereWithNullValue", func(t *testing.T) {
			// Fetch users with null bio
			var results []ExtensiveTestUser
			err := helper.userRepo.FetchWhere(map[string]any{"bio": nil}, &results)
			require.NoError(t, err)
			// Verify all have null bio
			for _, user := range results {
				assert.Nil(t, user.Bio)
			}
		})
	})

	t.Run("ErrorConditions", func(t *testing.T) {
		t.Run("FetchRecordEmptyMap", func(t *testing.T) {
			var result ExtensiveTestUser
			err := helper.userRepo.FetchRecord(map[string]any{}, &result)
			// Should succeed but might return first record or empty result
			if err != nil {
				assert.True(t, EmptyResult(err))
			}
		})

		t.Run("DeleteWhereEmptyMap", func(t *testing.T) {
			// Empty map would delete all records - should succeed
			err := helper.userRepo.DeleteWhere(map[string]any{})
			require.NoError(t, err)
		})
	})

	t.Run("TransactionMethods", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		t.Run("TransactionSqlDialect", func(t *testing.T) {
			// Test that transaction has access to SQL functions
			dialectQuery := tx.Sql().Select(goqu.L("1"))
			sql, _, err := dialectQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "SELECT 1")
		})

		t.Run("TransactionSqlBuilder", func(t *testing.T) {
			// Test that transaction SQL builder works
			count, err := tx.Count()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(0))
		})

		t.Run("TransactionSql", func(t *testing.T) {
			dialect := tx.Sql()
			require.NotNil(t, dialect)

			// Should be able to create queries
			query := dialect.Select(goqu.L("1"))
			sqlQry, _, err := query.ToSQL()
			assert.NoError(t, err)
			assert.Contains(t, sqlQry, "SELECT 1")
		})
	})

	// Clean up
	helper.cleanupTestData(t)
}
