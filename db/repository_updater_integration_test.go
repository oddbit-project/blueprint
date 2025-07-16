//go:build integration && pgsql
// +build integration,pgsql

package db

import (
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtensive_Updater_Interface tests all Updater interface methods comprehensively
func TestExtensive_Updater_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	// Create sample data for testing updates
	users := helper.createSampleUsers(t)
	require.Len(t, users, 4, "Should have 4 sample users")

	t.Run("Update", func(t *testing.T) {
		t.Run("UpdateMultipleRecords", func(t *testing.T) {
			// Update all pending users to active
			err := helper.userRepo.UpdateFields(&ExtensiveTestUser{},
				map[string]any{
					"status":    "active",
					"is_active": true,
				},
				map[string]any{
					"status": "pending",
				})
			require.NoError(t, err)

			// Verify updates
			var results []ExtensiveTestUser
			err = helper.userRepo.FetchWhere(map[string]any{"status": "active"}, &results)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(results), 1, "Should have updated pending users to active")
		})
	})

	t.Run("UpdateRecord", func(t *testing.T) {
		t.Run("UpdateSingleFieldCondition", func(t *testing.T) {
			// Update Alice's score and bio
			newAge := 26
			newBio := "Updated bio for Alice"
			updateUser := ExtensiveTestUser{
				Name:     "Alice Johnson Updated",
				Age:      &newAge,
				Status:   "premium",
				Score:    98.5,
				IsActive: true,
				Bio:      &newBio,
				Tags:     pq.StringArray{"developer", "golang", "backend", "premium"},
			}

			whereConditions := map[string]any{
				"id": users[0].ID,
			}

			err := helper.userRepo.UpdateRecord(&updateUser, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "Alice Johnson Updated", result.Name)
			assert.Equal(t, "premium", result.Status)
			assert.Equal(t, 98.5, result.Score)
			assert.NotNil(t, result.Age)
			assert.Equal(t, 26, *result.Age)
			assert.NotNil(t, result.Bio)
			assert.Equal(t, "Updated bio for Alice", *result.Bio)
		})

		t.Run("UpdateMultipleFieldConditions", func(t *testing.T) {
			// Update Bob using multiple conditions
			updateUser := ExtensiveTestUser{
				Name:     "Bob Smith Senior",
				Status:   "senior",
				Score:    92.0,
				IsActive: true,
				Email:    "bob@example.com",
			}

			whereConditions := map[string]any{
				"email":  "bob@example.com",
				"status": "active", // Bob was updated to active in previous test
			}

			err := helper.userRepo.UpdateRecord(&updateUser, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "bob@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "Bob Smith Senior", result.Name)
			assert.Equal(t, "senior", result.Status)
			assert.Equal(t, 92.0, result.Score)
			assert.True(t, result.IsActive)
		})

		t.Run("UpdateWithNullValues", func(t *testing.T) {
			// Update David to have null age and bio
			updateUser := ExtensiveTestUser{
				Name:     "David Brown Updated",
				Age:      nil, // Set to null
				Status:   "updated",
				Score:    80.0,
				IsActive: true,
				Bio:      nil, // Keep null
				Tags:     pq.StringArray{"sales", "updated"},
				Email:    "david@example.com",
			}

			whereConditions := map[string]any{
				"id": users[3].ID,
			}

			err := helper.userRepo.UpdateRecord(&updateUser, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[3].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "David Brown Updated", result.Name)
			assert.Equal(t, "updated", result.Status)
			assert.Nil(t, result.Age, "Age should remain null")
			assert.Nil(t, result.Bio, "Bio should remain null")
		})

		t.Run("UpdateNoWhereConditions", func(t *testing.T) {
			// Test update without WHERE conditions (should fail)
			updateUser := ExtensiveTestUser{
				Score: 100.0, // Set all scores to 100
			}

			err := helper.userRepo.UpdateRecord(&updateUser, nil)
			require.Error(t, err)
		})
	})

	t.Run("UpdateFields", func(t *testing.T) {
		t.Run("UpdateSpecificFields", func(t *testing.T) {
			// Update only specific fields of Carol
			fieldsToUpdate := map[string]any{
				"status":    "expert",
				"score":     97.5,
				"is_active": true,
			}

			whereConditions := map[string]any{
				"id": users[2].ID,
			}

			err := helper.userRepo.UpdateFields(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions)
			require.NoError(t, err)

			// Verify only specified fields were updated
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[2].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "expert", result.Status)
			assert.Equal(t, 97.5, result.Score)
			assert.True(t, result.IsActive)
			// Name should remain unchanged
			assert.Equal(t, "Carol Williams", result.Name)
		})

		t.Run("UpdateFieldsArray", func(t *testing.T) {
			// Update tags array
			fieldsToUpdate := map[string]any{
				"tags": pq.StringArray{"updated", "tags", "array"},
			}

			whereConditions := map[string]any{
				"id": users[1].ID,
			}

			err := helper.userRepo.UpdateFields(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[1].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, []string{"updated", "tags", "array"}, []string(result.Tags))
		})
	})

	t.Run("UpdateReturning", func(t *testing.T) {
		t.Run("UpdateReturning_StructTarget", func(t *testing.T) {
			newAge := 31
			updateUser := ExtensiveTestUser{
				Name:     "Bob Updated Again",
				Age:      &newAge,
				Status:   "master",
				Score:    94.5,
				IsActive: true,
				Email:    users[1].Email,
			}

			whereConditions := map[string]any{
				"id": users[1].ID,
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.UpdateReturning(&updateUser, whereConditions,
				pq.StringArray{"id", "name", "age", "status", "score", "updated_at"}, result)
			require.NoError(t, err)

			assert.Equal(t, users[1].ID, result.ID)
			assert.Equal(t, "Bob Updated Again", result.Name)
			assert.NotNil(t, result.Age)
			assert.Equal(t, 31, *result.Age)
			assert.Equal(t, "master", result.Status)
			assert.Equal(t, 94.5, result.Score)
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be set")
		})

		t.Run("UpdateReturning_MultipleVariables", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Status:   "legend",
				Score:    99.9,
				IsActive: true,
				Email:    users[2].Email,
				Name:     users[2].Name,
			}

			whereConditions := map[string]any{
				"id": users[2].ID,
			}

			var id int64
			var name, status string
			var score float64
			var isActive bool
			var updatedAt time.Time

			err := helper.userRepo.UpdateReturning(&updateUser, whereConditions,
				pq.StringArray{"id", "name", "status", "score", "is_active", "updated_at"},
				&id, &name, &status, &score, &isActive, &updatedAt)
			require.NoError(t, err)

			assert.Equal(t, users[2].ID, id)
			assert.Equal(t, "Carol Williams", name) // Name shouldn't change
			assert.Equal(t, "legend", status)
			assert.Equal(t, 99.9, score)
			assert.True(t, isActive)
			assert.False(t, updatedAt.IsZero())
		})

		t.Run("UpdateReturning_SingleVariable", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Score: 88.8,
				Email: users[3].Email,
			}

			whereConditions := map[string]any{
				"id": users[3].ID,
			}

			updatedAt := time.Time{}
			err := helper.userRepo.UpdateReturning(&updateUser, whereConditions,
				pq.StringArray{"updated_at"}, &updatedAt)
			require.NoError(t, err)

			assert.False(t, updatedAt.IsZero(), "UpdatedAt should be returned")
		})

		t.Run("UpdateReturning_NoTargets", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Score: 77.7,
			}

			whereConditions := map[string]any{
				"id": users[0].ID,
			}

			err := helper.userRepo.UpdateReturning(&updateUser, whereConditions, pq.StringArray{"id"})
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for no targets")
		})
	})

	t.Run("UpdateFieldsReturning", func(t *testing.T) {
		t.Run("UpdateFieldsReturning_StructTarget", func(t *testing.T) {
			fieldsToUpdate := map[string]any{
				"status":    "champion",
				"score":     100.0,
				"is_active": true,
			}

			whereConditions := map[string]any{
				"id": users[0].ID,
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.UpdateFieldsReturning(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions,
				pq.StringArray{"id", "name", "status", "score", "is_active", "updated_at"}, result)
			require.NoError(t, err)

			assert.Equal(t, users[0].ID, result.ID)
			assert.Equal(t, "Alice Johnson Updated", result.Name) // From previous test
			assert.Equal(t, "champion", result.Status)
			assert.Equal(t, 100.0, result.Score)
			assert.True(t, result.IsActive)
			assert.False(t, result.UpdatedAt.IsZero())
		})

		t.Run("UpdateFieldsReturning_MultipleVariables", func(t *testing.T) {
			fieldsToUpdate := map[string]any{
				"bio":   "Updated bio via fields returning",
				"score": 95.5,
			}

			whereConditions := map[string]any{
				"id": users[1].ID,
			}

			var id int64
			var bio string
			var score float64
			var updatedAt time.Time

			err := helper.userRepo.UpdateFieldsReturning(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions,
				pq.StringArray{"id", "bio", "score", "updated_at"}, &id, &bio, &score, &updatedAt)
			require.NoError(t, err)

			assert.Equal(t, users[1].ID, id)
			assert.Equal(t, "Updated bio via fields returning", bio)
			assert.Equal(t, 95.5, score)
			assert.False(t, updatedAt.IsZero())
		})

		t.Run("UpdateFieldsReturning_WithNullField", func(t *testing.T) {
			fieldsToUpdate := map[string]any{
				"age": nil, // Set age to null
			}

			whereConditions := map[string]any{
				"id": users[2].ID,
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.UpdateFieldsReturning(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions,
				pq.StringArray{"id", "name", "age"}, result)
			require.NoError(t, err)

			assert.Equal(t, users[2].ID, result.ID)
			assert.Equal(t, "Carol Williams", result.Name)
			assert.Nil(t, result.Age, "Age should be set to null")
		})

		t.Run("UpdateFieldsReturning_ArrayField", func(t *testing.T) {
			fieldsToUpdate := map[string]any{
				"tags": pq.StringArray{"field", "update", "returning", "test"},
			}

			whereConditions := map[string]any{
				"id": users[3].ID,
			}

			tags := pq.StringArray{}
			err := helper.userRepo.UpdateFieldsReturning(&ExtensiveTestUser{}, fieldsToUpdate, whereConditions,
				[]string{"tags"}, &tags)
			require.NoError(t, err)

			assert.Equal(t, pq.StringArray{"field", "update", "returning", "test"}, tags)
		})
	})

	t.Run("UpdateByKey", func(t *testing.T) {
		t.Run("UpdateByPrimaryKey", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Name:     "Alice Final Update",
				Status:   "final",
				Score:    100.0,
				IsActive: true,
			}

			err := helper.userRepo.UpdateByKey(&updateUser, "id", users[0].ID)
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", users[0].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "Alice Final Update", result.Name)
			assert.Equal(t, "final", result.Status)
			assert.Equal(t, 100.0, result.Score)
		})

		t.Run("UpdateByUniqueKey", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Name:     "Bob Final Update",
				Status:   "ultimate",
				Score:    99.0,
				IsActive: true,
				Email:    "bob@example.com",
			}

			err := helper.userRepo.UpdateByKey(&updateUser, "email", "bob@example.com")
			require.NoError(t, err)

			// Verify update
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "bob@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, "Bob Final Update", result.Name)
			assert.Equal(t, "ultimate", result.Status)
			assert.Equal(t, 99.0, result.Score)
		})

		t.Run("UpdateByKeyNotFound", func(t *testing.T) {
			updateUser := ExtensiveTestUser{
				Name:   "Non-existent User",
				Status: "test",
			}

			err := helper.userRepo.UpdateByKey(&updateUser, "id", 99999)
			require.NoError(t, err) // Update succeeds but affects 0 rows

			// Verify no record exists with this ID
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", 99999, &result)
			assert.True(t, EmptyResult(err), "Should not find record with ID 99999")
		})
	})

	// Test with profiles repository for variety
	t.Run("Updater_ProfileRepository", func(t *testing.T) {
		// Create a profile first
		profiles := helper.createSampleProfiles(t, users)
		require.Len(t, profiles, 3, "Should have 3 profiles")

		t.Run("UpdateProfileRecord", func(t *testing.T) {
			updateProfile := TestProfile{
				UserID:   profiles[0].ID,
				Website:  "https://updated-alice.dev",
				Company:  "UpdatedTechCorp",
				Location: "Updated San Francisco, CA",
			}

			whereConditions := map[string]any{
				"id": profiles[0].ID,
			}

			err := helper.profileRepo.UpdateRecord(&updateProfile, whereConditions)
			require.NoError(t, err)

			// Verify update
			var result TestProfile
			err = helper.profileRepo.FetchByKey("id", profiles[0].ID, &result)
			require.NoError(t, err)
			assert.Equal(t, "https://updated-alice.dev", result.Website)
			assert.Equal(t, "UpdatedTechCorp", result.Company)
			assert.Equal(t, "Updated San Francisco, CA", result.Location)
		})

		t.Run("UpdateProfileFieldsReturning", func(t *testing.T) {
			fieldsToUpdate := map[string]any{
				"website": "https://super-bob.com",
				"company": "SuperStartup",
			}

			whereConditions := map[string]any{
				"user_id": users[1].ID,
			}

			result := &TestProfile{}
			err := helper.profileRepo.UpdateFieldsReturning(&TestProfile{}, fieldsToUpdate, whereConditions,
				pq.StringArray{"id", "user_id", "website", "company", "location"}, result)
			require.NoError(t, err)

			assert.Equal(t, profiles[1].ID, result.ID)
			assert.Equal(t, users[1].ID, result.UserID)
			assert.Equal(t, "https://super-bob.com", result.Website)
			assert.Equal(t, "SuperStartup", result.Company)
			assert.Equal(t, "New York, NY", result.Location) // Should remain unchanged
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}
