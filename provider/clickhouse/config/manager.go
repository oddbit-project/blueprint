package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/log"
	"strings"
	"time"
)

// Manager manages ClickHouse configuration
type Manager struct {
	client *db.SqlClient
	logger *log.Logger
	config *ClickHouseConfig
}

// NewManager creates a new ClickHouse configuration manager
func NewManager(client *db.SqlClient, logger *log.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger,
		config: NewClickHouseConfig(),
	}
}

// LoadConfig loads the existing configuration from ClickHouse
func (m *Manager) LoadConfig(ctx context.Context) error {
	if err := m.loadUsers(ctx); err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}
	
	if err := m.loadProfiles(ctx); err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}
	
	if err := m.loadQuotas(ctx); err != nil {
		return fmt.Errorf("failed to load quotas: %w", err)
	}
	
	if err := m.loadDatabases(ctx); err != nil {
		return fmt.Errorf("failed to load databases: %w", err)
	}
	
	if err := m.loadStorageTiers(ctx); err != nil {
		return fmt.Errorf("failed to load storage tiers: %w", err)
	}
	
	if err := m.loadStoragePolicies(ctx); err != nil {
		return fmt.Errorf("failed to load storage policies: %w", err)
	}
	
	if err := m.loadRoles(ctx); err != nil {
		return fmt.Errorf("failed to load roles: %w", err)
	}
	
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *ClickHouseConfig {
	return m.config
}

// SetConfig sets the configuration
func (m *Manager) SetConfig(config *ClickHouseConfig) {
	m.config = config
}

// ExportConfig exports the configuration to JSON
func (m *Manager) ExportConfig() ([]byte, error) {
	return json.MarshalIndent(m.config, "", "  ")
}

// ImportConfig imports the configuration from JSON
func (m *Manager) ImportConfig(data []byte) error {
	return json.Unmarshal(data, m.config)
}

// ApplyConfig applies the configuration to the ClickHouse server
func (m *Manager) ApplyConfig(ctx context.Context) error {
	// Apply configuration in order of dependencies
	if err := m.applyStorageTiers(ctx); err != nil {
		return fmt.Errorf("failed to apply storage tiers: %w", err)
	}
	
	if err := m.applyStoragePolicies(ctx); err != nil {
		return fmt.Errorf("failed to apply storage policies: %w", err)
	}
	
	if err := m.applyProfiles(ctx); err != nil {
		return fmt.Errorf("failed to apply profiles: %w", err)
	}
	
	if err := m.applyQuotas(ctx); err != nil {
		return fmt.Errorf("failed to apply quotas: %w", err)
	}
	
	if err := m.applyRoles(ctx); err != nil {
		return fmt.Errorf("failed to apply roles: %w", err)
	}
	
	if err := m.applyUsers(ctx); err != nil {
		return fmt.Errorf("failed to apply users: %w", err)
	}
	
	if err := m.applyDatabases(ctx); err != nil {
		return fmt.Errorf("failed to apply databases: %w", err)
	}
	
	return nil
}

// loadUsers loads users from the ClickHouse server
func (m *Manager) loadUsers(ctx context.Context) error {
	query := `
		SELECT 
			name,
			storage_policy,
			readonly,
			allow_databases,
			allow_dictionaries,
			profile_name,
			quota_name,
			networks,
			settings
		FROM system.users
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.Users = make(map[string]UserConfig)
	
	for rows.Next() {
		var (
			name string
			storagePolicy, readonly, allowDatabases, allowDictionaries, profileName, quotaName, networks, settings sql.NullString
		)
		
		if err := rows.Scan(&name, &storagePolicy, &readonly, &allowDatabases, &allowDictionaries, &profileName, &quotaName, &networks, &settings); err != nil {
			return err
		}
		
		user := UserConfig{
			Name:     name,
			Settings: make(map[string]string),
		}
		
		if profileName.Valid {
			user.Profile = profileName.String
		}
		
		if quotaName.Valid {
			user.Quota = quotaName.String
		}
		
		if networks.Valid && networks.String != "" {
			user.Networks = strings.Split(networks.String, ",")
		}
		
		if allowDatabases.Valid && allowDatabases.String != "" {
			user.AllowDatabases = strings.Split(allowDatabases.String, ",")
		}
		
		if allowDictionaries.Valid && allowDictionaries.String != "" {
			user.AllowDictionary = strings.Split(allowDictionaries.String, ",")
		}
		
		// Parse settings
		if settings.Valid && settings.String != "" {
			var settingsMap map[string]string
			if err := json.Unmarshal([]byte(settings.String), &settingsMap); err == nil {
				user.Settings = settingsMap
			}
		}
		
		m.config.Users[name] = user
	}
	
	return rows.Err()
}

// loadProfiles loads profiles from the ClickHouse server
func (m *Manager) loadProfiles(ctx context.Context) error {
	query := `
		SELECT 
			name,
			readonly,
			settings
		FROM system.profiles
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.Profiles = make(map[string]ProfileConfig)
	
	for rows.Next() {
		var (
			name string
			readonly sql.NullInt32
			settings sql.NullString
		)
		
		if err := rows.Scan(&name, &readonly, &settings); err != nil {
			return err
		}
		
		profile := ProfileConfig{
			Name:     name,
			Settings: make(map[string]string),
		}
		
		if readonly.Valid {
			profile.ReadOnly = readonly.Int32 > 0
		}
		
		// Parse settings
		if settings.Valid && settings.String != "" {
			var settingsMap map[string]string
			if err := json.Unmarshal([]byte(settings.String), &settingsMap); err == nil {
				profile.Settings = settingsMap
			}
		}
		
		m.config.Profiles[name] = profile
	}
	
	return rows.Err()
}

// loadQuotas loads quotas from the ClickHouse server
func (m *Manager) loadQuotas(ctx context.Context) error {
	query := `
		SELECT 
			name,
			intervals,
			keys
		FROM system.quotas
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.Quotas = make(map[string]QuotaConfig)
	
	for rows.Next() {
		var (
			name string
			intervals, keys sql.NullString
		)
		
		if err := rows.Scan(&name, &intervals, &keys); err != nil {
			return err
		}
		
		quota := QuotaConfig{
			Name:      name,
			Intervals: []Interval{},
		}
		
		// Parse intervals
		if intervals.Valid && intervals.String != "" {
			var intervalsArray []map[string]interface{}
			if err := json.Unmarshal([]byte(intervals.String), &intervalsArray); err == nil {
				for _, i := range intervalsArray {
					interval := Interval{}
					
					if duration, ok := i["duration"].(float64); ok {
						interval.Duration = fromSeconds(duration)
					}
					
					if queries, ok := i["queries"].(float64); ok {
						interval.Queries = int(queries)
					}
					
					if errors, ok := i["errors"].(float64); ok {
						interval.Errors = int(errors)
					}
					
					if resultRows, ok := i["result_rows"].(float64); ok {
						interval.ResultRows = int(resultRows)
					}
					
					if readRows, ok := i["read_rows"].(float64); ok {
						interval.ReadRows = int(readRows)
					}
					
					if executionTime, ok := i["execution_time"].(float64); ok {
						interval.ExecutionTime = fromSeconds(executionTime)
					}
					
					quota.Intervals = append(quota.Intervals, interval)
				}
			}
		}
		
		m.config.Quotas[name] = quota
	}
	
	return rows.Err()
}

// loadDatabases loads databases from the ClickHouse server
func (m *Manager) loadDatabases(ctx context.Context) error {
	query := `
		SELECT 
			name,
			engine,
			data_path,
			metadata_path,
			uuid
		FROM system.databases
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.Databases = make(map[string]DatabaseConfig)
	
	for rows.Next() {
		var (
			name, engine string
			dataPath, metadataPath, uuid sql.NullString
		)
		
		if err := rows.Scan(&name, &engine, &dataPath, &metadataPath, &uuid); err != nil {
			return err
		}
		
		// Skip system databases
		if name == "system" || name == "information_schema" {
			continue
		}
		
		database := DatabaseConfig{
			Name:   name,
			Engine: engine,
		}
		
		m.config.Databases[name] = database
	}
	
	return rows.Err()
}

// loadStorageTiers loads storage tiers (disks) from the ClickHouse server
func (m *Manager) loadStorageTiers(ctx context.Context) error {
	query := `
		SELECT 
			name,
			type,
			path,
			free_space,
			total_space
		FROM system.disks
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.StorageTiers = make(map[string]StorageTierConfig)
	
	for rows.Next() {
		var (
			name, diskType, path string
			freeSpace, totalSpace uint64
		)
		
		if err := rows.Scan(&name, &diskType, &path, &freeSpace, &totalSpace); err != nil {
			return err
		}
		
		tier := StorageTierConfig{
			Name:     name,
			Type:     "disk", // system.disks only shows disks
			DiskType: diskType,
			Path:     path,
		}
		
		m.config.StorageTiers[name] = tier
	}
	
	return rows.Err()
}

// loadStoragePolicies loads storage policies from the ClickHouse server
func (m *Manager) loadStoragePolicies(ctx context.Context) error {
	query := `
		SELECT 
			policy_name,
			volume_name,
			volume_priority,
			volume_type,
			disks,
			max_data_part_size,
			move_factor,
			prefer_not_to_merge
		FROM system.storage_policies
		ORDER BY policy_name, volume_priority
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	m.config.StoragePolicies = make(map[string]StoragePolicyConfig)
	
	currentPolicy := ""
	var policy StoragePolicyConfig
	
	for rows.Next() {
		var (
			policyName, volumeName, volumeType string
			volumePriority int
			disks string
			maxDataPartSize, moveFactor sql.NullInt64
			preferNotToMerge sql.NullBool
		)
		
		if err := rows.Scan(&policyName, &volumeName, &volumePriority, &volumeType, &disks, &maxDataPartSize, &moveFactor, &preferNotToMerge); err != nil {
			return err
		}
		
		// If we encounter a new policy, store the previous one and create a new one
		if currentPolicy != policyName {
			if currentPolicy != "" {
				m.config.StoragePolicies[currentPolicy] = policy
			}
			
			currentPolicy = policyName
			policy = StoragePolicyConfig{
				Name:    policyName,
				Volumes: []Volume{},
			}
		}
		
		volume := Volume{
			Name:  volumeName,
			Disks: strings.Split(disks, ","),
		}
		
		if maxDataPartSize.Valid {
			volume.MaxDataPartSizeBytes = maxDataPartSize.Int64
		}
		
		if preferNotToMerge.Valid {
			volume.PreferNotToMerge = preferNotToMerge.Bool
		}
		
		policy.Volumes = append(policy.Volumes, volume)
	}
	
	// Store the last policy
	if currentPolicy != "" {
		m.config.StoragePolicies[currentPolicy] = policy
	}
	
	return rows.Err()
}

// loadRoles loads roles from the ClickHouse server
func (m *Manager) loadRoles(ctx context.Context) error {
	// Check if roles are supported before querying
	if !m.supportsRoles(ctx) {
		// Roles are not supported in this ClickHouse version
		m.config.Roles = make(map[string]RoleConfig)
		return nil
	}

	query := `
		SELECT 
			name,
			settings,
			grants
		FROM system.roles
	`
	
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		if isTableNotFoundError(err) {
			// system.roles might not exist in older ClickHouse versions
			m.config.Roles = make(map[string]RoleConfig)
			return nil
		}
		return err
	}
	defer rows.Close()
	
	m.config.Roles = make(map[string]RoleConfig)
	
	for rows.Next() {
		var (
			name string
			settings, grants sql.NullString
		)
		
		if err := rows.Scan(&name, &settings, &grants); err != nil {
			return err
		}
		
		role := RoleConfig{
			Name:     name,
			Settings: make(map[string]string),
		}
		
		// Parse settings
		if settings.Valid && settings.String != "" {
			var settingsMap map[string]string
			if err := json.Unmarshal([]byte(settings.String), &settingsMap); err == nil {
				role.Settings = settingsMap
			}
		}
		
		// Parse grants
		if grants.Valid && grants.String != "" {
			var grantsArray []string
			if err := json.Unmarshal([]byte(grants.String), &grantsArray); err == nil {
				role.Grants = grantsArray
			}
		}
		
		m.config.Roles[name] = role
	}
	
	return rows.Err()
}

// applyUsers applies user configuration to the ClickHouse server
func (m *Manager) applyUsers(ctx context.Context) error {
	// First, load existing users to compare
	existingUsers := make(map[string]struct{})
	
	query := "SELECT name FROM system.users"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingUsers[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each user configuration
	for _, user := range m.config.Users {
		// If the user already exists, update it
		if _, exists := existingUsers[user.Name]; exists {
			if err := m.updateUser(ctx, user); err != nil {
				return err
			}
		} else {
			// Otherwise, create the user
			if err := m.createUser(ctx, user); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// createUser creates a new user in ClickHouse
func (m *Manager) createUser(ctx context.Context, user UserConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("CREATE USER IF NOT EXISTS %s", quoteIdentifier(user.Name)))
	
	// Add authentication
	if user.Password != "" {
		query.WriteString(fmt.Sprintf(" IDENTIFIED WITH plaintext_password BY '%s'", escapeString(user.Password)))
	} else if user.HashedPassword != "" {
		query.WriteString(fmt.Sprintf(" IDENTIFIED WITH sha256_password BY '%s'", escapeString(user.HashedPassword)))
	}
	
	// Add host restrictions
	if len(user.Networks) > 0 {
		query.WriteString(" HOST ")
		for i, network := range user.Networks {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("'%s'", escapeString(network)))
		}
	}
	
	// Add profile and quota
	if user.Profile != "" {
		query.WriteString(fmt.Sprintf(" DEFAULT ROLE %s", quoteIdentifier(user.Profile)))
	}
	
	if user.Quota != "" {
		query.WriteString(fmt.Sprintf(" QUOTA %s", quoteIdentifier(user.Quota)))
	}
	
	// Set user settings
	if len(user.Settings) > 0 {
		query.WriteString(" SETTINGS ")
		i := 0
		for key, value := range user.Settings {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("%s = '%s'", key, escapeString(value)))
			i++
		}
	}
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", user.Name, err)
	}
	
	// Apply database and dictionary restrictions
	if len(user.AllowDatabases) > 0 {
		allowQuery := fmt.Sprintf("GRANT SHOW, SELECT ON %s.* TO %s", 
			strings.Join(quoteIdentifierList(user.AllowDatabases), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, allowQuery); err != nil {
			return fmt.Errorf("failed to grant database permissions to user %s: %w", user.Name, err)
		}
	}
	
	if len(user.DenyDatabases) > 0 {
		denyQuery := fmt.Sprintf("REVOKE ALL ON %s.* FROM %s", 
			strings.Join(quoteIdentifierList(user.DenyDatabases), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, denyQuery); err != nil {
			return fmt.Errorf("failed to revoke database permissions from user %s: %w", user.Name, err)
		}
	}
	
	// Apply roles
	if len(user.Roles) > 0 {
		rolesQuery := fmt.Sprintf("GRANT %s TO %s", 
			strings.Join(quoteIdentifierList(user.Roles), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, rolesQuery); err != nil {
			return fmt.Errorf("failed to grant roles to user %s: %w", user.Name, err)
		}
	}
	
	return nil
}

// updateUser updates an existing user in ClickHouse
func (m *Manager) updateUser(ctx context.Context, user UserConfig) error {
	// First, alter the user's authentication and settings
	var alterQuery strings.Builder
	
	alterQuery.WriteString(fmt.Sprintf("ALTER USER %s", quoteIdentifier(user.Name)))
	
	// Update authentication if provided
	if user.Password != "" {
		alterQuery.WriteString(fmt.Sprintf(" IDENTIFIED WITH plaintext_password BY '%s'", escapeString(user.Password)))
	} else if user.HashedPassword != "" {
		alterQuery.WriteString(fmt.Sprintf(" IDENTIFIED WITH sha256_password BY '%s'", escapeString(user.HashedPassword)))
	}
	
	// Update host restrictions
	if len(user.Networks) > 0 {
		alterQuery.WriteString(" HOST ")
		for i, network := range user.Networks {
			if i > 0 {
				alterQuery.WriteString(", ")
			}
			alterQuery.WriteString(fmt.Sprintf("'%s'", escapeString(network)))
		}
	}
	
	// Update profile and quota
	if user.Profile != "" {
		alterQuery.WriteString(fmt.Sprintf(" DEFAULT ROLE %s", quoteIdentifier(user.Profile)))
	}
	
	if user.Quota != "" {
		alterQuery.WriteString(fmt.Sprintf(" QUOTA %s", quoteIdentifier(user.Quota)))
	}
	
	// Update user settings
	if len(user.Settings) > 0 {
		alterQuery.WriteString(" SETTINGS ")
		i := 0
		for key, value := range user.Settings {
			if i > 0 {
				alterQuery.WriteString(", ")
			}
			alterQuery.WriteString(fmt.Sprintf("%s = '%s'", key, escapeString(value)))
			i++
		}
	}
	
	// Execute the alter query
	_, err := m.client.Conn.ExecContext(ctx, alterQuery.String())
	if err != nil {
		return fmt.Errorf("failed to update user %s: %w", user.Name, err)
	}
	
	// Revoke existing database and dictionary restrictions
	_, err = m.client.Conn.ExecContext(ctx, fmt.Sprintf("REVOKE ALL ON *.* FROM %s", quoteIdentifier(user.Name)))
	if err != nil {
		return fmt.Errorf("failed to revoke permissions from user %s: %w", user.Name, err)
	}
	
	// Apply new database and dictionary restrictions
	if len(user.AllowDatabases) > 0 {
		allowQuery := fmt.Sprintf("GRANT SHOW, SELECT ON %s.* TO %s", 
			strings.Join(quoteIdentifierList(user.AllowDatabases), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, allowQuery); err != nil {
			return fmt.Errorf("failed to grant database permissions to user %s: %w", user.Name, err)
		}
	}
	
	if len(user.DenyDatabases) > 0 {
		denyQuery := fmt.Sprintf("REVOKE ALL ON %s.* FROM %s", 
			strings.Join(quoteIdentifierList(user.DenyDatabases), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, denyQuery); err != nil {
			return fmt.Errorf("failed to revoke database permissions from user %s: %w", user.Name, err)
		}
	}
	
	// Revoke existing roles
	_, err = m.client.Conn.ExecContext(ctx, fmt.Sprintf("REVOKE ALL ROLES FROM %s", quoteIdentifier(user.Name)))
	if err != nil {
		return fmt.Errorf("failed to revoke roles from user %s: %w", user.Name, err)
	}
	
	// Apply new roles
	if len(user.Roles) > 0 {
		rolesQuery := fmt.Sprintf("GRANT %s TO %s", 
			strings.Join(quoteIdentifierList(user.Roles), ", "), 
			quoteIdentifier(user.Name))
		if _, err := m.client.Conn.ExecContext(ctx, rolesQuery); err != nil {
			return fmt.Errorf("failed to grant roles to user %s: %w", user.Name, err)
		}
	}
	
	return nil
}

// applyProfiles applies profile configuration to the ClickHouse server
func (m *Manager) applyProfiles(ctx context.Context) error {
	// First, load existing profiles to compare
	existingProfiles := make(map[string]struct{})
	
	query := "SELECT name FROM system.profiles"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingProfiles[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each profile configuration
	for _, profile := range m.config.Profiles {
		// If the profile already exists, update it
		if _, exists := existingProfiles[profile.Name]; exists {
			if err := m.updateProfile(ctx, profile); err != nil {
				return err
			}
		} else {
			// Otherwise, create the profile
			if err := m.createProfile(ctx, profile); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// createProfile creates a new profile in ClickHouse
func (m *Manager) createProfile(ctx context.Context, profile ProfileConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("CREATE SETTINGS PROFILE IF NOT EXISTS %s", quoteIdentifier(profile.Name)))
	
	// Add settings
	if len(profile.Settings) > 0 {
		query.WriteString(" SETTINGS ")
		i := 0
		for key, value := range profile.Settings {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("%s = %s", key, value))
			i++
		}
	}
	
	// Add readonly flag
	if profile.ReadOnly {
		query.WriteString(" READONLY = 1")
	}
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to create profile %s: %w", profile.Name, err)
	}
	
	return nil
}

// updateProfile updates an existing profile in ClickHouse
func (m *Manager) updateProfile(ctx context.Context, profile ProfileConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("ALTER SETTINGS PROFILE %s", quoteIdentifier(profile.Name)))
	
	// Add settings
	if len(profile.Settings) > 0 {
		query.WriteString(" SETTINGS ")
		i := 0
		for key, value := range profile.Settings {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("%s = %s", key, value))
			i++
		}
	}
	
	// Add readonly flag
	if profile.ReadOnly {
		query.WriteString(" READONLY = 1")
	} else {
		query.WriteString(" READONLY = 0")
	}
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to update profile %s: %w", profile.Name, err)
	}
	
	return nil
}

// applyQuotas applies quota configuration to the ClickHouse server
func (m *Manager) applyQuotas(ctx context.Context) error {
	// First, load existing quotas to compare
	existingQuotas := make(map[string]struct{})
	
	query := "SELECT name FROM system.quotas"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingQuotas[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each quota configuration
	for _, quota := range m.config.Quotas {
		// If the quota already exists, update it
		if _, exists := existingQuotas[quota.Name]; exists {
			if err := m.updateQuota(ctx, quota); err != nil {
				return err
			}
		} else {
			// Otherwise, create the quota
			if err := m.createQuota(ctx, quota); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// createQuota creates a new quota in ClickHouse
func (m *Manager) createQuota(ctx context.Context, quota QuotaConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("CREATE QUOTA IF NOT EXISTS %s", quoteIdentifier(quota.Name)))
	
	// Add intervals
	if len(quota.Intervals) > 0 {
		for _, interval := range quota.Intervals {
			query.WriteString(fmt.Sprintf(" FOR INTERVAL %d SECOND", int(interval.Duration.Seconds())))
			
			limitParts := []string{}
			
			if interval.Queries > 0 {
				limitParts = append(limitParts, fmt.Sprintf("MAX QUERIES %d", interval.Queries))
			}
			
			if interval.Errors > 0 {
				limitParts = append(limitParts, fmt.Sprintf("MAX ERRORS %d", interval.Errors))
			}
			
			if interval.ResultRows > 0 {
				limitParts = append(limitParts, fmt.Sprintf("MAX RESULT ROWS %d", interval.ResultRows))
			}
			
			if interval.ReadRows > 0 {
				limitParts = append(limitParts, fmt.Sprintf("MAX READ ROWS %d", interval.ReadRows))
			}
			
			if interval.ExecutionTime > 0 {
				limitParts = append(limitParts, fmt.Sprintf("MAX EXECUTION TIME %d", int(interval.ExecutionTime.Seconds())))
			}
			
			if len(limitParts) > 0 {
				query.WriteString(" " + strings.Join(limitParts, " "))
			}
		}
	}
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to create quota %s: %w", quota.Name, err)
	}
	
	return nil
}

// updateQuota updates an existing quota in ClickHouse
func (m *Manager) updateQuota(ctx context.Context, quota QuotaConfig) error {
	// First drop the existing quota
	dropQuery := fmt.Sprintf("DROP QUOTA IF EXISTS %s", quoteIdentifier(quota.Name))
	_, err := m.client.Conn.ExecContext(ctx, dropQuery)
	if err != nil {
		return fmt.Errorf("failed to drop quota %s: %w", quota.Name, err)
	}
	
	// Then create it again
	return m.createQuota(ctx, quota)
}

// applyStorageTiers applies storage tier configuration to the ClickHouse server
func (m *Manager) applyStorageTiers(ctx context.Context) error {
	// First, load existing disks to compare
	existingDisks := make(map[string]struct{})
	
	query := "SELECT name FROM system.disks"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingDisks[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each storage tier configuration
	for _, tier := range m.config.StorageTiers {
		// Skip if the disk already exists (disks can only be configured in config files)
		if _, exists := existingDisks[tier.Name]; exists {
			continue
		}
		
		m.logger.Info("Storage tier exists in configuration but not on server", log.KV{"tier": tier.Name})
	}
	
	return nil
}

// applyStoragePolicies applies storage policy configuration to the ClickHouse server
func (m *Manager) applyStoragePolicies(ctx context.Context) error {
	// Storage policies cannot be created through SQL, only through XML configuration
	// Log warning for policies in config but not on server
	
	// First, load existing policies to compare
	existingPolicies := make(map[string]struct{})
	
	query := "SELECT DISTINCT policy_name FROM system.storage_policies"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingPolicies[name] = struct{}{}
	}
	rows.Close()
	
	// Check each policy in the configuration
	for name := range m.config.StoragePolicies {
		if _, exists := existingPolicies[name]; !exists {
			m.logger.Info("Storage policy exists in configuration but not on server", log.KV{"policy": name})
		}
	}
	
	return nil
}

// applyRoles applies role configuration to the ClickHouse server
func (m *Manager) applyRoles(ctx context.Context) error {
	// Check if roles are supported
	if !m.supportsRoles(ctx) {
		m.logger.Info("Roles are not supported in this ClickHouse version", log.KV{})
		return nil
	}
	
	// First, load existing roles to compare
	existingRoles := make(map[string]struct{})
	
	query := "SELECT name FROM system.roles"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		if isTableNotFoundError(err) {
			// system.roles might not exist in older ClickHouse versions
			return nil
		}
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingRoles[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each role configuration
	for _, role := range m.config.Roles {
		// If the role already exists, update it
		if _, exists := existingRoles[role.Name]; exists {
			if err := m.updateRole(ctx, role); err != nil {
				return err
			}
		} else {
			// Otherwise, create the role
			if err := m.createRole(ctx, role); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// createRole creates a new role in ClickHouse
func (m *Manager) createRole(ctx context.Context, role RoleConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("CREATE ROLE IF NOT EXISTS %s", quoteIdentifier(role.Name)))
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to create role %s: %w", role.Name, err)
	}
	
	// Apply settings
	if len(role.Settings) > 0 {
		var settingsQuery strings.Builder
		
		settingsQuery.WriteString(fmt.Sprintf("ALTER ROLE %s SETTINGS ", quoteIdentifier(role.Name)))
		i := 0
		for key, value := range role.Settings {
			if i > 0 {
				settingsQuery.WriteString(", ")
			}
			settingsQuery.WriteString(fmt.Sprintf("%s = %s", key, value))
			i++
		}
		
		_, err = m.client.Conn.ExecContext(ctx, settingsQuery.String())
		if err != nil {
			return fmt.Errorf("failed to set settings for role %s: %w", role.Name, err)
		}
	}
	
	// Apply grants
	for _, grant := range role.Grants {
		grantQuery := fmt.Sprintf("GRANT %s TO %s", grant, quoteIdentifier(role.Name))
		_, err = m.client.Conn.ExecContext(ctx, grantQuery)
		if err != nil {
			return fmt.Errorf("failed to grant permission '%s' to role %s: %w", grant, role.Name, err)
		}
	}
	
	return nil
}

// updateRole updates an existing role in ClickHouse
func (m *Manager) updateRole(ctx context.Context, role RoleConfig) error {
	// Revoke existing grants
	_, err := m.client.Conn.ExecContext(ctx, fmt.Sprintf("REVOKE ALL ON *.* FROM %s", quoteIdentifier(role.Name)))
	if err != nil {
		return fmt.Errorf("failed to revoke permissions from role %s: %w", role.Name, err)
	}
	
	// Apply settings
	if len(role.Settings) > 0 {
		var settingsQuery strings.Builder
		
		settingsQuery.WriteString(fmt.Sprintf("ALTER ROLE %s SETTINGS ", quoteIdentifier(role.Name)))
		i := 0
		for key, value := range role.Settings {
			if i > 0 {
				settingsQuery.WriteString(", ")
			}
			settingsQuery.WriteString(fmt.Sprintf("%s = %s", key, value))
			i++
		}
		
		_, err = m.client.Conn.ExecContext(ctx, settingsQuery.String())
		if err != nil {
			return fmt.Errorf("failed to set settings for role %s: %w", role.Name, err)
		}
	}
	
	// Apply grants
	for _, grant := range role.Grants {
		grantQuery := fmt.Sprintf("GRANT %s TO %s", grant, quoteIdentifier(role.Name))
		_, err = m.client.Conn.ExecContext(ctx, grantQuery)
		if err != nil {
			return fmt.Errorf("failed to grant permission '%s' to role %s: %w", grant, role.Name, err)
		}
	}
	
	return nil
}

// applyDatabases applies database configuration to the ClickHouse server
func (m *Manager) applyDatabases(ctx context.Context) error {
	// First, load existing databases to compare
	existingDatabases := make(map[string]struct{})
	
	query := "SELECT name FROM system.databases WHERE name NOT IN ('system', 'information_schema')"
	rows, err := m.client.Conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		existingDatabases[name] = struct{}{}
	}
	rows.Close()
	
	// Apply each database configuration
	for _, database := range m.config.Databases {
		// If the database already exists, update it
		if _, exists := existingDatabases[database.Name]; exists {
			if err := m.updateDatabase(ctx, database); err != nil {
				return err
			}
		} else {
			// Otherwise, create the database
			if err := m.createDatabase(ctx, database); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// createDatabase creates a new database in ClickHouse
func (m *Manager) createDatabase(ctx context.Context, database DatabaseConfig) error {
	var query strings.Builder
	
	query.WriteString(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteIdentifier(database.Name)))
	
	// Add engine
	if database.Engine != "" {
		query.WriteString(fmt.Sprintf(" ENGINE = %s", database.Engine))
	}
	
	// Add comment if provided
	if database.Comment != "" {
		query.WriteString(fmt.Sprintf(" COMMENT '%s'", escapeString(database.Comment)))
	}
	
	// Execute the query
	_, err := m.client.Conn.ExecContext(ctx, query.String())
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", database.Name, err)
	}
	
	// Apply permissions if provided
	if len(database.AllowedUsers) > 0 {
		allowQuery := fmt.Sprintf("GRANT ALL ON %s.* TO %s", 
			quoteIdentifier(database.Name), 
			strings.Join(quoteIdentifierList(database.AllowedUsers), ", "))
		if _, err := m.client.Conn.ExecContext(ctx, allowQuery); err != nil {
			return fmt.Errorf("failed to grant permissions on database %s: %w", database.Name, err)
		}
	}
	
	if len(database.AllowedRoles) > 0 {
		allowRolesQuery := fmt.Sprintf("GRANT ALL ON %s.* TO %s", 
			quoteIdentifier(database.Name), 
			strings.Join(quoteIdentifierList(database.AllowedRoles), ", "))
		if _, err := m.client.Conn.ExecContext(ctx, allowRolesQuery); err != nil {
			return fmt.Errorf("failed to grant role permissions on database %s: %w", database.Name, err)
		}
	}
	
	return nil
}

// updateDatabase updates an existing database in ClickHouse
func (m *Manager) updateDatabase(ctx context.Context, database DatabaseConfig) error {
	// Not much can be updated for a database through SQL
	// Just update permissions
	
	// Revoke existing permissions
	_, err := m.client.Conn.ExecContext(ctx, fmt.Sprintf("REVOKE ALL ON %s.* FROM ALL", quoteIdentifier(database.Name)))
	if err != nil {
		return fmt.Errorf("failed to revoke permissions on database %s: %w", database.Name, err)
	}
	
	// Apply new permissions
	if len(database.AllowedUsers) > 0 {
		allowQuery := fmt.Sprintf("GRANT ALL ON %s.* TO %s", 
			quoteIdentifier(database.Name), 
			strings.Join(quoteIdentifierList(database.AllowedUsers), ", "))
		if _, err := m.client.Conn.ExecContext(ctx, allowQuery); err != nil {
			return fmt.Errorf("failed to grant permissions on database %s: %w", database.Name, err)
		}
	}
	
	if len(database.AllowedRoles) > 0 {
		allowRolesQuery := fmt.Sprintf("GRANT ALL ON %s.* TO %s", 
			quoteIdentifier(database.Name), 
			strings.Join(quoteIdentifierList(database.AllowedRoles), ", "))
		if _, err := m.client.Conn.ExecContext(ctx, allowRolesQuery); err != nil {
			return fmt.Errorf("failed to grant role permissions on database %s: %w", database.Name, err)
		}
	}
	
	return nil
}

// supportsRoles checks if the ClickHouse server supports roles
func (m *Manager) supportsRoles(ctx context.Context) bool {
	query := "SELECT 1 FROM system.tables WHERE database = 'system' AND name = 'roles' LIMIT 1"
	var exists int
	err := m.client.Conn.QueryRowContext(ctx, query).Scan(&exists)
	// If we got an error or no result, roles are not supported
	if err != nil || exists != 1 {
		return false
	}
	return true
}

// Helper functions

// isTableNotFoundError checks if the error is a table not found error
func isTableNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Table system.roles doesn't exist")
}

// fromSeconds converts seconds to duration
func fromSeconds(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

// quoteIdentifier quotes a SQL identifier
func quoteIdentifier(name string) string {
	return "`" + strings.Replace(name, "`", "``", -1) + "`"
}

// quoteIdentifierList quotes a list of SQL identifiers
func quoteIdentifierList(names []string) []string {
	result := make([]string, len(names))
	for i, name := range names {
		result[i] = quoteIdentifier(name)
	}
	return result
}

// escapeString escapes a string for SQL
func escapeString(s string) string {
	return strings.Replace(s, "'", "''", -1)
}