// +build integration

package db

import (
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtensive_Writer_Interface tests all Writer interface methods comprehensively
func TestExtensive_Writer_Interface(t *testing.T) {
	helper := setupExtensiveIntegrationTest(t)
	defer helper.cleanup()

	t.Run("Insert", func(t *testing.T) {
		t.Run("InsertSingleRecord", func(t *testing.T) {
			age := 28
			bio := "Single record insert test"
			user := ExtensiveTestUser{
				Name:     "Single Insert User",
				Email:    "single@example.com",
				Age:      &age,
				Status:   "active",
				Score:    88.5,
				IsActive: true,
				Bio:      &bio,
				Tags:     pq.StringArray{"test", "single"},
			}

			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Verify insertion by fetching
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "single@example.com", &result)
			require.NoError(t, err)
			assertUserEqual(t, &user, &result, "Single insert")
		})

		t.Run("InsertMultipleRecords", func(t *testing.T) {
			age1, age2, age3 := 25, 30, 35
			bio1 := "Batch user 1"
			bio2 := "Batch user 2"

			users := []any{
				&ExtensiveTestUser{
					Name:     "Batch User 1",
					Email:    "batch1@example.com",
					Age:      &age1,
					Status:   "active",
					Score:    75.0,
					IsActive: true,
					Bio:      &bio1,
					Tags:     pq.StringArray{"batch", "user1"},
				},
				&ExtensiveTestUser{
					Name:     "Batch User 2",
					Email:    "batch2@example.com",
					Age:      &age2,
					Status:   "pending",
					Score:    82.5,
					IsActive: false,
					Bio:      &bio2,
					Tags:     pq.StringArray{"batch", "user2"},
				},
				&ExtensiveTestUser{
					Name:     "Batch User 3",
					Email:    "batch3@example.com",
					Age:      &age3,
					Status:   "active",
					Score:    90.0,
					IsActive: true,
					Bio:      nil, // Test null bio
					Tags:     pq.StringArray{"batch", "user3"},
				},
			}

			err := helper.userRepo.Insert(users...)
			require.NoError(t, err)

			// Verify all users were inserted
			var results []ExtensiveTestUser
			fieldValues := map[string]any{"status": "active"}
			err = helper.userRepo.FetchWhere(fieldValues, &results)
			require.NoError(t, err)

			// Should have at least 2 active users from this batch
			activeCount := 0
			for _, result := range results {
				if result.Name == "Batch User 1" || result.Name == "Batch User 3" {
					activeCount++
				}
			}
			assert.GreaterOrEqual(t, activeCount, 2, "Should have inserted active batch users")
		})

		t.Run("InsertWithOmitNilFields", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Omit Nil User",
				Email:    "omitnil@example.com",
				Age:      nil, // This should be omitted
				Status:   "pending",
				Score:    77.5,
				IsActive: false,
				Bio:      nil, // This should be omitted
				Tags:     pq.StringArray{"omitnil"},
			}

			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Verify insertion and nil fields
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "omitnil@example.com", &result)
			require.NoError(t, err)

			assert.Equal(t, "Omit Nil User", result.Name)
			assert.Nil(t, result.Age, "Age should be nil")
			assert.Nil(t, result.Bio, "Bio should be nil")
		})

		t.Run("InsertConstraintViolation", func(t *testing.T) {
			// Try to insert duplicate email
			user1 := ExtensiveTestUser{
				Name:     "Duplicate Test 1",
				Email:    "duplicate@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"duplicate"},
			}

			user2 := ExtensiveTestUser{
				Name:     "Duplicate Test 2",
				Email:    "duplicate@example.com", // Same email
				Status:   "active",
				Score:    85.0,
				IsActive: true,
				Tags:     pq.StringArray{"duplicate"},
			}

			// First insert should succeed
			err := helper.userRepo.Insert(&user1)
			require.NoError(t, err)

			// Second insert should fail due to unique constraint
			err = helper.userRepo.Insert(&user2)
			assert.Error(t, err, "Should fail due to unique email constraint")
		})

		t.Run("InsertEmptyArray", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Empty Tags User",
				Email:    "emptytags@example.com",
				Status:   "active",
				Score:    70.0,
				IsActive: true,
				Tags:     pq.StringArray{}, // Empty array
			}

			err := helper.userRepo.Insert(&user)
			require.NoError(t, err)

			// Verify insertion
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("email", "emptytags@example.com", &result)
			require.NoError(t, err)
			assert.Equal(t, []string{}, []string(result.Tags), "Tags should be empty array")
		})
	})

	t.Run("InsertReturning", func(t *testing.T) {
		t.Run("InsertReturning_StructTarget", func(t *testing.T) {
			age := 29
			bio := "Struct target test"
			user := ExtensiveTestUser{
				Name:     "Struct Target User",
				Email:    "struct@example.com",
				Age:      &age,
				Status:   "active",
				Score:    91.0,
				IsActive: true,
				Bio:      &bio,
				Tags:     pq.StringArray{"struct", "target"},
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.InsertReturning(&user, pq.StringArray{"id", "name", "email", "created_at", "updated_at"}, result)
			require.NoError(t, err)

			assert.NotZero(t, result.ID, "ID should be populated")
			assert.Equal(t, "Struct Target User", result.Name)
			assert.Equal(t, "struct@example.com", result.Email)
			assert.False(t, result.CreatedAt.IsZero(), "CreatedAt should be populated")
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be populated")
		})

		t.Run("InsertReturning_MultipleVariables", func(t *testing.T) {
			age := 26
			user := ExtensiveTestUser{
				Name:     "Multiple Vars User",
				Email:    "multivars@example.com",
				Age:      &age,
				Status:   "pending",
				Score:    84.0,
				IsActive: false,
				Tags:     pq.StringArray{"multiple", "variables"},
			}

			var id int64
			var name, email, status string
			var score float64
			var isActive bool
			var createdAt time.Time

			err := helper.userRepo.InsertReturning(&user, 
				pq.StringArray{"id", "name", "email", "status", "score", "is_active", "created_at"}, 
				&id, &name, &email, &status, &score, &isActive, &createdAt)
			require.NoError(t, err)

			assert.NotZero(t, id, "ID should be populated")
			assert.Equal(t, "Multiple Vars User", name)
			assert.Equal(t, "multivars@example.com", email)
			assert.Equal(t, "pending", status)
			assert.Equal(t, 84.0, score)
			assert.False(t, isActive)
			assert.False(t, createdAt.IsZero(), "CreatedAt should be populated")
		})

		t.Run("InsertReturning_SingleVariable", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Single Var User",
				Email:    "singlevar@example.com",
				Status:   "active",
				Score:    78.5,
				IsActive: true,
				Tags:     pq.StringArray{"single", "variable"},
			}

			var id int64
			err := helper.userRepo.InsertReturning(&user, pq.StringArray{"id"}, &id)
			require.NoError(t, err)

			assert.NotZero(t, id, "ID should be populated")

			// Verify the record was actually inserted
			var result ExtensiveTestUser
			err = helper.userRepo.FetchByKey("id", id, &result)
			require.NoError(t, err)
			assert.Equal(t, "Single Var User", result.Name)
		})

		t.Run("InsertReturning_AllFields", func(t *testing.T) {
			age := 32
			bio := "All fields test"
			user := ExtensiveTestUser{
				Name:     "All Fields User",
				Email:    "allfields@example.com",
				Age:      &age,
				Status:   "active",
				Score:    96.5,
				IsActive: true,
				Bio:      &bio,
				Tags:     pq.StringArray{"all", "fields", "test"},
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.InsertReturning(&user, 
				pq.StringArray{"id", "name", "email", "age", "status", "score", "is_active", "bio", "tags", "created_at", "updated_at"}, 
				result)
			require.NoError(t, err)

			assert.NotZero(t, result.ID)
			assert.Equal(t, "All Fields User", result.Name)
			assert.Equal(t, "allfields@example.com", result.Email)
			assert.NotNil(t, result.Age)
			assert.Equal(t, 32, *result.Age)
			assert.Equal(t, "active", result.Status)
			assert.Equal(t, 96.5, result.Score)
			assert.True(t, result.IsActive)
			assert.NotNil(t, result.Bio)
			assert.Equal(t, "All fields test", *result.Bio)
			assert.Equal(t, []string{"all", "fields", "test"}, []string(result.Tags))
			assert.False(t, result.CreatedAt.IsZero())
			assert.False(t, result.UpdatedAt.IsZero())
		})

		t.Run("InsertReturning_WithNullFields", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Null Fields User",
				Email:    "nullfields@example.com",
				Age:      nil, // Null age
				Status:   "inactive",
				Score:    65.0,
				IsActive: false,
				Bio:      nil, // Null bio
				Tags:     pq.StringArray{"null", "fields"},
			}

			result := &ExtensiveTestUser{}
			err := helper.userRepo.InsertReturning(&user, 
				pq.StringArray{"id", "name", "email", "age", "status", "bio", "created_at"}, 
				result)
			require.NoError(t, err)

			assert.NotZero(t, result.ID)
			assert.Equal(t, "Null Fields User", result.Name)
			assert.Nil(t, result.Age, "Age should remain nil")
			assert.Nil(t, result.Bio, "Bio should remain nil")
			assert.Equal(t, "inactive", result.Status)
		})

		t.Run("InsertReturning_ArrayTypes", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Array Test User",
				Email:    "arraytest@example.com",
				Status:   "active",
				Score:    89.0,
				IsActive: true,
				Tags:     pq.StringArray{"array", "postgresql", "tags", "test"},
			}

			var id int64
			var tags pq.StringArray
			err := helper.userRepo.InsertReturning(&user, pq.StringArray{"id", "tags"}, &id, &tags)
			require.NoError(t, err)

			assert.NotZero(t, id)
			assert.Equal(t, []string{"array", "postgresql", "tags", "test"}, []string(tags))
		})

		t.Run("InsertReturning_NoTargets", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "No Targets User",
				Email:    "notargets@example.com",
				Status:   "active",
				Score:    70.0,
				IsActive: true,
				Tags:     pq.StringArray{"no", "targets"},
			}

			err := helper.userRepo.InsertReturning(&user, pq.StringArray{"id"})
			assert.Equal(t, ErrInvalidParameters, err, "Should return error for no targets")
		})

		t.Run("InsertReturning_EmptyReturnFields", func(t *testing.T) {
			user := ExtensiveTestUser{
				Name:     "Empty Return User",
				Email:    "emptyreturn@example.com",
				Status:   "active",
				Score:    75.0,
				IsActive: true,
				Tags:     pq.StringArray{"empty", "return"},
			}

			var id int64
			err := helper.userRepo.InsertReturning(&user, pq.StringArray{}, &id)
			assert.Error(t, err, "Should return error for empty return fields")
		})

		// Test with profiles repository for variety
		t.Run("InsertReturning_ProfileRepository", func(t *testing.T) {
			// First create a user to reference
			user := ExtensiveTestUser{
				Name:     "Profile Owner",
				Email:    "profileowner@example.com",
				Status:   "active",
				Score:    80.0,
				IsActive: true,
				Tags:     pq.StringArray{"profile", "owner"},
			}
			var userID int64
			err := helper.userRepo.InsertReturning(&user, pq.StringArray{"id"}, &userID)
			require.NoError(t, err)

			// Now create profile
			profile := TestProfile{
				UserID:   userID,
				Website:  "https://profiletest.com",
				Company:  "ProfileCorp",
				Location: "Test City, TC",
			}

			result := &TestProfile{}
			err = helper.profileRepo.InsertReturning(&profile, pq.StringArray{"id", "user_id", "website", "company", "location"}, result)
			require.NoError(t, err)

			assert.NotZero(t, result.ID)
			assert.Equal(t, userID, result.UserID)
			assert.Equal(t, "https://profiletest.com", result.Website)
			assert.Equal(t, "ProfileCorp", result.Company)
			assert.Equal(t, "Test City, TC", result.Location)
		})
	})

	// Clean up test data for next tests
	helper.cleanupTestData(t)
}