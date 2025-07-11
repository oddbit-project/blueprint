// +build integration

package db

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtensive_Reader_Interface tests all Reader interface methods comprehensively
func TestExtensive_Reader_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create sample data
	users := helper.createSampleUsers(t)
	_ = helper.createSampleProfiles(t, users)

	t.Run("FetchOne", func(t *testing.T) {
		t.Run("FetchFirstRecord", func(t *testing.T) {
			var result ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Where(goqu.C("email").Eq("alice@example.com"))
			
			err := helper.userRepo.FetchOne(query, &result)
			require.NoError(t, err)
			
			assert.Equal(t, users[0].ID, result.ID)
			assert.Equal(t, "Alice Johnson", result.Name)
			assert.Equal(t, "alice@example.com", result.Email)
			assert.Equal(t, "active", result.Status)
			assert.True(t, result.IsActive)
			assert.NotNil(t, result.Age)
			assert.Equal(t, 25, *result.Age)
			assert.NotNil(t, result.Bio)
		})

		t.Run("FetchWithComplexQuery", func(t *testing.T) {
			var result ExtensiveTestUser
			query := helper.userRepo.SqlSelect().
				Where(goqu.C("status").Eq("active")).
				Where(goqu.C("score").Gt(90)).
				Order(goqu.C("score").Desc())
			
			err := helper.userRepo.FetchOne(query, &result)
			require.NoError(t, err)
			
			// Should get Alice (highest score among active users)
			assert.Equal(t, "Alice Johnson", result.Name)
			assert.Equal(t, 95.5, result.Score)
		})

		t.Run("FetchNonExistentRecord", func(t *testing.T) {
			var result ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Where(goqu.C("email").Eq("nonexistent@example.com"))
			
			err := helper.userRepo.FetchOne(query, &result)
			assert.True(t, EmptyResult(err), "Should return empty result error")
		})

		t.Run("FetchWithNullFields", func(t *testing.T) {
			var result ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Where(goqu.C("email").Eq("david@example.com"))
			
			err := helper.userRepo.FetchOne(query, &result)
			require.NoError(t, err)
			
			assert.Equal(t, "David Brown", result.Name)
			assert.Nil(t, result.Age, "Age should be nil for David")
			assert.Nil(t, result.Bio, "Bio should be nil for David")
		})
	})

	t.Run("Fetch", func(t *testing.T) {
		t.Run("FetchAllUsers", func(t *testing.T) {
			var results []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Order(goqu.C("id").Asc())
			
			err := helper.userRepo.Fetch(query, &results)
			require.NoError(t, err)
			require.Len(t, results, 4, "Should fetch all 4 users")
			
			// Verify order and data
			assert.Equal(t, "Alice Johnson", results[0].Name)
			assert.Equal(t, "Bob Smith", results[1].Name)
			assert.Equal(t, "Carol Williams", results[2].Name)
			assert.Equal(t, "David Brown", results[3].Name)
		})

		t.Run("FetchActiveUsers", func(t *testing.T) {
			var results []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().
				Where(goqu.C("is_active").IsTrue()).
				Order(goqu.C("name").Asc())
			
			err := helper.userRepo.Fetch(query, &results)
			require.NoError(t, err)
			require.Len(t, results, 2, "Should fetch 2 active users")
			
			assert.Equal(t, "Alice Johnson", results[0].Name)
			assert.Equal(t, "Carol Williams", results[1].Name)
		})

		t.Run("FetchWithLimitAndOffset", func(t *testing.T) {
			var results []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().
				Order(goqu.C("id").Asc()).
				Limit(2).
				Offset(1)
			
			err := helper.userRepo.Fetch(query, &results)
			require.NoError(t, err)
			require.Len(t, results, 2, "Should fetch 2 users with offset")
			
			assert.Equal(t, "Bob Smith", results[0].Name)
			assert.Equal(t, "Carol Williams", results[1].Name)
		})

		t.Run("FetchEmptyResult", func(t *testing.T) {
			var results []ExtensiveTestUser
			query := helper.userRepo.SqlSelect().Where(goqu.C("status").Eq("nonexistent"))
			
			err := helper.userRepo.Fetch(query, &results)
			require.NoError(t, err)
			assert.Len(t, results, 0, "Should return empty slice for no matches")
		})
	})

	t.Run("FetchRecord", func(t *testing.T) {
		t.Run("FetchBySingleField", func(t *testing.T) {
			var result ExtensiveTestUser
			fieldValues := map[string]any{
				"email": "bob@example.com",
			}
			
			err := helper.userRepo.FetchRecord(fieldValues, &result)
			require.NoError(t, err)
			
			assert.Equal(t, "Bob Smith", result.Name)
			assert.Equal(t, "bob@example.com", result.Email)
			assert.Equal(t, "pending", result.Status)
		})

		t.Run("FetchByMultipleFields", func(t *testing.T) {
			var result ExtensiveTestUser
			fieldValues := map[string]any{
				"status":    "active",
				"is_active": true,
			}
			
			err := helper.userRepo.FetchRecord(fieldValues, &result)
			require.NoError(t, err)
			
			// Should get first active user (Alice or Carol)
			assert.True(t, result.Name == "Alice Johnson" || result.Name == "Carol Williams")
			assert.Equal(t, "active", result.Status)
			assert.True(t, result.IsActive)
		})

		t.Run("FetchRecordNotFound", func(t *testing.T) {
			var result ExtensiveTestUser
			fieldValues := map[string]any{
				"email": "notfound@example.com",
			}
			
			err := helper.userRepo.FetchRecord(fieldValues, &result)
			assert.True(t, EmptyResult(err), "Should return empty result error")
		})

		t.Run("FetchWithNilFieldValues", func(t *testing.T) {
			var result ExtensiveTestUser
			
			err := helper.userRepo.FetchRecord(nil, &result)
			assert.Equal(t, ErrInvalidParameters, err)
		})
	})

	t.Run("FetchByKey", func(t *testing.T) {
		t.Run("FetchByPrimaryKey", func(t *testing.T) {
			var result ExtensiveTestUser
			
			err := helper.userRepo.FetchByKey("id", users[1].ID, &result)
			require.NoError(t, err)
			
			assert.Equal(t, users[1].ID, result.ID)
			assert.Equal(t, "Bob Smith", result.Name)
			assert.Equal(t, "bob@example.com", result.Email)
		})

		t.Run("FetchByUniqueKey", func(t *testing.T) {
			var result ExtensiveTestUser
			
			err := helper.userRepo.FetchByKey("email", "carol@example.com", &result)
			require.NoError(t, err)
			
			assert.Equal(t, "Carol Williams", result.Name)
			assert.Equal(t, "carol@example.com", result.Email)
			assert.Equal(t, "active", result.Status)
		})

		t.Run("FetchByKeyNotFound", func(t *testing.T) {
			var result ExtensiveTestUser
			
			err := helper.userRepo.FetchByKey("id", 99999, &result)
			assert.True(t, EmptyResult(err), "Should return empty result error")
		})

		t.Run("FetchByNonExistentField", func(t *testing.T) {
			var result ExtensiveTestUser
			
			err := helper.userRepo.FetchByKey("nonexistent_field", "value", &result)
			assert.Error(t, err, "Should return error for non-existent field")
		})
	})

	t.Run("FetchWhere", func(t *testing.T) {
		t.Run("FetchActiveUsersWhere", func(t *testing.T) {
			var results []ExtensiveTestUser
			fieldValues := map[string]any{
				"is_active": true,
			}
			
			err := helper.userRepo.FetchWhere(fieldValues, &results)
			require.NoError(t, err)
			require.Len(t, results, 2, "Should fetch 2 active users")
			
			for _, user := range results {
				assert.True(t, user.IsActive, "All users should be active")
			}
		})

		t.Run("FetchByStatusAndActivity", func(t *testing.T) {
			var results []ExtensiveTestUser
			fieldValues := map[string]any{
				"status":    "active",
				"is_active": true,
			}
			
			err := helper.userRepo.FetchWhere(fieldValues, &results)
			require.NoError(t, err)
			require.Len(t, results, 2, "Should fetch 2 active users with active status")
			
			for _, user := range results {
				assert.Equal(t, "active", user.Status)
				assert.True(t, user.IsActive)
			}
		})

		t.Run("FetchWhereNoMatches", func(t *testing.T) {
			var results []ExtensiveTestUser
			fieldValues := map[string]any{
				"status": "nonexistent",
			}
			
			err := helper.userRepo.FetchWhere(fieldValues, &results)
			require.NoError(t, err)
			assert.Len(t, results, 0, "Should return empty slice for no matches")
		})

		t.Run("FetchWhereNilFieldValues", func(t *testing.T) {
			var results []ExtensiveTestUser
			
			err := helper.userRepo.FetchWhere(nil, &results)
			assert.Equal(t, ErrInvalidParameters, err)
		})
	})

	t.Run("Exists", func(t *testing.T) {
		t.Run("ExistsTrue", func(t *testing.T) {
			exists, err := helper.userRepo.Exists("email", "alice@example.com")
			require.NoError(t, err)
			assert.True(t, exists, "Alice should exist")
		})

		t.Run("ExistsFalse", func(t *testing.T) {
			exists, err := helper.userRepo.Exists("email", "nonexistent@example.com")
			require.NoError(t, err)
			assert.False(t, exists, "Non-existent email should not exist")
		})

		t.Run("ExistsWithSkip", func(t *testing.T) {
			// Check if another user exists with the same status as Alice, but not Alice herself
			exists, err := helper.userRepo.Exists("status", "active", "id", users[0].ID)
			require.NoError(t, err)
			assert.True(t, exists, "Carol should exist with status 'active' but different ID than Alice")
		})

		t.Run("ExistsWithSkipNoOtherMatch", func(t *testing.T) {
			// Check if another user exists with pending status, but not Bob
			exists, err := helper.userRepo.Exists("status", "pending", "id", users[1].ID)
			require.NoError(t, err)
			assert.False(t, exists, "No other user should have 'pending' status except Bob")
		})

		t.Run("ExistsInvalidSkipParameters", func(t *testing.T) {
			_, err := helper.userRepo.Exists("status", "active", "only_one_param")
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for invalid skip parameters")
		})

		t.Run("ExistsByNonExistentField", func(t *testing.T) {
			// This should return false (no records match) rather than error
			exists, err := helper.userRepo.Exists("nonexistent_field", "value")
			assert.Error(t, err, "Should return error for non-existent field")
			assert.False(t, exists)
		})

		t.Run("ExistsByNullValue", func(t *testing.T) {
			// Test existence check with null value
			exists, err := helper.userRepo.Exists("age", nil)
			require.NoError(t, err)
			assert.True(t, exists, "David has null age, so this should exist")
		})
	})

	// Test with different repositories (profiles)
	t.Run("Reader_ProfileRepository", func(t *testing.T) {
		t.Run("FetchProfileByUserID", func(t *testing.T) {
			var result TestProfile
			
			err := helper.profileRepo.FetchByKey("user_id", users[0].ID, &result)
			require.NoError(t, err)
			
			assert.Equal(t, users[0].ID, result.UserID)
			assert.Equal(t, "https://alice.dev", result.Website)
			assert.Equal(t, "TechCorp", result.Company)
		})

		t.Run("FetchAllProfiles", func(t *testing.T) {
			var results []TestProfile
			query := helper.profileRepo.SqlSelect().Order(goqu.C("id").Asc())
			
			err := helper.profileRepo.Fetch(query, &results)
			require.NoError(t, err)
			require.Len(t, results, 3, "Should fetch all 3 profiles")
		})

		t.Run("ProfileExistsByCompany", func(t *testing.T) {
			exists, err := helper.profileRepo.Exists("company", "TechCorp")
			require.NoError(t, err)
			assert.True(t, exists, "TechCorp profile should exist")
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}