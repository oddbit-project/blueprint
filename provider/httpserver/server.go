package httpserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"net/http"
	"time"
)

type ServerConfig struct {
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	ReadTimeout  int               `json:"readTimeout"`
	WriteTimeout int               `json:"writeTimeout"`
	Debug        bool              `json:"debug"`
	Options      map[string]string `json:"options"`
	tlsProvider.ServerConfig
}

type Server struct {
	Config *ServerConfig
	Router *gin.Engine
	Server *http.Server
}

func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:         "",
		Port:         ServerDefaultPort,
		ReadTimeout:  ServerDefaultReadTimeout,
		WriteTimeout: ServerDefaultWriteTimeout,
		Debug:        false,
		Options:      make(map[string]string),
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

// GetOption retrieves the value associated with the specified key from the Options map of the ServerConfig.
// If the key exists, the corresponding value is returned. Otherwise, the defaultValue is returned.
// The Options map is defined as map[string]string in the ServerConfig struct.
// Example usage:
//
//	serverConfig := ServerConfig{
//	  // initialize other fields
//	  Options: map[string]string{
//	    "key1": "value1",
//	    "key2": "value2",
//	  },
//	}
//	option := serverConfig.GetOption("key1", "default")
//	// option is "value1"
//	option := serverConfig.GetOption("key3", "default")
//	// option is "default"
func (c *ServerConfig) GetOption(key string, defaultValue string) string {
	if v, ok := c.Options[key]; ok {
		return v
	}
	return defaultValue
}

func (c *ServerConfig) Validate() error {
	return nil
}

// NewRouter creates a new gin router with standardized logging
func NewRouter(serverName string, debug bool) *gin.Engine {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// Use our structured logging middleware
	router.Use(log.HTTPLogMiddleware(serverName))

	// Still include recovery middleware
	router.Use(gin.Recovery())

	return router
}

func (c *ServerConfig) NewServer() (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewServer(c)
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
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		return nil, err
	}
	router := NewRouter(cfg.GetOption("serverName", ServerDefaultName), cfg.Debug)
	result := &Server{
		Config: cfg,
		Router: router,
		Server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:      router,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
			TLSConfig:    tlsConfig,
		},
	}
	return result, nil
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
