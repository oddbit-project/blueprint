package config

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func setupMockDB(t *testing.T) (*db.SqlClient, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Convert sql.DB to sqlx.DB
	sqlxDB := sqlx.NewDb(mockDB, "clickhouse")

	client := &db.SqlClient{
		Conn: sqlxDB,
		DriverName: "clickhouse",
	}

	return client, mock
}

func setupTestLogger() *log.Logger {
	logger := &log.Logger{}
	return logger
}

func TestNewManager(t *testing.T) {
	client, _ := setupMockDB(t)
	logger := setupTestLogger()

	manager := NewManager(client, logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, client, manager.client)
	assert.Equal(t, logger, manager.logger)
}

func TestLoadConfig(t *testing.T) {
	client, mock := setupMockDB(t)
	logger := setupTestLogger()
	manager := NewManager(client, logger)

	// Mock users query
	mock.ExpectQuery("SELECT .* FROM system.users").WillReturnRows(
		sqlmock.NewRows([]string{"name", "storage_policy", "readonly", "allow_databases", "allow_dictionaries", "profile_name", "quota_name", "networks", "settings"}).
			AddRow("default", nil, nil, nil, nil, nil, nil, "127.0.0.1,::1", "{\"max_memory_usage\":\"10000000000\"}").
			AddRow("admin", nil, nil, nil, nil, "admin", "admin_quota", "127.0.0.1", "{\"max_memory_usage\":\"10000000000\"}"),
	)

	// Mock profiles query
	mock.ExpectQuery("SELECT .* FROM system.profiles").WillReturnRows(
		sqlmock.NewRows([]string{"name", "readonly", "settings"}).
			AddRow("default", 0, "{\"max_memory_usage\":\"10000000000\"}").
			AddRow("admin", 0, "{\"max_memory_usage\":\"20000000000\"}"),
	)

	// Mock quotas query
	mock.ExpectQuery("SELECT .* FROM system.quotas").WillReturnRows(
		sqlmock.NewRows([]string{"name", "intervals", "keys"}).
			AddRow("default", "[{\"duration\":3600, \"queries\":10000}]", nil).
			AddRow("admin_quota", "[{\"duration\":3600, \"queries\":20000}]", nil),
	)

	// Mock databases query
	mock.ExpectQuery("SELECT .* FROM system.databases").WillReturnRows(
		sqlmock.NewRows([]string{"name", "engine", "data_path", "metadata_path", "uuid"}).
			AddRow("default", "Atomic", "/var/lib/clickhouse/data/default/", "/var/lib/clickhouse/metadata/default/", nil).
			AddRow("system", "Atomic", "/var/lib/clickhouse/data/system/", "/var/lib/clickhouse/metadata/system/", nil),
	)

	// Mock storage tiers query
	mock.ExpectQuery("SELECT .* FROM system.disks").WillReturnRows(
		sqlmock.NewRows([]string{"name", "type", "path", "free_space", "total_space"}).
			AddRow("default", "local", "/var/lib/clickhouse/data/", 100000000000, 200000000000).
			AddRow("s3", "s3", "s3://my-bucket/clickhouse/", 9999999999999, 9999999999999),
	)

	// Mock storage policies query
	mock.ExpectQuery("SELECT .* FROM system.storage_policies").WillReturnRows(
		sqlmock.NewRows([]string{"policy_name", "volume_name", "volume_priority", "volume_type", "disks", "max_data_part_size", "move_factor", "prefer_not_to_merge"}).
			AddRow("default", "default", 1, "default", "default", nil, nil, nil).
			AddRow("tiered", "hot", 1, "default", "default", 1073741824, nil, nil).
			AddRow("tiered", "cold", 2, "default", "s3", nil, nil, nil),
	)

	// Mock roles query - table doesn't exist case
	mock.ExpectQuery("SELECT .* FROM system.tables").WillReturnRows(
		sqlmock.NewRows([]string{"exists"}).AddRow(0),
	)

	// Load configuration
	err := manager.LoadConfig(context.Background())
	require.NoError(t, err)

	// Verify configuration loaded
	config := manager.GetConfig()
	assert.NotNil(t, config)

	// Verify users
	assert.Len(t, config.Users, 2)
	assert.Contains(t, config.Users, "default")
	assert.Contains(t, config.Users, "admin")

	// Verify admin user
	adminUser := config.Users["admin"]
	assert.Equal(t, "admin", adminUser.Name)
	assert.Equal(t, "admin", adminUser.Profile)
	assert.Equal(t, "admin_quota", adminUser.Quota)
	assert.Equal(t, []string{"127.0.0.1"}, adminUser.Networks)

	// Verify profiles
	assert.Len(t, config.Profiles, 2)
	assert.Contains(t, config.Profiles, "default")
	assert.Contains(t, config.Profiles, "admin")

	// Verify quotas
	assert.Len(t, config.Quotas, 2)
	assert.Contains(t, config.Quotas, "default")
	assert.Contains(t, config.Quotas, "admin_quota")

	// Verify quota details
	defaultQuota := config.Quotas["default"]
	assert.Equal(t, "default", defaultQuota.Name)
	assert.Len(t, defaultQuota.Intervals, 1)
	assert.Equal(t, time.Hour, defaultQuota.Intervals[0].Duration)
	assert.Equal(t, 10000, defaultQuota.Intervals[0].Queries)

	// Verify databases
	assert.Len(t, config.Databases, 1) // system db should be skipped
	assert.Contains(t, config.Databases, "default")

	// Verify storage tiers
	assert.Len(t, config.StorageTiers, 2)
	assert.Contains(t, config.StorageTiers, "default")
	assert.Contains(t, config.StorageTiers, "s3")

	// Verify storage policies
	assert.Len(t, config.StoragePolicies, 2)
	assert.Contains(t, config.StoragePolicies, "default")
	assert.Contains(t, config.StoragePolicies, "tiered")

	// Verify tiered policy details
	tieredPolicy := config.StoragePolicies["tiered"]
	assert.Equal(t, "tiered", tieredPolicy.Name)
	assert.Len(t, tieredPolicy.Volumes, 2)
	assert.Equal(t, "hot", tieredPolicy.Volumes[0].Name)
	assert.Equal(t, "cold", tieredPolicy.Volumes[1].Name)
}

func TestExportImportConfig(t *testing.T) {
	client, _ := setupMockDB(t)
	logger := setupTestLogger()
	manager := NewManager(client, logger)

	// Add a test config
	originalConfig := manager.GetConfig()
	originalConfig.Users["test_user"] = UserConfig{
		Name:     "test_user",
		Password: "password",
		Profile:  "default",
		Networks: []string{"127.0.0.1"},
	}

	// Export config
	data, err := manager.ExportConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Create a new manager and import the config
	newManager := NewManager(client, logger)
	err = newManager.ImportConfig(data)
	require.NoError(t, err)

	// Verify the config was imported correctly
	importedConfig := newManager.GetConfig()
	assert.Contains(t, importedConfig.Users, "test_user")
	assert.Equal(t, "test_user", importedConfig.Users["test_user"].Name)
	assert.Equal(t, "password", importedConfig.Users["test_user"].Password)
}

func TestUserManager(t *testing.T) {
	client, mock := setupMockDB(t)
	logger := setupTestLogger()
	manager := NewManager(client, logger)
	userManager := NewUserManager(manager)

	// Setup test user
	testUser := UserConfig{
		Name:     "test_user",
		Password: "password",
		Profile:  "default",
		Networks: []string{"127.0.0.1"},
		Roles:    []string{"reader"},
	}

	// Add the user to the config
	manager.config.Users["test_user"] = testUser

	// GetUser should return the user
	user, err := userManager.GetUser("test_user")
	require.NoError(t, err)
	assert.Equal(t, testUser.Name, user.Name)
	assert.Equal(t, testUser.Password, user.Password)

	// ListUsers should include the user
	users := userManager.ListUsers()
	assert.Len(t, users, 1)
	assert.Equal(t, testUser.Name, users[0].Name)

	// Test CreateUser
	newUser := UserConfig{
		Name:     "new_user",
		Password: "password",
		Profile:  "default",
	}

	// Mock SQL execution
	mock.ExpectExec("CREATE USER IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(1, 1))

	err = userManager.CreateUser(context.Background(), newUser)
	require.NoError(t, err)
	assert.Contains(t, manager.config.Users, "new_user")

	// Test UpdateUser
	updatedUser := UserConfig{
		Name:     "test_user",
		Password: "new_password",
		Profile:  "admin",
	}

	// Mock SQL executions for update
	mock.ExpectExec("ALTER USER").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("REVOKE ALL ON").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("REVOKE ALL ROLES FROM").WillReturnResult(sqlmock.NewResult(1, 1))

	err = userManager.UpdateUser(context.Background(), updatedUser)
	require.NoError(t, err)
	assert.Equal(t, "new_password", manager.config.Users["test_user"].Password)
	assert.Equal(t, "admin", manager.config.Users["test_user"].Profile)

	// Test DeleteUser
	mock.ExpectExec("DROP USER IF EXISTS").WillReturnResult(sqlmock.NewResult(1, 1))

	err = userManager.DeleteUser(context.Background(), "test_user")
	require.NoError(t, err)
	assert.NotContains(t, manager.config.Users, "test_user")
}

func TestStorageManager(t *testing.T) {
	client, _ := setupMockDB(t)
	logger := setupTestLogger()
	manager := NewManager(client, logger)
	storageManager := NewStorageManager(manager)

	// Setup test storage tier
	testTier := StorageTierConfig{
		Name:     "test_disk",
		Type:     "disk",
		DiskType: "local",
		Path:     "/var/lib/clickhouse/data/test/",
	}

	// Add the tier to the config
	manager.config.StorageTiers["test_disk"] = testTier

	// GetStorageTier should return the tier
	tier, err := storageManager.GetStorageTier("test_disk")
	require.NoError(t, err)
	assert.Equal(t, testTier.Name, tier.Name)
	assert.Equal(t, testTier.Path, tier.Path)

	// ListStorageTiers should include the tier
	tiers := storageManager.ListStorageTiers()
	assert.Len(t, tiers, 1)
	assert.Equal(t, testTier.Name, tiers[0].Name)

	// Setup test storage policy
	testPolicy := StoragePolicyConfig{
		Name: "test_policy",
		Volumes: []Volume{
			{
				Name:  "main",
				Disks: []string{"test_disk"},
			},
		},
	}

	// Add the policy to the config
	manager.config.StoragePolicies["test_policy"] = testPolicy

	// GetStoragePolicy should return the policy
	policy, err := storageManager.GetStoragePolicy("test_policy")
	require.NoError(t, err)
	assert.Equal(t, testPolicy.Name, policy.Name)
	assert.Len(t, policy.Volumes, 1)
	assert.Equal(t, "main", policy.Volumes[0].Name)

	// ListStoragePolicies should include the policy
	policies := storageManager.ListStoragePolicies()
	assert.Len(t, policies, 1)
	assert.Equal(t, testPolicy.Name, policies[0].Name)

	// Test GenerateStorageConfig
	config, err := storageManager.GenerateStorageConfig()
	require.NoError(t, err)
	assert.Contains(t, config, "<test_disk>")
	assert.Contains(t, config, "<test_policy>")
	assert.Contains(t, config, "<main>")
}

func TestQuoteAndEscapeFunctions(t *testing.T) {
	// Test quoteIdentifier
	assert.Equal(t, "`test`", quoteIdentifier("test"))
	assert.Equal(t, "`test``table`", quoteIdentifier("test`table"))

	// Test quoteIdentifierList
	assert.Equal(t, []string{"`test`", "`table`"}, quoteIdentifierList([]string{"test", "table"}))

	// Test escapeString
	assert.Equal(t, "test", escapeString("test"))
	assert.Equal(t, "O''Reilly", escapeString("O'Reilly"))
}