package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/kv"
	"net/http"
	"slices"
)

type ApiServer struct {
	srv        *httpserver.Server
	logger     *log.Logger
	sessionMgr *session.Manager
}

// UserIdentity user identity for authentication purposes
type UserIdentity struct {
	Username string
}

func NewApiServer(cfg *Config) (*ApiServer, error) {

	// create API server logger
	logger := log.New("api-server")

	// create http server
	srv, err := httpserver.NewServer(cfg.Api, logger)
	if err != nil {
		return nil, err
	}

	// ===================================================
	// creating session manager manually
	// ===================================================

	// create session store backend - memory
	storeBackend := kv.NewMemoryKV()

	// create session store - lets use memory
	sessionStore, err := session.NewStore(cfg.Session, storeBackend, logger)
	if err != nil {
		return nil, err
	}

	// create session manager
	sessionMgr, err := session.NewManager(cfg.Session,
		session.ManagerWithStore(sessionStore),
		session.ManagerWithLogger(logger))

	if err != nil {
		return nil, err
	}
	srv.AddMiddleware(sessionMgr.Middleware())

	// ===================================================
	// alternative quick version with defaults:
	//
	// storeBackend :=  kv.NewMemoryKV()
	// sessionMgr, err := srv.UseSession(cfg.Session,storeBackend, logger)
	//

	api := &ApiServer{
		srv:        srv,
		logger:     logger,
		sessionMgr: sessionMgr,
	}

	// register public routes
	srv.Route().GET("/", api.index)
	srv.Route().POST("/login", api.login)

	// enable authentication using session identity verification
	// Note: if using a struct or a custom type as identity value, **always** include it
	// in the parameters of auth.NewAuthSession() or register it using gob.Register(custom_type);
	// registration must match exactly the usage - no ptr unwrapping is performed
	srv.AddMiddleware(auth.AuthMiddleware(auth.NewAuthSession(&UserIdentity{})))

	// register private routes
	srv.Route().GET("/dashboard", api.protectedDashboard)
	srv.Route().GET("/logout", api.logout)

	return api, nil
}

// currentUser helper function to fetch current user
func (a *ApiServer) currentUser(c *gin.Context) *UserIdentity {
	user, exists := auth.GetSessionIdentity(c)
	if exists && user != nil {
		return user.(*UserIdentity)
	}
	return nil
}

// Authenticate mock authentication helper
func (a *ApiServer) Authenticate(username, password string) (*UserIdentity, bool) {
	// to simulate authentication, we accept any username as long as the password
	// is either "1234" or "changeme"
	if slices.Contains([]string{"1234", "changeme"}, password) {
		return &UserIdentity{username}, true
	}
	return nil, false
}

// main shows login status
func (a *ApiServer) index(c *gin.Context) {
	currentUser := a.currentUser(c)
	if currentUser == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "No user logged in",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": fmt.Sprintf("logged in user: %s", currentUser.Username),
		})
	}
}

// login endpoint
func (a *ApiServer) login(c *gin.Context) {
	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if !httpserver.ValidateJSON(c, &loginRequest) {
		// response is filled automatically
		return
	}

	// perform user login
	user, valid := a.Authenticate(loginRequest.Username, loginRequest.Password)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	// store the valid login in the current session
	session.Get(c).SetIdentity(user)

	// Regenerate session ID for security
	a.sessionMgr.Regenerate(c)

	c.JSON(http.StatusOK, gin.H{
		"message": "logged in successfully",
	})
}

// protectedDashboard shows an auth-only page
func (a *ApiServer) protectedDashboard(c *gin.Context) {
	currentUser := a.currentUser(c)
	c.JSON(http.StatusOK, gin.H{
		"status": fmt.Sprintf("logged in user: %s", currentUser.Username),
	})
}

// logout terminates current session
func (a *ApiServer) logout(c *gin.Context) {
	currentSession := session.Get(c)
	currentSession.DeleteIdentity()

	c.JSON(http.StatusOK, gin.H{
		"status": "logged out successfully",
	})
}

func (a *ApiServer) Start() error {
	return a.srv.Start()
}

func (a *ApiServer) Stop(ctx context.Context) error {
	err := a.srv.Shutdown(ctx)
	a.sessionMgr.Shutdown()
	return err
}
