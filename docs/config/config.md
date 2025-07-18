# blueprint.config

Configuration providers for blueprint

## Overview

The blueprint configuration system provides flexible configuration management through multiple providers:

- **JsonProvider**: Reads configuration from JSON files, streams, or raw data
- **EnvProvider**: Reads configuration from environment variables with advanced features
- **Default Values**: Both providers support default values via struct tags
- **Thread Safety**: EnvProvider and JsonProvider include thread-safe concurrent access
- **Nested Structures**: Full support for complex nested configuration structures

## Using JSON files

Example configuration file *config.json*:
```json
{
  "server": {
    "host": "localhost",
    "port": 1234
  }
}
```

### Basic Usage

```golang
package main

import "github.com/oddbit-project/blueprint/config/provider"

type ServerConfig struct {
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	CertFile     string            `json:"certFile"`
	CertKeyFile  string            `json:"certKeyFile"`
	ReadTimeout  int               `json:"readTimeout"`
	WriteTimeout int               `json:"writeTimeout"`
	Debug        bool              `json:"debug"`
	Options      map[string]string `json:"options"`
}

func main() {
	if cfg, err := provider.NewJsonProvider("config.json"); err == nil {
        serverConfig := &ServerConfig{}
        // extract struct serverConfig from "server" key
        if err := cfg.GetKey("server", serverConfig); err == nil {
		    // run server using config
	    } else {
	        // error reading config key
	    }
	} else {
	    // error reading config file
	}
}
```

### JSON Provider with Default Values

```golang
type ServerConfig struct {
	Host         string            `json:"host" default:"localhost"`
	Port         int               `json:"port" default:"8080"`
	CertFile     string            `json:"certFile" default:""`
	CertKeyFile  string            `json:"certKeyFile" default:""`
	ReadTimeout  int               `json:"readTimeout" default:"30"`
	WriteTimeout int               `json:"writeTimeout" default:"30"`
	Debug        bool              `json:"debug" default:"false"`
	Options      map[string]string `json:"options"`
}

func main() {
	// JSON file with partial configuration
	// Missing fields will use default values from struct tags
	cfg, err := provider.NewJsonProvider("config.json")
	if err != nil {
		panic(err)
	}
	
	// Get entire configuration with defaults applied
	var appConfig struct {
		Server ServerConfig `json:"server"`
	}
	if err := cfg.Get(&appConfig); err != nil {
		panic(err)
	}
	
	// appConfig.Server.Host will be "localhost" if not in JSON
	// appConfig.Server.Port will be 8080 if not in JSON
}
```

### Multiple Data Sources

```golang
// From file
cfg, err := provider.NewJsonProvider("config.json")

// From []byte
jsonData := []byte(`{"host": "example.com", "port": 9090}`)
cfg, err := provider.NewJsonProvider(jsonData)

// From io.Reader
file, _ := os.Open("config.json")
cfg, err := provider.NewJsonProvider(file)

// From json.RawMessage
var rawMsg json.RawMessage
// ... populate rawMsg
cfg, err := provider.NewJsonProvider(rawMsg)
```


## Using Environment variables

### Basic Usage

```golang
type ServerConfig struct {
	Host         string
	Port         int
	CertFile     string
	CertKeyFile  string
	ReadTimeout  int
	WriteTimeout int
	Debug        bool
	Options      map[string]string
}

func main() {
	cfg := provider.NewEnvProvider("SERVER_", true) // prefix and convertCamelCase enabled
	serverConfig := &ServerConfig{}
	if err := cfg.GetKey("", serverConfig); err == nil { // read SERVER_ env vars to struct
		// run server using config
	} else {
		fmt.Println(err)
	}
}
```

### Environment Variables with Default Values

```golang
type ServerConfig struct {
	Host         string `env:"HOST" default:"localhost"`
	Port         int    `env:"PORT" default:"8080"`
	CertFile     string `env:"CERT_FILE" default:""`
	CertKeyFile  string `env:"CERT_KEY_FILE" default:""`
	ReadTimeout  int    `env:"READ_TIMEOUT" default:"30"`
	WriteTimeout int    `env:"WRITE_TIMEOUT" default:"30"`
	Debug        bool   `env:"DEBUG" default:"false"`
	MaxConns     int    `env:"MAX_CONNECTIONS" default:"100"`
}

func main() {
	// Set only some environment variables
	os.Setenv("SERVER_HOST", "production.example.com")
	os.Setenv("SERVER_PORT", "9090")
	// DEBUG, READ_TIMEOUT, etc. will use defaults
	
	cfg := provider.NewEnvProvider("SERVER_", true)
	serverConfig := &ServerConfig{}
	
	if err := cfg.Get(serverConfig); err != nil {
		panic(err)
	}
	
	// serverConfig.Host = "production.example.com" (from env var)
	// serverConfig.Port = 9090 (from env var)
	// serverConfig.Debug = false (from default tag)
	// serverConfig.ReadTimeout = 30 (from default tag)
}
```

### Nested Structures

```golang
type DatabaseConfig struct {
	Host     string `env:"HOST" default:"localhost"`
	Port     int    `env:"PORT" default:"5432"`
	Database string `env:"NAME" default:"myapp"`
	Username string `env:"USER" default:"user"`
	Password string `env:"PASSWORD" default:""`
}

type ServerConfig struct {
	Host string `env:"HOST" default:"localhost"`
	Port int    `env:"PORT" default:"8080"`
}

type AppConfig struct {
	Database DatabaseConfig `env:"DATABASE"`
	Server   ServerConfig   `env:"SERVER"`
}

func main() {
	// Environment variables:
	// APP_DATABASE_HOST=db.example.com
	// APP_DATABASE_PORT=5432
	// APP_SERVER_HOST=api.example.com
	// APP_SERVER_PORT=9090
	
	cfg := provider.NewEnvProvider("APP_", true)
	config := &AppConfig{}
	
	if err := cfg.Get(config); err != nil {
		panic(err)
	}
	
	// config.Database.Host = "db.example.com" (from APP_DATABASE_HOST)
	// config.Database.Database = "myapp" (from default tag)
	// config.Server.Host = "api.example.com" (from APP_SERVER_HOST)
}
```

### CamelCase Conversion

```golang
type Config struct {
	DatabaseURL      string `env:"DATABASE_URL"`
	MaxConnections   int    `env:"MAX_CONNECTIONS" default:"10"`
	EnableTLS        bool   `env:"ENABLE_TLS" default:"true"`
	ConnectionTimeout int   `env:"CONNECTION_TIMEOUT" default:"30"`
}

func main() {
	// With convertCamelCase=true:
	// Field "DatabaseURL" maps to "DATABASE_URL"
	// Field "MaxConnections" maps to "MAX_CONNECTIONS"
	// Field "EnableTLS" maps to "ENABLE_TLS"
	
	cfg := provider.NewEnvProvider("APP_", true) // convertCamelCase=true
	config := &Config{}
	
	if err := cfg.Get(config); err != nil {
		panic(err)
	}
}
```

### Supported Environment Variable Types

- **string**: Direct string value
- **int**: Parsed with `strconv.Atoi()`
- **bool**: Parsed with `strconv.ParseBool()` (supports "true", "false", "1", "0", etc.)
- **float64**: Parsed with `strconv.ParseFloat()`
- **[]string**: Comma-separated values, automatically trimmed

Example:
```bash
# String
export APP_HOST=localhost

# Integer
export APP_PORT=8080

# Boolean
export APP_DEBUG=true

# Float
export APP_TIMEOUT=30.5

# String slice
export APP_FEATURES=feature1,feature2,feature3
```


## Advanced Features

### Configuration Interface

Both providers implement the `ConfigInterface` which provides:

```golang
type ConfigProvider interface {
	Get(dest interface{}) error                                    // Get entire config
	GetKey(key string, dest interface{}) error                     // Get specific key
	GetStringKey(key string) (string, error)                       // Get string value
	GetBoolKey(key string) (bool, error)                           // Get boolean value
	GetIntKey(key string) (int, error)                             // Get integer value
	GetFloat64Key(key string) (float64, error)                     // Get float value
	GetSliceKey(key, separator string) ([]string, error)           // Get string slice
	GetConfigNode(key string) (ConfigInterface, error)             // Get nested config
	KeyExists(key string) bool                                     // Check key existence
	KeyListExists(keys []string) bool                              // Check multiple keys
}
```

### Thread Safety

Both `EnvProvider` and `JsonProvider` are thread-safe and use read-write mutexes for concurrent access:

```golang
cfg := provider.NewEnvProvider("APP_", true)

// Safe to use from multiple goroutines
go func() {
	var config AppConfig
	cfg.Get(&config)
}()

go func() {
	host, _ := cfg.GetStringKey("HOST")
	fmt.Println(host)
}()
```

### Nested Configuration Access

For JSON configurations, you can access nested nodes:

```golang
// config.json
{
  "database": {
    "host": "localhost",
    "port": 5432
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  }
}

// Access nested configuration
cfg, _ := provider.NewJsonProvider("config.json")
dbNode, _ := cfg.GetConfigNode("database")
host, _ := dbNode.GetStringKey("host") // "localhost"
```

### Default Values Behavior

Default values are applied when:

**EnvProvider**: Environment variable is not set
**JsonProvider**: JSON field is missing or has zero value

```golang
type Config struct {
	Host  string `json:"host" env:"HOST" default:"localhost"`
	Port  int    `json:"port" env:"PORT" default:"8080"`
	Debug bool   `json:"debug" env:"DEBUG" default:"false"`
}

// With empty JSON: {}
// Or missing environment variables
// All fields will use their default values
```

## Using Wrappers

### StrOrFile

The StrOrFile() wrapper attempts to identify a valid file path on the argument string. If a
valid path is detected (string either starts with "/" or "./"), will attempt to load the contents 
of the file and return it as a string value. If no valid filepath is detected, or file is not found,
will just return the argument string:

```golang
import "github.com/oddbit-project/blueprint/config"

myPass := config.StrOrFile("some password") // myPass = "some password"
myPass := config.StrOrFile("./credentials.txt") // myPass = contents of credentials.txt
```

This is particularly useful for secrets management:

```golang
type DatabaseConfig struct {
	Host     string `env:"HOST" default:"localhost"`
	Password string `env:"PASSWORD" default:""`
}

func main() {
	cfg := provider.NewEnvProvider("DB_", false)
	dbConfig := &DatabaseConfig{}
	cfg.Get(dbConfig)
	
	// If DB_PASSWORD="/path/to/secret", password will be file contents
	// If DB_PASSWORD="plain_password", password will be the literal string
	actualPassword := config.StrOrFile(dbConfig.Password)
}
```

## Best Practices

### 1. Use Default Values for Development

```golang
type Config struct {
	DatabaseURL string `env:"DATABASE_URL" default:"postgres://localhost/myapp_dev"`
	RedisURL    string `env:"REDIS_URL" default:"redis://localhost:6379"`
	Port        int    `env:"PORT" default:"8080"`
	LogLevel    string `env:"LOG_LEVEL" default:"info"`
}
```

### 2. Combine Multiple Providers

```golang
func LoadConfig() (*Config, error) {
	config := &Config{}
	
	// Try JSON first
	if jsonCfg, err := provider.NewJsonProvider("config.json"); err == nil {
		if err := jsonCfg.Get(config); err == nil {
			return config, nil
		}
	}
	
	// Fallback to environment variables
	envCfg := provider.NewEnvProvider("APP_", true)
	return config, envCfg.Get(config)
}
```

### 3. Validate Configuration

```golang
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	return nil
}

func main() {
	config := &Config{}
	cfg := provider.NewEnvProvider("APP_", true)
	
	if err := cfg.Get(config); err != nil {
		log.Fatal(err)
	}
	
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}
```

