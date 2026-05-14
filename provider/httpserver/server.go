package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/log/writer"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	httplog "github.com/oddbit-project/blueprint/provider/httpserver/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

type ServerConfig struct {
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	ReadTimeout    int      `json:"readTimeout"`
	WriteTimeout   int      `json:"writeTimeout"`
	Debug          bool     `json:"debug"`
	ServerName     string   `json:"serverName"`
	TrustedProxies []string `json:"trustedProxies"`
	tlsProvider.ServerConfig
}

type Server struct {
	Config *ServerConfig
	Router *gin.Engine
	Server *http.Server
	Logger *log.Logger
}

type OptionsFunc func(*Server) error

func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:           "",
		Port:           ServerDefaultPort,
		ReadTimeout:    ServerDefaultReadTimeout,
		WriteTimeout:   ServerDefaultWriteTimeout,
		Debug:          false,
		ServerName:     ServerDefaultName,
		TrustedProxies: make([]string, 0),
		ServerConfig: tlsProvider.ServerConfig{
			TLSCert: "",
			TLSKey:  "",
			TlsKeyCredential: tlsProvider.TlsKeyCredential{
				Password:       "",
				PasswordEnvVar: "",
				PasswordFile:   "",
			},
			TLSAllowedCACerts:  nil,
			TLSCipherSuites:    nil,
			TLSMinVersion:      "",
			TLSMaxVersion:      "",
			TLSAllowedDNSNames: nil,
			TLSEnable:          false,
		},
	}
}

// GetUrl build http url from config
func (c *ServerConfig) GetUrl() string {
	if c.TLSEnable {
		return fmt.Sprintf("https://%s:%d", c.Host, c.Port)
	}
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

func (c *ServerConfig) Validate() error {
	if c.ServerName == "" {
		c.ServerName = ServerDefaultName
	}
	if c.Port == 0 {
		c.Port = ServerDefaultPort
	}
	if c.Port < 0 || c.Port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = ServerDefaultReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = ServerDefaultWriteTimeout
	}
	return nil
}

// NewRouter creates a new gin router with standardized logging
func NewRouter(serverName string, debug bool, logger *log.Logger) *gin.Engine {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	if logger != nil {
		// Use our structured logging middleware
		router.Use(httplog.HTTPLogMiddleware(logger))
	}

	// Still include recovery middleware
	router.Use(gin.Recovery())

	return router
}

func (c *ServerConfig) NewServer(logger *log.Logger) (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewServer(c, logger)
}

// NewServer creates a new http server.
//
// Example usage:
//
//	cfg := &ServerConfig{...}
//	server, err := NewServer(cfg)
//	if err != nil {
//	  log.Fatal(err)
//	}
//	server.Start()
func NewServer(cfg *ServerConfig, logger *log.Logger) (*Server, error) {
	if cfg == nil {
		cfg = NewServerConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if logger == nil {
		logger = httplog.NewHTTPLogger(cfg.ServerName)
	}

	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		return nil, err
	}
	router := NewRouter(cfg.ServerName, cfg.Debug, logger)

	if len(cfg.TrustedProxies) > 0 {
		if err = router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			return nil, err
		}
	}

	result := &Server{
		Config: cfg,
		Router: router,
		Logger: logger,
		Server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:      router,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
			TLSConfig:    tlsConfig,
			ErrorLog:     writer.NewErrorLog(logger), // error log wrapper
		},
	}

	return result, nil
}

// WithDefaultSecurityHeaders returns an OptionsFunc that enables default security headers
func WithDefaultSecurityHeaders() OptionsFunc {
	return func(s *Server) error {
		s.UseDefaultSecurityHeaders()
		return nil
	}
}

// WithAuthToken returns an OptionsFunc that enables token-based authentication.
// If headerName is empty, auth.DefaultTokenAuthHeader is used.
func WithAuthToken(headerName, secret string) OptionsFunc {
	return func(s *Server) error {
		if headerName == "" {
			headerName = auth.DefaultTokenAuthHeader
		}
		s.UseAuth(auth.NewAuthToken(headerName, secret))
		return nil
	}
}

// ProcessOptions applies functional options to the server
func (s *Server) ProcessOptions(withOptions ...OptionsFunc) error {
	for _, withOption := range withOptions {
		if err := withOption(s); err != nil {
			return err
		}
	}
	return nil
}

// AddMiddleware adds the specified middleware function to the server's router.
// The middlewareFunc parameter should be a function that accepts a *gin.Context parameter.
// The function is added as middleware to the server's router using the Use() method of the gin.Engine.
// This allows the middleware to be executed for each incoming request before reaching the final handler.
// Example usage:
//
//	server.AddMiddleware(myMiddleware)
//
//	func myMiddleware(ctx *gin.Context) {
//	  // do something before reaching the final handler
//	  ctx.Next()
//	  // do something after the final handler
//	}
//
// Note: The AddMiddleware method is defined on the Server struct which contains a Router field of type gin.Engine.
func (s *Server) AddMiddleware(middlewareFunc gin.HandlerFunc) {
	s.Router.Use(middlewareFunc)
}

// Group creates a new RouterGroup with the specified relativePath.
// A RouterGroup is used to group routes together and apply common middleware and settings.
// The relativePath parameter is the base path for all routes added to the group.
// The returned value is a pointer to the newly created RouterGroup.
//
// Example usage:
//
//	server := &Server{
//	  // initialize other fields
//	  Router: gin.New(),
//	}
//
// group := server.Group("/api")
// group.GET("/users", getUsers)
//
// This will create a group with the base path "/api" and add a route for GET "/users".
// All routes added to the group will have the "/api" prefix.
func (s *Server) Group(relativePath string) *gin.RouterGroup {
	return s.Router.Group(relativePath)
}

// Route returns the gin.Engine instance associated with the Server.
//
// It is used to access the underlying gin.Engine for adding routes and defining middleware.
//
// Example usage:
//
//	server := Server{
//	  // initialize other fields
//	}
//	engine := server.Route()
//	engine.GET("/hello", func(c *gin.Context) {
//	  // handle request
//	})
//
// Note: The gin.Engine instance is stored in the Router field of the Server struct.
func (s *Server) Route() *gin.Engine {
	return s.Router
}

// Start starts the HTTP server of the Server instance.
// If the Server's TLSConfig is nil, it starts the server using ListenAndServe method of httpserver.Server.
// Otherwise, it starts the server using ListenAndServeTLS method of httpserver.Server.
// If the returned error from the server is not http.ErrServerClosed, it is returned.
// Otherwise, nil is returned.
//
// Usage example:
//
//	func (a *SampleApplication) Run() {
//	    // register http destructor callback
//	    blueprint.RegisterDestructor(func() error {
//	        return a.httpServer.Shutdown(a.container.GetContext())
//	    })
//
//	    // Start  application - http server
//	    a.container.Run(func(app interface{}) error {
//	        go func() {
//	            log.Info().Msg(fmt.Sprintf("Running Sample Application API at %s:%d", a.httpServer.Config.Host, a.httpServer.Config.Port))
//	            a.container.AbortFatal(a.httpServer.Start())
//	        }()
//	        return nil
//	    })
//	}
func (s *Server) Start() error {
	var err error
	if s.Server.TLSConfig == nil {
		err = s.Server.ListenAndServe()
	} else {
		err = s.Server.ListenAndServeTLS("", "")
	}
	// when Shutdown() is called, the return error is http.ErrServerClosed
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server by calling the Shutdown method of the underlying httpserver.Server.
// It takes a context.Context object as a parameter, which can be used to control the shutdown process.
// The method returns an error if the shutdown process fails.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}
