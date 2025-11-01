package htpasswd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

// Entry represents a single htpasswd entry
type Entry struct {
	Username string
	Hash     string
}

type Container struct {
	entries map[string]*Entry
	mutex   sync.RWMutex
}

// NewContainer creates a new htpasswd container
func NewContainer() *Container {
	return &Container{
		entries: make(map[string]*Entry),
	}
}

// NewFromFile creates a container from a file
func NewFromFile(path string) (*Container, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return NewFromReader(f)
}

// NewFromReader creates a container from a reader
func NewFromReader(src io.Reader) (*Container, error) {
	container := NewContainer()
	return container, container.Read(src)
}

// Load reads and parses an htpasswd source
func (c *Container) Read(src io.Reader) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	scanner := bufio.NewScanner(src)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse username:hash format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format at line %d: %s", lineNum, line)
		}

		username := strings.TrimSpace(parts[0])
		hash := strings.TrimSpace(parts[1])

		if username == "" {
			return fmt.Errorf("empty username at line %d", lineNum)
		}

		c.entries[username] = &Entry{
			Username: username,
			Hash:     hash,
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

// Save writes the htpasswd file
func (c *Container) Write(dest io.Writer) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	writer := bufio.NewWriter(dest)
	defer writer.Flush()

	for _, entry := range c.entries {
		line := fmt.Sprintf("%s:%s\n", entry.Username, entry.Hash)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write entry: %w", err)
		}
	}

	return nil
}

// AddUser adds or updates a user entry
func (c *Container) AddUser(username, hash string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if err := ValidateUsername(username); err != nil {
		return err
	}

	if err := ValidatePassword(hash); err != nil {
		return err
	}

	c.entries[username] = &Entry{
		Username: username,
		Hash:     hash,
	}

	return nil
}

// DeleteUser removes a user entry
func (c *Container) DeleteUser(username string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.entries[username]; !exists {
		return fmt.Errorf("user %s not found", username)
	}

	delete(c.entries, username)
	return nil
}

// GetUser retrieves a user entry
func (c *Container) GetUser(username string) (*Entry, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[username]
	if !exists {
		return nil, fmt.Errorf("user %s not found", username)
	}

	return &Entry{
		Username: entry.Username,
		Hash:     entry.Hash,
	}, nil
}

// ListUsers returns all usernames
func (c *Container) ListUsers() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	users := make([]string, 0, len(c.entries))
	for username := range c.entries {
		users = append(users, username)
	}

	return users
}

// UserExists checks if a user exists
func (c *Container) UserExists(username string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	_, exists := c.entries[username]
	return exists
}

// Count returns the number of users
func (c *Container) Count() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.entries)
}

// VerifyUser validate a user password
func (c *Container) VerifyUser(username string, password string) (bool, error) {
	if err := ValidateUsername(username); err != nil {
		return false, fmt.Errorf("invalid username: %w", err)
	}

	if err := ValidatePassword(password); err != nil {
		return false, fmt.Errorf("invalid password: %w", err)
	}

	entry, err := c.GetUser(username)
	if err != nil {
		return false, err
	}

	return VerifyPassword(password, entry.Hash), nil
}

// AddUserWithHash adds a user and password
func (c *Container) AddUserWithHash(username string, password string, hashType HashType) error {
	hash, err := HashPassword(password, hashType)
	if err != nil {
		return err
	}
	if err := c.AddUser(username, hash); err != nil {
		return err
	}
	return nil
}

// AddUserPassword adds a user and password with bcrypt
func (c *Container) AddUserPassword(username string, password string) error {
	hash, err := HashPassword(password, HashTypeBcrypt)
	if err != nil {
		return err
	}
	if err := c.AddUser(username, hash); err != nil {
		return err
	}
	return nil
}

// ValidateUsername validates a username according to htpasswd rules
func ValidateUsername(username string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Check for colon character (forbidden in htpasswd format)
	if strings.Contains(username, ":") {
		return fmt.Errorf("username cannot contain colon (:) character")
	}

	// Check length (255 bytes maximum)
	if len(username) > 255 {
		return fmt.Errorf("username cannot exceed 255 bytes (got %d)", len(username))
	}

	// Check for valid UTF-8 encoding
	if !utf8.ValidString(username) {
		return fmt.Errorf("username must be valid UTF-8")
	}

	return nil
}

// ValidatePassword validates a password
func ValidatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Check for valid UTF-8 encoding
	if !utf8.ValidString(password) {
		return fmt.Errorf("password must be valid UTF-8")
	}

	return nil
}
