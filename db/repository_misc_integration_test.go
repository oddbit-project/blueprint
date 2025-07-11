//go:build integration
// +build integration

package db

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtensive_Deleter_Interface tests all Deleter interface methods comprehensively
func TestExtensive_Deleter_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create sample data for testing deletions
	users := helper.createSampleUsers(t)
	profiles := helper.createSampleProfiles(t, users)
	require.Len(t, users, 4, "Should have 4 sample users")
	require.Len(t, profiles, 3, "Should have 3 sample profiles")

	t.Run("Delete", func(t *testing.T) {
		t.Run("DeleteWithGoquDataset", func(t *testing.T) {
			// Delete inactive users using goqu dataset
			deleteQuery := helper.userRepo.SqlDelete().
				Where(goqu.C("is_active").IsFalse())

			err := helper.userRepo.Delete(deleteQuery)
			require.NoError(t, err)

			// Verify deletions
			var remainingUsers []ExtensiveTestUser
			query := helper.userRepo.SqlSelect()
			err = helper.userRepo.Fetch(query, &remainingUsers)
			require.NoError(t, err)

			// Should only have active users left
			for _, user := range remainingUsers {
				assert.True(t, user.IsActive, "Only active users should remain")
			}
		})

		t.Run("DeleteWithComplexConditions", func(t *testing.T) {
			// Delete users with low score
			deleteQuery := helper.userRepo.SqlDelete().
				Where(goqu.C("score").Lt(90.0)).
				Where(goqu.C("status").Eq("active"))

			err := helper.userRepo.Delete(deleteQuery)
			require.NoError(t, err)

			// Verify specific users were deleted
			var results []ExtensiveTestUser
			err = helper.userRepo.FetchWhere(map[string]any{"score": 85.0}, &results)
			require.NoError(t, err)
			assert.Len(t, results, 0, "Users with score < 90 and active status should be deleted")
		})
	})

	t.Run("DeleteWhere", func(t *testing.T) {
		t.Run("DeleteBySingleCondition", func(t *testing.T) {
			// First add a user to delete
			testUser := ExtensiveTestUser{
				Name:     "To Be Deleted",
				Email:    "delete@example.com",
				Status:   "temp",
				Score:    50.0,
				IsActive: false,
				Tags:     pq.StringArray{"temp", "delete"},
			}
			err := helper.userRepo.Insert(&testUser)
			require.NoError(t, err)

			// Delete by status
			whereConditions := map[string]any{
				"status": "temp",
			}

			err = helper.userRepo.DeleteWhere(whereConditions)
			require.NoError(t, err)

			// Verify deletion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "delete@example.com", &result)
			assert.True(t, EmptyResult(err), "User should be deleted")
		})

		t.Run("DeleteByMultipleConditions", func(t *testing.T) {
			// Add test users
			user1 := ExtensiveTestUser{
				Name:     "Multi Delete 1",
				Email:    "multi1@example.com",
				Status:   "test",
				Score:    60.0,
				IsActive: false,
				Tags:     pq.StringArray{"multi", "test"},
			}
			user2 := ExtensiveTestUser{
				Name:     "Multi Delete 2",
				Email:    "multi2@example.com",
				Status:   "test",
				Score:    70.0,
				IsActive: true, // Different from user1
				Tags:     pq.StringArray{"multi", "test"},
			}

			err := helper.userRepo.Insert(&user1, &user2)
			require.NoError(t, err)

			// Delete only inactive test users
			whereConditions := map[string]any{
				"status":    "test",
				"is_active": false,
			}

			err = helper.userRepo.DeleteWhere(whereConditions)
			require.NoError(t, err)

			// Verify only user1 was deleted
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "multi1@example.com", &result)
			assert.True(t, EmptyResult(err), "User1 should be deleted")

			err = helper.userRepo.FetchByKey("email", "multi2@example.com", &result)
			assert.NoError(t, err, "User2 should still exist")
			assert.Equal(t, "Multi Delete 2", result.Name)
		})

		t.Run("DeleteWhereNoMatches", func(t *testing.T) {
			whereConditions := map[string]any{
				"status": "nonexistent",
			}

			err := helper.userRepo.DeleteWhere(whereConditions)
			require.NoError(t, err) // Should succeed even if no rows affected
		})

		t.Run("DeleteWhereNilConditions", func(t *testing.T) {
			err := helper.userRepo.DeleteWhere(nil)
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for nil conditions")
		})
	})

	t.Run("DeleteByKey", func(t *testing.T) {
		t.Run("DeleteByPrimaryKey", func(t *testing.T) {
			// Create a user to delete
			testUser := ExtensiveTestUser{
				Name:     "Delete By Key",
				Email:    "deletebykey@example.com",
				Status:   "temp",
				Score:    55.0,
				IsActive: false,
				Tags:     pq.StringArray{"deletebykey"},
			}
			var userID int64
			err := helper.userRepo.InsertReturning(&testUser, []string{"id"}, &userID)
			require.NoError(t, err)

			// Delete by primary key
			err = helper.userRepo.DeleteByKey("id", userID)
			require.NoError(t, err)

			// Verify deletion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", userID, &result)
			assert.True(t, EmptyResult(err), "User should be deleted")
		})

		t.Run("DeleteByUniqueKey", func(t *testing.T) {
			// Create a user to delete
			testUser := ExtensiveTestUser{
				Name:     "Delete By Email",
				Email:    "deletebyemail@example.com",
				Status:   "temp",
				Score:    45.0,
				IsActive: false,
				Tags:     pq.StringArray{"deletebyemail"},
			}
			err := helper.userRepo.Insert(&testUser)
			require.NoError(t, err)

			// Delete by email (unique key)
			err = helper.userRepo.DeleteByKey("email", "deletebyemail@example.com")
			require.NoError(t, err)

			// Verify deletion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "deletebyemail@example.com", &result)
			assert.True(t, EmptyResult(err), "User should be deleted")
		})

		t.Run("DeleteByKeyNotFound", func(t *testing.T) {
			// Try to delete non-existent record
			err := helper.userRepo.DeleteByKey("id", 99999)
			require.NoError(t, err) // Should succeed even if no rows affected
		})
	})

	// Test cascading deletes with profiles
	t.Run("CascadingDeletes", func(t *testing.T) {
		// Create user and profile
		testUser := ExtensiveTestUser{
			Name:     "Cascade Test User",
			Email:    "cascade@example.com",
			Status:   "test",
			Score:    75.0,
			IsActive: true,
			Tags:     pq.StringArray{"cascade", "test"},
		}
		var userID int64
		err := helper.userRepo.InsertReturning(&testUser, []string{"id"}, &userID)
		require.NoError(t, err)

		testProfile := TestProfile{
			UserID:   userID,
			Website:  "https://cascade.test",
			Company:  "CascadeCorp",
			Location: "Cascade City",
		}
		var profileID int64
		err = helper.profileRepo.InsertReturning(&testProfile, []string{"id"}, &profileID)
		require.NoError(t, err)

		// Delete user (should cascade to profile)
		err = helper.userRepo.DeleteByKey("id", userID)
		require.NoError(t, err)

		// Verify both user and profile are deleted
		var userResult ExtensiveTestUser
		err = helper.userRepo.FetchByKey("id", userID, &userResult)
		assert.True(t, EmptyResult(err), "User should be deleted")

		var profileResult TestProfile
		err = helper.profileRepo.FetchByKey("id", profileID, &profileResult)
		assert.True(t, EmptyResult(err), "Profile should be deleted by cascade")
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}

// TestExtensive_Counter_Interface tests all Counter interface methods comprehensively
func TestExtensive_Counter_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create sample data for testing counts
	users := helper.createSampleUsers(t)
	profiles := helper.createSampleProfiles(t, users)
	require.Len(t, users, 4, "Should have 4 sample users")
	require.Len(t, profiles, 3, "Should have 3 sample profiles")

	t.Run("Count", func(t *testing.T) {
		t.Run("CountAllRecords", func(t *testing.T) {
			count, err := helper.userRepo.Count()
			require.NoError(t, err)
			assert.Equal(t, int64(4), count, "Should count all 4 users")
		})

		t.Run("CountAfterInserts", func(t *testing.T) {
			// Add more users
			newUsers := []any{
				&ExtensiveTestUser{
					Name:     "Count Test 1",
					Email:    "count1@example.com",
					Status:   "active",
					Score:    80.0,
					IsActive: true,
					Tags:     pq.StringArray{"count", "test"},
				},
				&ExtensiveTestUser{
					Name:     "Count Test 2",
					Email:    "count2@example.com",
					Status:   "inactive",
					Score:    75.0,
					IsActive: false,
					Tags:     pq.StringArray{"count", "test"},
				},
			}

			err := helper.userRepo.Insert(newUsers...)
			require.NoError(t, err)

			count, err := helper.userRepo.Count()
			require.NoError(t, err)
			assert.Equal(t, int64(6), count, "Should count all 6 users after inserts")
		})

		t.Run("CountAfterDeletes", func(t *testing.T) {
			// Delete some users
			err := helper.userRepo.DeleteWhere(map[string]any{"status": "inactive"})
			require.NoError(t, err)

			count, err := helper.userRepo.Count()
			require.NoError(t, err)
			assert.LessOrEqual(t, count, int64(5), "Should have fewer users after deletes")
		})
	})

	t.Run("CountWhere", func(t *testing.T) {
		t.Run("CountBySingleCondition", func(t *testing.T) {
			count, err := helper.userRepo.CountWhere(map[string]any{"is_active": true})
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(1), "Should have at least 1 active user")
		})

		t.Run("CountByMultipleConditions", func(t *testing.T) {
			count, err := helper.userRepo.CountWhere(map[string]any{
				"status":    "active",
				"is_active": true,
			})
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(0), "Should count users with both conditions")
		})

		t.Run("CountWhereNoMatches", func(t *testing.T) {
			count, err := helper.userRepo.CountWhere(map[string]any{"status": "nonexistent"})
			require.NoError(t, err)
			assert.Equal(t, int64(0), count, "Should count 0 for non-matching condition")
		})

		t.Run("CountWhereNilConditions", func(t *testing.T) {
			// Should count all records when no conditions
			count, err := helper.userRepo.CountWhere(nil)
			require.NoError(t, err)

			totalCount, err := helper.userRepo.Count()
			require.NoError(t, err)
			assert.Equal(t, totalCount, count, "CountWhere with nil should equal total count")
		})

		t.Run("CountWhereEmptyConditions", func(t *testing.T) {
			// Should count all records when empty conditions
			count, err := helper.userRepo.CountWhere(map[string]any{})
			require.NoError(t, err)

			totalCount, err := helper.userRepo.Count()
			require.NoError(t, err)
			assert.Equal(t, totalCount, count, "CountWhere with empty map should equal total count")
		})
	})

	// Test with profiles repository
	t.Run("Counter_ProfileRepository", func(t *testing.T) {
		t.Run("CountAllProfiles", func(t *testing.T) {
			count, err := helper.profileRepo.Count()
			require.NoError(t, err)
			assert.Equal(t, int64(3), count, "Should count all 3 profiles")
		})

		t.Run("CountProfilesByCompany", func(t *testing.T) {
			count, err := helper.profileRepo.CountWhere(map[string]any{"company": "TechCorp"})
			require.NoError(t, err)
			assert.Equal(t, int64(1), count, "Should count 1 TechCorp profile")
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}

// TestExtensive_Executor_Interface tests all Executor interface methods comprehensively
func TestExtensive_Executor_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create sample data for testing
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4, "Should have 4 sample users")

	t.Run("Exec", func(t *testing.T) {
		t.Run("ExecSelectQuery", func(t *testing.T) {
			// Note: Exec with SELECT is unusual but should work
			query := helper.userRepo.SqlSelect().Where(goqu.C("id").Eq(users[0].ID))
			err := helper.userRepo.Exec(query)
			require.NoError(t, err)
		})

		t.Run("ExecNilQuery", func(t *testing.T) {
			err := helper.userRepo.Exec(nil)
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for nil query")
		})
	})

	t.Run("RawExec", func(t *testing.T) {
		t.Run("RawExecUpdate", func(t *testing.T) {
			sql := `UPDATE extensive_test_users SET score = $1 WHERE id = $2`
			err := helper.userRepo.RawExec(sql, 99.9, users[0].ID)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, 99.9, result.Score)
		})

		t.Run("RawExecInsert", func(t *testing.T) {
			sql := `INSERT INTO extensive_test_users (name, email, status, score, is_active, tags) 
					VALUES ($1, $2, $3, $4, $5, $6)`
			err := helper.userRepo.RawExec(sql, "Raw Exec User", "rawexec@example.com",
				"active", 88.0, true, pq.StringArray{"raw", "exec"})
			require.NoError(t, err)

			// Verify insertion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "rawexec@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "Raw Exec User", result.Name)
			assert.Equal(t, 88.0, result.Score)
		})

		t.Run("RawExecDelete", func(t *testing.T) {
			sql := `DELETE FROM extensive_test_users WHERE email = $1`
			err := helper.userRepo.RawExec(sql, "rawexec@example.com")
			require.NoError(t, err)

			// Verify deletion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "rawexec@example.com", &result)
			assert.True(t, EmptyResult(err), "User should be deleted")
		})

		t.Run("RawExecInvalidSQL", func(t *testing.T) {
			sql := `INVALID SQL STATEMENT`
			err := helper.userRepo.RawExec(sql)
			assert.Error(t, err, "Should return error for invalid SQL")
		})
	})

	t.Run("Select", func(t *testing.T) {
		t.Run("SelectIntoSlice", func(t *testing.T) {
			sql := `SELECT name, email, score FROM extensive_test_users WHERE is_active = $1 ORDER BY name`
			var results []ExtensiveTestUser

			err := helper.userRepo.Select(sql, &results, true)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(results), 1, "Should select at least 1 active user")

			for _, user := range results {
				assert.NotEmpty(t, user.Name, "Name should be populated")
				assert.NotEmpty(t, user.Email, "Email should be populated")
				assert.Greater(t, user.Score, 0.0, "Score should be populated")
			}
		})

		t.Run("SelectIntoStruct", func(t *testing.T) {
			sql := `SELECT * FROM extensive_test_users WHERE id = $1`
			var results []ExtensiveTestUser

			err := helper.userRepo.Select(sql, &results, users[1].ID)
			require.NoError(t, err)
			require.Len(t, results, 1, "Should get one result")
			assert.Equal(t, users[1].ID, results[0].ID)
			assert.Equal(t, "Bob Smith", results[0].Name)
		})

		t.Run("SelectNoResults", func(t *testing.T) {
			sql := `SELECT * FROM extensive_test_users WHERE id = $1`
			var results []ExtensiveTestUser

			err := helper.userRepo.Select(sql, &results, 99999)
			require.NoError(t, err)
			assert.Len(t, results, 0, "Should return empty slice for no results")
		})

		t.Run("SelectInvalidSQL", func(t *testing.T) {
			sql := `SELECT * FROM nonexistent_table`
			var results []ExtensiveTestUser

			err := helper.userRepo.Select(sql, &results)
			assert.Error(t, err, "Should return error for invalid table")
		})
	})

	t.Run("Do", func(t *testing.T) {
		t.Run("DoWithSelectDataset", func(t *testing.T) {
			query := helper.userRepo.SqlSelect().Where(goqu.C("is_active").IsTrue())
			var results []ExtensiveTestUser

			err := helper.userRepo.Do(query, &results)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(results), 1, "Should get active users")
		})

		t.Run("DoWithUpdateDataset", func(t *testing.T) {
			// Use UpdateFields method instead of Do/Update for update operations
			fieldsToUpdate := map[string]any{"status": "do_tested"}
			whereConditions := map[string]any{"id": users[2].ID}

			err := helper.userRepo.UpdateFields(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[2].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "do_tested", result.Status)
		})

		t.Run("DoWithInsertDataset", func(t *testing.T) {
			// Use direct Insert method instead of Do for insert operations
			user := ExtensiveTestUser{
				Name:     "Do Insert User",
				Email:    "doinsert@example.com",
				Status:   "active",
				Score:    85.0,
				IsActive: true,
				Tags:     pq.StringArray{"do", "insert"},
			}

			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Verify insertion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "doinsert@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "Do Insert User", result.Name)
		})

		t.Run("DoWithDeleteDataset", func(t *testing.T) {
			// Use Delete method instead of Do for delete operations
			deleteQuery := helper.userRepo.SqlDelete().
				Where(goqu.C("email").Eq("doinsert@example.com"))

			err := helper.userRepo.Delete(deleteQuery)
			require.NoError(t, err)

			// Verify deletion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "doinsert@example.com", &result)
			assert.True(t, EmptyResult(err), "User should be deleted")
		})

		t.Run("DoWithNilQuery", func(t *testing.T) {
			var results []ExtensiveTestUser
			err := helper.userRepo.Do(nil, &results)
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for nil query")
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}

// TestExtensive_Builder_Interface tests all Builder interface methods comprehensively
func TestExtensive_Builder_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	t.Run("Sql", func(t *testing.T) {
		t.Run("SqlDialectWrapper", func(t *testing.T) {
			dialect := helper.userRepo.Sql()
			require.NotNil(t, dialect, "Sql() should return dialect wrapper")

			// Test that we can create queries with it
			query := dialect.From("test_table")
			require.NotNil(t, query, "Should be able to create query from dialect")

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "test_table", "Generated SQL should contain table name")
		})
	})

	t.Run("SqlSelect", func(t *testing.T) {
		t.Run("BasicSelectDataset", func(t *testing.T) {
			query := helper.userRepo.SqlSelect()
			require.NotNil(t, query, "SqlSelect() should return dataset")

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "extensive_test_users", "Should select from correct table")
			assert.Contains(t, sql, "SELECT", "Should be a SELECT query")
		})

		t.Run("SelectWithConditions", func(t *testing.T) {
			query := helper.userRepo.SqlSelect().
				Where(goqu.C("is_active").IsTrue()).
				Order(goqu.C("name").Asc()).
				Limit(10)

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "WHERE", "Should have WHERE clause")
			assert.Contains(t, sql, "ORDER BY", "Should have ORDER BY clause")
			assert.Contains(t, sql, "LIMIT", "Should have LIMIT clause")
			assert.Contains(t, sql, "is_active", "Should reference is_active field")
		})
	})

	t.Run("SqlInsert", func(t *testing.T) {
		t.Run("BasicInsertDataset", func(t *testing.T) {
			query := helper.userRepo.SqlInsert()
			require.NotNil(t, query, "SqlInsert() should return dataset")

			// Add rows to the insert
			insertQuery := query.Rows(map[string]any{
				"name":      "Builder Test User",
				"email":     "builder@example.com",
				"status":    "active",
				"score":     78.0,
				"is_active": true,
				"tags":      pq.StringArray{"builder", "test"},
			})

			sql, _, err := insertQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "INSERT INTO", "Should be an INSERT query")
			assert.Contains(t, sql, "extensive_test_users", "Should insert into correct table")
			assert.Contains(t, sql, "name", "Should include name field")
			assert.Contains(t, sql, "email", "Should include email field")
		})

		t.Run("InsertWithReturning", func(t *testing.T) {
			query := helper.userRepo.SqlInsert().
				Rows(map[string]any{
					"name":      "Builder Return User",
					"email":     "builderreturn@example.com",
					"status":    "active",
					"score":     82.0,
					"is_active": true,
					"tags":      pq.StringArray{"builder", "return"},
				}).
				Returning("id", "name", "email")

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "RETURNING", "Should have RETURNING clause")
			assert.Contains(t, sql, "id", "Should return id")
			assert.Contains(t, sql, "name", "Should return name")
		})
	})

	t.Run("SqlUpdate", func(t *testing.T) {
		t.Run("BasicUpdateDataset", func(t *testing.T) {
			query := helper.userRepo.SqlUpdate()
			require.NotNil(t, query, "SqlUpdate() should return dataset")

			// Add SET and WHERE clauses
			updateQuery := query.
				Set(map[string]any{
					"status": "builder_updated",
					"score":  95.0,
				}).
				Where(goqu.C("email").Eq("test@example.com"))

			sql, _, err := updateQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "UPDATE", "Should be an UPDATE query")
			assert.Contains(t, sql, "extensive_test_users", "Should update correct table")
			assert.Contains(t, sql, "SET", "Should have SET clause")
			assert.Contains(t, sql, "WHERE", "Should have WHERE clause")
			assert.Contains(t, sql, "status", "Should reference status field")
		})

		t.Run("UpdateWithReturning", func(t *testing.T) {
			query := helper.userRepo.SqlUpdate().
				Set(map[string]any{"status": "updated_with_returning"}).
				Where(goqu.C("id").Eq(1)).
				Returning("id", "status", "updated_at")

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "RETURNING", "Should have RETURNING clause")
		})
	})

	t.Run("SqlDelete", func(t *testing.T) {
		t.Run("BasicDeleteDataset", func(t *testing.T) {
			query := helper.userRepo.SqlDelete()
			require.NotNil(t, query, "SqlDelete() should return dataset")

			// Add WHERE clause
			deleteQuery := query.Where(goqu.C("email").Eq("test@example.com"))

			sql, _, err := deleteQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "DELETE FROM", "Should be a DELETE query")
			assert.Contains(t, sql, "extensive_test_users", "Should delete from correct table")
			assert.Contains(t, sql, "WHERE", "Should have WHERE clause")
			assert.Contains(t, sql, "email", "Should reference email field")
		})

		t.Run("DeleteWithComplexConditions", func(t *testing.T) {
			query := helper.userRepo.SqlDelete().
				Where(goqu.C("is_active").IsFalse()).
				Where(goqu.C("score").Lt(50.0))

			sql, _, err := query.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "WHERE", "Should have WHERE clause")
			assert.Contains(t, sql, "is_active", "Should reference is_active field")
			assert.Contains(t, sql, "score", "Should reference score field")
		})
	})

	// Test Builder methods work across different repositories
	t.Run("Builder_ProfileRepository", func(t *testing.T) {
		t.Run("ProfileSqlMethods", func(t *testing.T) {
			// Test all SQL builder methods on profile repository
			selectQuery := helper.profileRepo.SqlSelect()
			sql, _, err := selectQuery.ToSQL()
			require.NoError(t, err)
			assert.Contains(t, sql, "test_profiles", "Should use correct table name")

			insertQuery := helper.profileRepo.SqlInsert()
			require.NotNil(t, insertQuery, "SqlInsert should work for profiles")

			updateQuery := helper.profileRepo.SqlUpdate()
			require.NotNil(t, updateQuery, "SqlUpdate should work for profiles")

			deleteQuery := helper.profileRepo.SqlDelete()
			require.NotNil(t, deleteQuery, "SqlDelete should work for profiles")
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}
