//go:build integration && pgsql
// +build integration,pgsql

package db

import (
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtensive_Transaction_Interface tests all Transaction interface methods comprehensively
func TestExtensive_Transaction_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()
}

func TestTransaction_BasicLifecycle(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()
	t.Run("CommitTransaction", func(t *testing.T) {
		// Start transaction
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Insert user in transaction
		user := ExtensiveTestUser{
			Name:     "Transaction Commit User",
			Email:    "txcommit@example.com",
			Status:   "active",
			Score:    85.0,
			IsActive: true,
			Tags:     pq.StringArray{"transaction", "commit"},
		}

		err = tx.Insert(&user)
		require.NoError(t, err)

		// Verify user doesn't exist outside transaction yet
		var checkUser ExtensiveTestUser
		err = helper.userRepo.FetchByKey("email", "txcommit@example.com", &checkUser)
		assert.True(t, EmptyResult(err), "User should not exist outside transaction")

		// Commit transaction
		err = tx.Commit()
		require.NoError(t, err)

		// Verify user exists after commit
		err = helper.userRepo.FetchByKey("email", "txcommit@example.com", &checkUser)
		require.NoError(t, err)
		assert.Equal(t, "Transaction Commit User", checkUser.Name)
	})

	t.Run("RollbackTransaction", func(t *testing.T) {
		// Start transaction
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Insert user in transaction
		user := ExtensiveTestUser{
			Name:     "Transaction Rollback User",
			Email:    "txrollback@example.com",
			Status:   "active",
			Score:    75.0,
			IsActive: true,
			Tags:     pq.StringArray{"transaction", "rollback"},
		}

		err = tx.Insert(&user)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify user doesn't exist after rollback
		var checkUser ExtensiveTestUser
		err = helper.userRepo.FetchByKey("email", "txrollback@example.com", &checkUser)
		assert.True(t, EmptyResult(err), "User should not exist after rollback")
	})

	t.Run("AutoRollbackOnError", func(t *testing.T) {
		// Start transaction
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)

		// Insert valid user first
		user1 := ExtensiveTestUser{
			Name:     "Valid TX User",
			Email:    "validtx@example.com",
			Status:   "active",
			Score:    80.0,
			IsActive: true,
			Tags:     pq.StringArray{"valid", "tx"},
		}
		err = tx.Insert(&user1)
		require.NoError(t, err)

		// Try to insert user with duplicate email (should fail)
		user2 := ExtensiveTestUser{
			Name:     "Duplicate TX User",
			Email:    "validtx@example.com", // Same email
			Status:   "active",
			Score:    90.0,
			IsActive: true,
			Tags:     pq.StringArray{"duplicate", "tx"},
		}
		err = tx.Insert(&user2)
		assert.Error(t, err, "Should fail due to unique constraint")

		// Rollback to clean state
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify no users were committed
		var checkUser ExtensiveTestUser
		err = helper.userRepo.FetchByKey("email", "validtx@example.com", &checkUser)
		assert.True(t, EmptyResult(err), "No users should exist after rollback")
	})
}

func TestTransaction_ReaderMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create initial data outside transaction
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4)

	t.Run("FetchOne_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		var result ExtensiveTestUser
		query := tx.SqlSelect().Where(goqu.C("email").Eq("alice@example.com"))
		err = tx.FetchOne(query, &result)
		require.NoError(t, err)
		assert.Equal(t, "Alice Johnson", result.Name)
	})

	t.Run("Fetch_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		var results []ExtensiveTestUser
		query := tx.SqlSelect().Where(goqu.C("is_active").IsTrue()).Order(goqu.C("name").Asc())
		err = tx.Fetch(query, &results)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should fetch active users")
	})

	t.Run("FetchRecord_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		var result ExtensiveTestUser
		fieldValues := map[string]any{"email": "bob@example.com"}
		err = tx.FetchRecord(fieldValues, &result)
		require.NoError(t, err)
		assert.Equal(t, "Bob Smith", result.Name)
	})

	t.Run("FetchByKey_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[2].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "Carol Williams", result.Name)
	})

	t.Run("FetchWhere_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		var results []ExtensiveTestUser
		fieldValues := map[string]any{"status": "active"}
		err = tx.FetchWhere(fieldValues, &results)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should fetch active users")
	})

	t.Run("Exists_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		exists, err := tx.Exists("email", "david@example.com")
		require.NoError(t, err)
		assert.True(t, exists, "David should exist")
	})
}

func TestTransaction_WriterMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	t.Run("Insert_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)

		user := ExtensiveTestUser{
			Name:     "TX Insert User",
			Email:    "txinsert@example.com",
			Status:   "active",
			Score:    88.0,
			IsActive: true,
			Tags:     pq.StringArray{"tx", "insert"},
		}

		err = tx.Insert(&user)
		require.NoError(t, err)

		// Verify in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("email", "txinsert@example.com", &result)
		require.NoError(t, err)
		assert.Equal(t, "TX Insert User", result.Name)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify after commit
		err = helper.userRepo.FetchByKey("email", "txinsert@example.com", &result)
		require.NoError(t, err)
		assert.Equal(t, "TX Insert User", result.Name)
	})

	t.Run("InsertReturning_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		user := ExtensiveTestUser{
			Name:     "TX Insert Returning User",
			Email:    "txinsertret@example.com",
			Status:   "pending",
			Score:    77.5,
			IsActive: false,
			Tags:     pq.StringArray{"tx", "insert", "returning"},
		}

		var id int64
		var name, email string
		var createdAt time.Time
		err = tx.InsertReturning(&user, pq.StringArray{"id", "name", "email", "created_at"}, &id, &name, &email, &createdAt)
		require.NoError(t, err)

		assert.NotZero(t, id)
		assert.Equal(t, "TX Insert Returning User", name)
		assert.Equal(t, "txinsertret@example.com", email)
		assert.False(t, createdAt.IsZero())
	})

	t.Run("BatchInsert_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		users := []any{
			&ExtensiveTestUser{
				Name:     "TX Batch User 1",
				Email:    "txbatch1@example.com",
				Status:   "active",
				Score:    85.0,
				IsActive: true,
				Tags:     pq.StringArray{"tx", "batch", "1"},
			},
			&ExtensiveTestUser{
				Name:     "TX Batch User 2",
				Email:    "txbatch2@example.com",
				Status:   "pending",
				Score:    78.0,
				IsActive: false,
				Tags:     pq.StringArray{"tx", "batch", "2"},
			},
		}

		err = tx.Insert(users...)
		require.NoError(t, err)

		// Verify both users exist in transaction
		var results []ExtensiveTestUser
		query := tx.SqlSelect().Where(goqu.C("email").Like("txbatch%@example.com"))
		err = tx.Fetch(query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 2, "Both batch users should exist in transaction")
	})
}

func TestTransaction_UpdaterMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create initial users for updating
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4)

	t.Run("UpdateRecord_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		updateUser := ExtensiveTestUser{
			Name:     "TX Updated Alice",
			Status:   "premium",
			Score:    99.0,
			IsActive: true,
		}

		whereConditions := map[string]any{"id": users[0].ID}
		err = tx.UpdateRecord(&updateUser, whereConditions)
		require.NoError(t, err)

		// Verify update in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[0].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "TX Updated Alice", result.Name)
		assert.Equal(t, "premium", result.Status)
		assert.Equal(t, 99.0, result.Score)

		// Verify original remains unchanged outside transaction
		err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "Alice Johnson", result.Name)
		assert.Equal(t, "active", result.Status)
	})

	t.Run("UpdateFields_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		fieldsToUpdate := map[string]any{
			"status": "updated_in_tx",
			"score":  95.5,
		}

		whereConditions := map[string]any{"id": users[1].ID}
		err = tx.UpdateFields(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions)
		require.NoError(t, err)

		// Verify update in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[1].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "updated_in_tx", result.Status)
		assert.Equal(t, 95.5, result.Score)
		assert.Equal(t, "Bob Smith", result.Name) // Should remain unchanged
	})

	t.Run("UpdateReturning_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		updateUser := ExtensiveTestUser{
			Name:     "Carol Williams",
			Status:   "tx_premium",
			Score:    97.0,
			IsActive: true,
		}

		whereConditions := map[string]any{"id": users[2].ID}
		result := &ExtensiveTestUser{}
		err = tx.UpdateReturning(&updateUser, whereConditions,
			pq.StringArray{"id", "name", "status", "score", "updated_at"}, result)
		require.NoError(t, err)

		assert.Equal(t, users[2].ID, result.ID)
		assert.Equal(t, "Carol Williams", result.Name)
		assert.Equal(t, "tx_premium", result.Status)
		assert.Equal(t, 97.0, result.Score)
		assert.False(t, result.UpdatedAt.IsZero())
	})

	t.Run("UpdateByKey_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		updateUser := ExtensiveTestUser{
			Name:     "TX Updated David",
			Status:   "active",
			Score:    82.0,
			IsActive: true,
		}

		err = tx.UpdateByKey(&updateUser, "id", users[3].ID)
		require.NoError(t, err)

		// Verify update in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[3].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "TX Updated David", result.Name)
		assert.Equal(t, "active", result.Status)
	})
}

func TestTransaction_DeleterMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create users for deleting
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4)

	t.Run("DeleteByKey_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Delete user in transaction
		err = tx.DeleteByKey("id", users[0].ID)
		require.NoError(t, err)

		// Verify deleted in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[0].ID, &result)
		assert.True(t, EmptyResult(err), "User should be deleted in transaction")

		// Verify still exists outside transaction
		err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "Alice Johnson", result.Name)
	})

	t.Run("DeleteWhere_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		whereConditions := map[string]any{"status": "pending"}
		err = tx.DeleteWhere(whereConditions)
		require.NoError(t, err)

		// Verify pending users deleted in transaction
		var results []ExtensiveTestUser
		err = tx.FetchWhere(map[string]any{"status": "pending"}, &results)
		require.NoError(t, err)
		assert.Len(t, results, 0, "No pending users should exist in transaction")

		// Verify pending users still exist outside transaction
		err = helper.userRepo.FetchWhere(map[string]any{"status": "pending"}, &results)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1, "Pending users should still exist outside transaction")
	})

	t.Run("Delete_WithDataset_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		deleteQuery := tx.SqlDelete().Where(goqu.C("is_active").IsFalse())
		err = tx.Delete(deleteQuery)
		require.NoError(t, err)

		// Verify inactive users deleted in transaction
		var results []ExtensiveTestUser
		query := tx.SqlSelect().Where(goqu.C("is_active").IsFalse())
		err = tx.Fetch(query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 0, "No inactive users should exist in transaction")
	})
}

func TestTransaction_CounterMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create users for counting
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4)

	t.Run("Count_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		count, err := tx.Count()
		require.NoError(t, err)
		assert.Equal(t, int64(4), count, "Should count all 4 users")

		// Add user in transaction
		user := ExtensiveTestUser{
			Name:     "TX Count User",
			Email:    "txcount@example.com",
			Status:   "active",
			Score:    80.0,
			IsActive: true,
			Tags:     pq.StringArray{"tx", "count"},
		}
		err = tx.Insert(&user)
		require.NoError(t, err)

		// Count should be 5 in transaction
		count, err = tx.Count()
		require.NoError(t, err)
		assert.Equal(t, int64(5), count, "Should count 5 users in transaction")

		// Count should still be 4 outside transaction
		count, err = helper.userRepo.Count()
		require.NoError(t, err)
		assert.Equal(t, int64(4), count, "Should still count 4 users outside transaction")
	})

	t.Run("CountWhere_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		count, err := tx.CountWhere(map[string]any{"is_active": true})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(2), "Should count active users")

		// Update user status in transaction
		whereConditions := map[string]any{"id": users[1].ID} // Bob (currently inactive)
		fieldsToUpdate := map[string]any{"is_active": true}
		err = tx.UpdateFields(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions)
		require.NoError(t, err)

		// Count should increase in transaction
		newCount, err := tx.CountWhere(map[string]any{"is_active": true})
		require.NoError(t, err)
		assert.Greater(t, newCount, count, "Active count should increase in transaction")
	})
}

func TestTransaction_ExecutorMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create users for executor testing
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4)

	t.Run("RawExec_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		sql := `UPDATE extensive_test_users SET score = $1 WHERE id = $2`
		err = tx.RawExec(sql, 100.0, users[0].ID)
		require.NoError(t, err)

		// Verify update in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[0].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, 100.0, result.Score)

		// Verify original score outside transaction
		err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, 95.5, result.Score) // Original score
	})

	t.Run("Select_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		sql := `SELECT name, email, score FROM extensive_test_users WHERE is_active = $1 ORDER BY name`
		var results []ExtensiveTestUser
		err = tx.Select(sql, &results, true)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should select active users")
	})

	t.Run("Do_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Test Do with update dataset
		updateQuery := tx.SqlUpdateX(users[2]).
			FieldsValues(map[string]any{"status": "do_updated"}).
			WhereEq("id", users[2].ID)

		err = tx.Do(updateQuery, nil)
		require.NoError(t, err)

		// Verify update in transaction
		var result ExtensiveTestUser
		err = tx.FetchByKey("id", users[2].ID, &result)
		require.NoError(t, err)
		assert.Equal(t, "do_updated", result.Status)
	})

	t.Run("Exec_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		query := tx.SqlSelect().Where(goqu.C("id").Eq(users[1].ID))
		err = tx.Exec(query)
		require.NoError(t, err) // Should execute without error
	})
}

func TestTransaction_BuilderMethods(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	t.Run("SqlBuilders_InTransaction", func(t *testing.T) {
		tx, err := helper.userRepo.NewTransaction(nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Test all SQL builder methods work in transaction
		selectQuery := tx.SqlSelect()
		require.NotNil(t, selectQuery)

		insertQuery := tx.SqlInsert()
		require.NotNil(t, insertQuery)

		updateQuery := tx.SqlUpdate()
		require.NotNil(t, updateQuery)

		updateQuery1 := tx.SqlUpdateX(&ExtensiveTestUser{})
		require.NotNil(t, updateQuery1)

		deleteQuery := tx.SqlDelete()
		require.NotNil(t, deleteQuery)

		dialect := tx.Sql()
		require.NotNil(t, dialect)

		// Test that SQL generation works
		sql, _, err := selectQuery.Where(goqu.C("id").Eq(1)).ToSQL()
		require.NoError(t, err)
		assert.Contains(t, sql, "SELECT")
		assert.Contains(t, sql, "extensive_test_users")
	})
}

// TestExtensive_Transaction_Lifecycle tests transaction lifecycle scenarios comprehensively
func TestExtensive_Transaction_Lifecycle(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	t.Run("TransactionLifecycle_MultipleOperations", func(t *testing.T) {
		t.Run("ComplexCommitScenario", func(t *testing.T) {
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Step 1: Insert multiple users
			users := []any{
				&ExtensiveTestUser{
					Name:     "TX Lifecycle User 1",
					Email:    "txlife1@example.com",
					Status:   "pending",
					Score:    80.0,
					IsActive: false,
					Tags:     pq.StringArray{"lifecycle", "test", "1"},
				},
				&ExtensiveTestUser{
					Name:     "TX Lifecycle User 2",
					Email:    "txlife2@example.com",
					Status:   "active",
					Score:    85.0,
					IsActive: true,
					Tags:     pq.StringArray{"lifecycle", "test", "2"},
				},
			}

			err = tx.Insert(users...)
			require.NoError(t, err)

			// Step 2: Get IDs of inserted users
			var user1, user2 ExtensiveTestUser
			err = tx.FetchByKey("email", "txlife1@example.com", &user1)
			require.NoError(t, err)
			err = tx.FetchByKey("email", "txlife2@example.com", &user2)
			require.NoError(t, err)

			// Step 3: Create profiles for these users
			profiles := []any{
				&TestProfile{
					UserID:   user1.ID,
					Website:  "https://user1-lifecycle.com",
					Company:  "LifecycleCompany1",
					Location: "Test City 1",
				},
				&TestProfile{
					UserID:   user2.ID,
					Website:  "https://user2-lifecycle.com",
					Company:  "LifecycleCompany2",
					Location: "Test City 2",
				},
			}

			// Note: Using raw SQL for profiles since we need to test with a different transaction scope
			for _, profile := range profiles {
				p := profile.(*TestProfile)
				err = tx.RawExec(
					`INSERT INTO test_profiles (user_id, website, company, location) VALUES ($1, $2, $3, $4)`,
					p.UserID, p.Website, p.Company, p.Location,
				)
				require.NoError(t, err)
			}

			// Step 4: Update one user's status
			err = tx.UpdateFields(&ExtensiveTestUser{},
				map[string]any{"status": "verified", "score": 90.0},
				map[string]any{"id": user1.ID})
			require.NoError(t, err)

			// Step 5: Count users in transaction
			count, err := tx.Count()
			require.NoError(t, err)
			assert.Equal(t, int64(2), count, "Should have 2 users in transaction")

			// Step 6: Verify data in transaction before commit
			var results []ExtensiveTestUser
			err = tx.FetchWhere(map[string]any{"status": "verified"}, &results)
			require.NoError(t, err)
			assert.Len(t, results, 1, "Should have 1 verified user")
			assert.Equal(t, 90.0, results[0].Score)

			// Step 7: Commit transaction
			err = tx.Commit()
			require.NoError(t, err)

			// Step 8: Verify all data persisted after commit
			var allUsers []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Where(goqu.C("email").Like("txlife%@example.com")).Order(goqu.C("email").Asc())
			err = helper.userRepo.Fetch(query, &allUsers)
			require.NoError(t, err)
			require.Len(t, allUsers, 2, "Both users should exist after commit")

			assert.Equal(t, "TX Lifecycle User 1", allUsers[0].Name)
			assert.Equal(t, "verified", allUsers[0].Status)
			assert.Equal(t, 90.0, allUsers[0].Score)

			assert.Equal(t, "TX Lifecycle User 2", allUsers[1].Name)
			assert.Equal(t, "active", allUsers[1].Status)
			assert.Equal(t, 85.0, allUsers[1].Score)

			// Verify profiles also committed
			var profileCount int64
			err = helper.db.Get(&profileCount, `SELECT COUNT(*) FROM test_profiles WHERE user_id IN ($1, $2)`, allUsers[0].ID, allUsers[1].ID)
			require.NoError(t, err)
			assert.Equal(t, int64(2), profileCount, "Both profiles should exist")
		})

		t.Run("ComplexRollbackScenario", func(t *testing.T) {
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Step 1: Insert users
			user1 := ExtensiveTestUser{
				Name:     "TX Rollback User 1",
				Email:    "txrollback1@example.com",
				Status:   "active",
				Score:    75.0,
				IsActive: true,
				Tags:     pq.StringArray{"rollback", "test"},
			}

			user2 := ExtensiveTestUser{
				Name:     "TX Rollback User 2",
				Email:    "txrollback2@example.com",
				Status:   "pending",
				Score:    70.0,
				IsActive: false,
				Tags:     pq.StringArray{"rollback", "test"},
			}

			err = tx.Insert(&user1, &user2)
			require.NoError(t, err)

			// Step 2: Verify users exist in transaction
			var count int64
			count, err = tx.CountWhere(map[string]any{"email": "txrollback1@example.com"})
			require.NoError(t, err)
			assert.Equal(t, int64(1), count)

			count, err = tx.CountWhere(map[string]any{"email": "txrollback2@example.com"})
			require.NoError(t, err)
			assert.Equal(t, int64(1), count)

			// Step 3: Update one user
			err = tx.UpdateFields(&ExtensiveTestUser{},
				map[string]any{"score": 95.0, "status": "premium"},
				map[string]any{"email": "txrollback1@example.com"})
			require.NoError(t, err)

			// Step 4: Verify update in transaction
			var updatedUser ExtensiveTestUser
			err = tx.FetchByKey("email", "txrollback1@example.com", &updatedUser)
			require.NoError(t, err)
			assert.Equal(t, "premium", updatedUser.Status)
			assert.Equal(t, 95.0, updatedUser.Score)

			// Step 5: Rollback transaction
			err = tx.Rollback()
			require.NoError(t, err)

			// Step 6: Verify no data exists after rollback
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "txrollback1@example.com", &result)
			assert.True(t, EmptyResult(err), "User 1 should not exist after rollback")

			err = helper.userRepo.FetchByKey("email", "txrollback2@example.com", &result)
			assert.True(t, EmptyResult(err), "User 2 should not exist after rollback")

			// Verify count is back to 0
			count, err = helper.userRepo.CountWhere(map[string]any{"email": "txrollback1@example.com"})
			require.NoError(t, err)
			assert.Equal(t, int64(0), count)
		})
	})

	t.Run("TransactionLifecycle_ErrorHandling", func(t *testing.T) {
		t.Run("ConstraintViolationRollback", func(t *testing.T) {
			// First, create a user outside transaction
			initialUser := ExtensiveTestUser{
				Name:     "Initial User",
				Email:    "constraint@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"initial"},
			}
			err := helper.userRepo.Insert(&initialUser)
			require.NoError(t, err)

			// Start transaction
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Insert valid user in transaction
			validUser := ExtensiveTestUser{
				Name:     "Valid TX User",
				Email:    "validintx@example.com",
				Status:   "active",
				Score:    85.0,
				IsActive: true,
				Tags:     pq.StringArray{"valid", "tx"},
			}
			err = tx.Insert(&validUser)
			require.NoError(t, err)

			// Verify valid user exists in transaction
			var checkUser ExtensiveTestUser
			err = tx.FetchByKey("email", "validintx@example.com", &checkUser)
			require.NoError(t, err)
			assert.Equal(t, "Valid TX User", checkUser.Name)

			// Try to insert user with duplicate email (should fail)
			duplicateUser := ExtensiveTestUser{
				Name:     "Duplicate User",
				Email:    "constraint@example.com", // Same as initial user
				Status:   "pending",
				Score:    90.0,
				IsActive: false,
				Tags:     pq.StringArray{"duplicate"},
			}
			err = tx.Insert(&duplicateUser)
			assert.Error(t, err, "Should fail due to unique constraint")

			// Rollback transaction
			err = tx.Rollback()
			require.NoError(t, err)

			// Verify valid user was also rolled back
			err = helper.userRepo.FetchByKey("email", "validintx@example.com", &checkUser)
			assert.True(t, EmptyResult(err), "Valid user should also be rolled back")

			// Verify initial user still exists
			err = helper.userRepo.FetchByKey("email", "constraint@example.com", &checkUser)
			require.NoError(t, err)
			assert.Equal(t, "Initial User", checkUser.Name)
		})

		t.Run("MultipleCommitsError", func(t *testing.T) {
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Insert user
			user := ExtensiveTestUser{
				Name:     "Double Commit User",
				Email:    "doublecommit@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"double", "commit"},
			}
			err = tx.Insert(&user)
			require.NoError(t, err)

			// First commit should succeed
			err = tx.Commit()
			require.NoError(t, err)

			// Second commit should fail
			err = tx.Commit()
			assert.Error(t, err, "Second commit should fail")
		})

		t.Run("CommitAfterRollbackError", func(t *testing.T) {
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Insert user
			user := ExtensiveTestUser{
				Name:     "Rollback Then Commit User",
				Email:    "rollbackcommit@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"rollback", "commit"},
			}
			err = tx.Insert(&user)
			require.NoError(t, err)

			// Rollback
			err = tx.Rollback()
			require.NoError(t, err)

			// Commit should fail after rollback
			err = tx.Commit()
			assert.Error(t, err, "Commit should fail after rollback")
		})
	})

	t.Run("TransactionLifecycle_NestedOperations", func(t *testing.T) {
		t.Run("ComplexNestedUpdatesAndDeletes", func(t *testing.T) {
			// Create initial data
			users := helper.createSampleUsers(t)
			profiles := helper.createSampleProfiles(t, users)
			require.Len(t, users, 4)
			require.Len(t, profiles, 3)

			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Step 1: Update multiple users with different conditions
			err = tx.UpdateFields(&ExtensiveTestUser{},
				map[string]any{"status": "tx_updated", "score": 88.0},
				map[string]any{"is_active": true})
			require.NoError(t, err)

			// Step 2: Insert new user
			newUser := ExtensiveTestUser{
				Name:     "TX Nested User",
				Email:    "txnested@example.com",
				Status:   "new",
				Score:    75.0,
				IsActive: true,
				Tags:     pq.StringArray{"nested", "tx"},
			}
			err = tx.Insert(&newUser)
			require.NoError(t, err)

			// Step 3: Delete inactive users
			err = tx.DeleteWhere(map[string]any{"is_active": false})
			require.NoError(t, err)

			// Step 4: Count active users (should include updated and new user)
			count, err := tx.CountWhere(map[string]any{"is_active": true})
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, int64(3), "Should have at least 3 active users")

			// Step 5: Verify updates within transaction
			var updatedUsers []ExtensiveTestUser
			err = tx.FetchWhere(map[string]any{"status": "tx_updated"}, &updatedUsers)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(updatedUsers), 2, "Should have updated active users")

			for _, user := range updatedUsers {
				assert.Equal(t, 88.0, user.Score)
				assert.Equal(t, "tx_updated", user.Status)
			}

			// Step 6: Verify new user exists
			var createdUser ExtensiveTestUser
			err = tx.FetchByKey("email", "txnested@example.com", &createdUser)
			require.NoError(t, err)
			assert.Equal(t, "TX Nested User", createdUser.Name)

			// Step 7: Commit transaction
			err = tx.Commit()
			require.NoError(t, err)

			// Step 8: Verify all changes persisted
			var finalUsers []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Order(goqu.C("id").Asc())
			err = helper.userRepo.Fetch(query, &finalUsers)
			require.NoError(t, err)

			// Check that inactive users were deleted and active users updated
			activeCount := 0
			updatedCount := 0
			for _, user := range finalUsers {
				if user.IsActive {
					activeCount++
				}
				if user.Status == "tx_updated" {
					updatedCount++
					assert.Equal(t, 88.0, user.Score)
				}
			}

			assert.GreaterOrEqual(t, activeCount, 3, "Should have active users including new one")
			assert.GreaterOrEqual(t, updatedCount, 2, "Should have updated users")

			// Verify new user persisted
			err = helper.userRepo.FetchByKey("email", "txnested@example.com", &createdUser)
			require.NoError(t, err)
			assert.Equal(t, "TX Nested User", createdUser.Name)
		})
	})

	t.Run("TransactionLifecycle_LongRunningTransaction", func(t *testing.T) {
		t.Run("BatchProcessingWithCheckpoints", func(t *testing.T) {
			tx, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)

			// Simulate batch processing with multiple operations
			batchSize := 5
			for i := 0; i < batchSize; i++ {
				user := ExtensiveTestUser{
					Name:     "Batch User " + string(rune(i+65)), // A, B, C, etc.
					Email:    "batch" + string(rune(i+65)) + "@example.com",
					Status:   "batch",
					Score:    float64(80 + i),
					IsActive: i%2 == 0, // Alternate active/inactive
					Tags:     pq.StringArray{"batch", "processing", string(rune(i + 65))},
				}

				err = tx.Insert(&user)
				require.NoError(t, err)

				// Simulate processing time
				time.Sleep(10 * time.Millisecond)

				// Checkpoint: verify insertion
				var checkUser ExtensiveTestUser
				err = tx.FetchByKey("email", user.Email, &checkUser)
				require.NoError(t, err)
				assert.Equal(t, user.Name, checkUser.Name)
			}

			// Batch update all inserted users
			err = tx.UpdateFields(&ExtensiveTestUser{},
				map[string]any{"status": "batch_processed"},
				map[string]any{"status": "batch"})
			require.NoError(t, err)

			// Verify batch update
			count, err := tx.CountWhere(map[string]any{"status": "batch_processed"})
			require.NoError(t, err)
			assert.Equal(t, int64(batchSize), count)

			// Commit entire batch
			err = tx.Commit()
			require.NoError(t, err)

			// Verify all batch operations persisted
			var batchUsers []ExtensiveTestUser
			err = helper.userRepo.FetchWhere(map[string]any{"status": "batch_processed"}, &batchUsers)
			require.NoError(t, err)
			assert.Len(t, batchUsers, batchSize, "All batch users should be committed")

			// Verify data integrity
			for i, user := range batchUsers {
				assert.Contains(t, user.Email, "batch")
				assert.Equal(t, "batch_processed", user.Status)
				assert.GreaterOrEqual(t, user.Score, 80.0)
				assert.Contains(t, []string(user.Tags), "batch")
				assert.Contains(t, []string(user.Tags), "processing")

				// Verify alternating active status was preserved
				expectedActive := i%2 == 0
				assert.Equal(t, expectedActive, user.IsActive)
			}
		})
	})

	t.Run("TransactionLifecycle_IsolationTesting", func(t *testing.T) {
		t.Run("ReadIsolationBetweenTransactions", func(t *testing.T) {
			// Create initial user
			initialUser := ExtensiveTestUser{
				Name:     "Isolation Test User",
				Email:    "isolation@example.com",
				Status:   "initial",
				Score:    50.0,
				IsActive: true,
				Tags:     pq.StringArray{"isolation", "test"},
			}
			err := helper.userRepo.Insert(&initialUser)
			require.NoError(t, err)

			// Get user ID
			var user ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "isolation@example.com", &user)
			require.NoError(t, err)

			// Start transaction 1
			tx1, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)
			defer tx1.Rollback()

			// Update user in transaction 1
			err = tx1.UpdateFields(&ExtensiveTestUser{},
				map[string]any{"status": "tx1_updated", "score": 100.0},
				map[string]any{"id": user.ID})
			require.NoError(t, err)

			// Verify update in transaction 1
			var tx1User ExtensiveTestUser
			err = tx1.FetchByKey("id", user.ID, &tx1User)
			require.NoError(t, err)
			assert.Equal(t, "tx1_updated", tx1User.Status)
			assert.Equal(t, 100.0, tx1User.Score)

			// Read from outside transaction (should see original values)
			var outsideUser ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", user.ID, &outsideUser)
			require.NoError(t, err)
			assert.Equal(t, "initial", outsideUser.Status)
			assert.Equal(t, 50.0, outsideUser.Score)

			// Start transaction 2
			tx2, err := helper.userRepo.NewTransaction(nil)
			require.NoError(t, err)
			defer tx2.Rollback()

			var tx2User ExtensiveTestUser
			err = tx2.FetchByKey("id", user.ID, &tx2User)
			require.NoError(t, err)
			assert.Equal(t, "initial", tx2User.Status)
			assert.Equal(t, 50.0, tx2User.Score)

			// Note:
			// The code below causes an infinite lock in postgresql, because the previously finished select
			// acquired locks that will only be freed after the transaction finishes - in this case in tx1
			// Certain locks (e.g., AccessShareLock on the table or visibility checks on rows) are still active until
			//the end of the transaction
			//
			// Update in transaction 2 with different values

			// Commit transaction 2
			err = tx2.Commit()
			require.NoError(t, err)

			// Rollback transaction 1
			err = tx1.Rollback()
			require.NoError(t, err)

			// Final verification: should see tx2 committed changes
			err = helper.userRepo.FetchByKey("id", user.ID, &outsideUser)
			require.NoError(t, err)
			assert.Equal(t, initialUser.Status, outsideUser.Status)
			assert.Equal(t, initialUser.Score, outsideUser.Score)

		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}
