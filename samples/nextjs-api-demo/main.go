package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

type userRecord struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UserStore struct {
	data   []userRecord
	mx     sync.RWMutex
	lastId atomic.Int32
}

func newUserStore() *UserStore {
	result := &UserStore{
		data: make([]userRecord, 0),
		mx:   sync.RWMutex{},
	}
	result.Provide()
	return result
}

func (s *UserStore) Provide() {
	s.AddUser("Alice", "alice@example.com", "admin")
	s.AddUser("Bob", "bob@example.com", "user")
	s.AddUser("Charlie", "charlie@example.com", "user")
}

func (s *UserStore) AddUser(name string, email string, role string) userRecord {
	s.lastId.Add(1)
	record := userRecord{
		ID:    int(s.lastId.Load()),
		Name:  name,
		Email: email,
		Role:  role,
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	s.data = append(s.data, record)
	return record
}

func (s *UserStore) UpdateUser(id int, name *string, email *string, role *string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			if name != nil {
				s.data[i].Name = *name
			}
			if email != nil {
				s.data[i].Email = *email
			}
			if role != nil {
				s.data[i].Role = *role
			}
			return
		}
	}
}

func (s *UserStore) DeleteUser(id int) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return nil
		}
	}
	return errors.New("user not found")
}

func (s *UserStore) GetUsers() ([]userRecord, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()
	result := make([]userRecord, 0)
	for _, record := range s.data {
		result = append(result, record)
	}
	return result, nil
}

func (s *UserStore) GetUser(id int) *userRecord {
	s.mx.Lock()
	defer s.mx.Unlock()
	for _, r := range s.data {
		if r.ID == id {
			return &r
		}
	}
	return nil
}

var userStore = newUserStore()

func main() {
	log.Configure(log.NewDefaultConfig())
	logger := log.New("nextjs-api")

	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8080 // Different port from Next.js (usually 3000)
	srvConfig.Debug = true

	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// Setup sessions with relaxed settings for development
	sessionConfig := session.NewConfig()
	sessionConfig.Secure = false                       // Set to true in production with HTTPS
	sessionConfig.SameSite = int(http.SameSiteLaxMode) // Important for cross-origin
	sessionConfig.CookieName = "nextjs_session"        // Custom cookie name
	sessionConfig.ExpirationSeconds = 3600             // 1 hour
	_, err = server.UseSession(sessionConfig, nil, logger)
	if err != nil {
		logger.Fatal(err, "could not initialize session")
		os.Exit(1)
	}

	corsCfg := security.NewCorsConfig()
	corsCfg.AllowOrigins = []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"https://your-app.vercel.app", // Add your production domain
	}
	server.AddMiddleware(security.CORSMiddleware(corsCfg))

	// Health check endpoint (no session required)
	server.Route().GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "nextjs-api",
		})
	})

	// CSRF token endpoint (exempt from CSRF protection but requires session)
	server.Route().GET("/api/csrf-token", func(c *gin.Context) {
		sess := session.Get(c)
		if sess == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Could not initialize session",
			})
			return
		}

		// Generate new CSRF token
		token := security.GenerateCSRFToken(c)
		sess.Set("_csrf", token)

		c.JSON(http.StatusOK, gin.H{
			"csrf_token": token,
			"session_id": sess.ID,
		})
	})

	// Apply CSRF protection to all /api routes except GET requests
	api := server.Route().Group("/api")
	api.Use(security.CSRFProtection())
	{
		// User management endpoints
		api.POST("/users", createUser)
		api.PUT("/users/:id", updateUser)
		api.DELETE("/users/:id", deleteUser)
		api.GET("/users", getUsers)    // GET is exempt from CSRF
		api.GET("/users/:id", getUser) // GET is exempt from CSRF

		// Generic data endpoints
		api.POST("/data", handlePostData)
		api.PUT("/data/:id", handlePutData)
		api.DELETE("/data/:id", handleDeleteData)

		// Form submission endpoint
		api.POST("/submit", handleFormSubmit)

		// File upload endpoint
		api.POST("/upload", handleFileUpload)
	}

	logger.Info("Next.js API server starting on http://localhost:8080")
	logger.Info("CSRF protection enabled for POST/PUT/DELETE requests")
	logger.Info("CORS configured for Next.js development (localhost:3000)")

	server.Start()
}

// User management handlers
func createUser(c *gin.Context) {
	var user struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role"`
	}

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Set default role if not provided
	if user.Role == "" {
		user.Role = "user"
	}

	newUser := userStore.AddUser(user.Name, user.Email, user.Role)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
		"user": gin.H{
			"id":    newUser.ID,
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func updateUser(c *gin.Context) {
	rawId := c.Param("id")

	// Validate user ID
	userId, err := strconv.Atoi(rawId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	if userId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var updates struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
		Role  *string `json:"role"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	userStore.UpdateUser(userId, updates.Name, updates.Email, updates.Role)

	updatedFields := make(map[string]interface{})
	if updates.Name != nil {
		updatedFields["name"] = *updates.Name
	}
	if updates.Email != nil {
		updatedFields["email"] = *updates.Email
	}
	if updates.Role != nil {
		updatedFields["role"] = *updates.Role
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "User updated successfully",
		"user_id":        userId,
		"updated_fields": updatedFields,
	})
}

func deleteUser(c *gin.Context) {
	rawId := c.Param("id")

	// Validate user ID format
	userId, err := strconv.Atoi(rawId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}
	if userId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	userStore.DeleteUser(userId)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "User deleted successfully",
		"deleted_user_id": userId,
	})
}

func getUsers(c *gin.Context) {
	userList, _ := userStore.GetUsers()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"users":   userList,
		"total":   len(userList),
	})
}

func getUser(c *gin.Context) {
	rawId := c.Param("id")

	// Validate and convert user ID
	userId, err := strconv.Atoi(rawId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	if userId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	user := userStore.GetUser(userId)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
	return
}

// Generic data handlers
func handlePostData(c *gin.Context) {
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Data processed successfully",
		"received_data": data,
		"action":        "create",
	})
}

func handlePutData(c *gin.Context) {
	dataID := c.Param("id")

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Data updated successfully",
		"data_id":      dataID,
		"updated_data": data,
		"action":       "update",
	})
}

func handleDeleteData(c *gin.Context) {
	dataID := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "Data deleted successfully",
		"deleted_data_id": dataID,
		"action":          "delete",
	})
}

// Form submission handler
func handleFormSubmit(c *gin.Context) {
	var form struct {
		Name    string `json:"name" form:"name" binding:"required"`
		Email   string `json:"email" form:"email" binding:"required,email"`
		Message string `json:"message" form:"message" binding:"required"`
		Type    string `json:"type" form:"type"`
	}

	// Support both JSON and form data
	contentType := c.GetHeader("Content-Type")
	if contentType == "application/json" {
		if err := c.ShouldBindJSON(&form); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	} else {
		if err := c.ShouldBind(&form); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	// Set default type
	if form.Type == "" {
		form.Type = "contact"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Form submitted successfully",
		"form_data": gin.H{
			"name":    form.Name,
			"email":   form.Email,
			"message": form.Message,
			"type":    form.Type,
		},
	})
}

// File upload handler
func handleFileUpload(c *gin.Context) {
	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No file provided",
		})
		return
	}
	defer file.Close()

	// In a real application, you would save the file
	// For this demo, we just return file info
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File uploaded successfully",
		"file_info": gin.H{
			"filename": header.Filename,
			"size":     header.Size,
			"headers":  header.Header,
		},
	})
}
