package config

import (
	"context"
	"fmt"
)

// UserManager provides convenience methods for user management
type UserManager struct {
	manager *Manager
}

// NewUserManager creates a new user manager
func NewUserManager(manager *Manager) *UserManager {
	return &UserManager{
		manager: manager,
	}
}

// CreateUser creates a new user with the given configuration
func (um *UserManager) CreateUser(ctx context.Context, user UserConfig) error {
	if err := ValidateName(user.Name); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	// Check if the user already exists
	if _, exists := um.manager.config.Users[user.Name]; exists {
		return fmt.Errorf("user %s already exists", user.Name)
	}
	
	// Add the user to the configuration
	um.manager.config.Users[user.Name] = user
	
	// Apply the user configuration to the server
	return um.manager.createUser(ctx, user)
}

// UpdateUser updates an existing user with the given configuration
func (um *UserManager) UpdateUser(ctx context.Context, user UserConfig) error {
	if err := ValidateName(user.Name); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	// Check if the user exists
	if _, exists := um.manager.config.Users[user.Name]; !exists {
		return fmt.Errorf("user %s does not exist", user.Name)
	}
	
	// Update the user in the configuration
	um.manager.config.Users[user.Name] = user
	
	// Apply the user configuration to the server
	return um.manager.updateUser(ctx, user)
}

// DeleteUser deletes a user with the given name
func (um *UserManager) DeleteUser(ctx context.Context, name string) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	// Check if the user exists
	if _, exists := um.manager.config.Users[name]; !exists {
		return fmt.Errorf("user %s does not exist", name)
	}
	
	// Delete the user from the configuration
	delete(um.manager.config.Users, name)
	
	// Delete the user from the server
	query := fmt.Sprintf("DROP USER IF EXISTS %s", quoteIdentifier(name))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete user %s: %w", name, err)
	}
	
	return nil
}

// GetUser returns a user with the given name
func (um *UserManager) GetUser(name string) (UserConfig, error) {
	if err := ValidateName(name); err != nil {
		return UserConfig{}, fmt.Errorf("invalid user name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[name]
	if !exists {
		return UserConfig{}, fmt.Errorf("user %s does not exist", name)
	}
	
	return user, nil
}

// ListUsers returns a list of all users
func (um *UserManager) ListUsers() []UserConfig {
	users := make([]UserConfig, 0, len(um.manager.config.Users))
	for _, user := range um.manager.config.Users {
		users = append(users, user)
	}
	return users
}

// AddUserRole adds a role to a user
func (um *UserManager) AddUserRole(ctx context.Context, userName, roleName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(roleName); err != nil {
		return fmt.Errorf("invalid role name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Check if the role is already assigned
	for _, role := range user.Roles {
		if role == roleName {
			return nil // Role already assigned
		}
	}
	
	// Add the role
	user.Roles = append(user.Roles, roleName)
	um.manager.config.Users[userName] = user
	
	// Apply the role assignment to the server
	query := fmt.Sprintf("GRANT %s TO %s", 
		quoteIdentifier(roleName), 
		quoteIdentifier(userName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to add role %s to user %s: %w", roleName, userName, err)
	}
	
	return nil
}

// RemoveUserRole removes a role from a user
func (um *UserManager) RemoveUserRole(ctx context.Context, userName, roleName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(roleName); err != nil {
		return fmt.Errorf("invalid role name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Check if the role is assigned
	roleIndex := -1
	for i, role := range user.Roles {
		if role == roleName {
			roleIndex = i
			break
		}
	}
	
	if roleIndex == -1 {
		return nil // Role not assigned
	}
	
	// Remove the role
	user.Roles = append(user.Roles[:roleIndex], user.Roles[roleIndex+1:]...)
	um.manager.config.Users[userName] = user
	
	// Apply the role removal to the server
	query := fmt.Sprintf("REVOKE %s FROM %s", 
		quoteIdentifier(roleName), 
		quoteIdentifier(userName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to remove role %s from user %s: %w", roleName, userName, err)
	}
	
	return nil
}

// SetUserPassword sets the password for a user
func (um *UserManager) SetUserPassword(ctx context.Context, userName, password string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Update the password
	user.Password = password
	user.HashedPassword = "" // Clear hashed password if set
	um.manager.config.Users[userName] = user
	
	// Apply the password change to the server
	query := fmt.Sprintf("ALTER USER %s IDENTIFIED WITH plaintext_password BY '%s'", 
		quoteIdentifier(userName), 
		escapeString(password))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set password for user %s: %w", userName, err)
	}
	
	return nil
}

// SetUserQuota sets the quota for a user
func (um *UserManager) SetUserQuota(ctx context.Context, userName, quotaName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(quotaName); err != nil {
		return fmt.Errorf("invalid quota name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Update the quota
	user.Quota = quotaName
	um.manager.config.Users[userName] = user
	
	// Apply the quota change to the server
	query := fmt.Sprintf("ALTER USER %s QUOTA %s", 
		quoteIdentifier(userName), 
		quoteIdentifier(quotaName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set quota for user %s: %w", userName, err)
	}
	
	return nil
}

// SetUserProfile sets the profile for a user
func (um *UserManager) SetUserProfile(ctx context.Context, userName, profileName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(profileName); err != nil {
		return fmt.Errorf("invalid profile name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Update the profile
	user.Profile = profileName
	um.manager.config.Users[userName] = user
	
	// Apply the profile change to the server
	query := fmt.Sprintf("ALTER USER %s DEFAULT ROLE %s", 
		quoteIdentifier(userName), 
		quoteIdentifier(profileName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set profile for user %s: %w", userName, err)
	}
	
	return nil
}

// AddUserAllowedDatabase adds a database to the list of allowed databases for a user
func (um *UserManager) AddUserAllowedDatabase(ctx context.Context, userName, databaseName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(databaseName); err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Check if the database is already allowed
	for _, db := range user.AllowDatabases {
		if db == databaseName {
			return nil // Database already allowed
		}
	}
	
	// Add the database
	user.AllowDatabases = append(user.AllowDatabases, databaseName)
	um.manager.config.Users[userName] = user
	
	// Apply the permission to the server
	query := fmt.Sprintf("GRANT SHOW, SELECT ON %s.* TO %s", 
		quoteIdentifier(databaseName), 
		quoteIdentifier(userName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to grant database permissions to user %s: %w", userName, err)
	}
	
	return nil
}

// RemoveUserAllowedDatabase removes a database from the list of allowed databases for a user
func (um *UserManager) RemoveUserAllowedDatabase(ctx context.Context, userName, databaseName string) error {
	if err := ValidateName(userName); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := ValidateName(databaseName); err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}
	
	// Check if the user exists
	user, exists := um.manager.config.Users[userName]
	if !exists {
		return fmt.Errorf("user %s does not exist", userName)
	}
	
	// Check if the database is allowed
	dbIndex := -1
	for i, db := range user.AllowDatabases {
		if db == databaseName {
			dbIndex = i
			break
		}
	}
	
	if dbIndex == -1 {
		return nil // Database not allowed
	}
	
	// Remove the database
	user.AllowDatabases = append(user.AllowDatabases[:dbIndex], user.AllowDatabases[dbIndex+1:]...)
	um.manager.config.Users[userName] = user
	
	// Apply the permission removal to the server
	query := fmt.Sprintf("REVOKE ALL ON %s.* FROM %s", 
		quoteIdentifier(databaseName), 
		quoteIdentifier(userName))
	_, err := um.manager.client.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to revoke database permissions from user %s: %w", userName, err)
	}
	
	return nil
}