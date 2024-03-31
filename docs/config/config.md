# blueprint.config

Configuration providers for blueprint

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


## Using Environment variables

Defined environment variables:
```
SERVER_HOST
SERVER_PORT
SERVER_CERT_FILE
SERVER_CERT_KEY_FILE
SERVER_READ_TIMEOUT
SERVER_WRITE_TIMEOUT
SERVER_DEBUG
SERVER_OPTIONS
```

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
	cfg := provider.NewEnvProvider("", true) // no prefix specified, but convertCamelCase enabled
	serverConfig := &ServerConfig{}
	if err := cfg.GetKey("server", serverConfig); err == nil { // read SERVER_ env vars to struct
		// run server using config
	} else {
		fmt.Println(err)
	}
}
```


## Using Wrappers

### StrOrFile

The StrOrFile() wrapper attempts to identify a valid file path on the argument string. If a
valid path is detected (string either starts with "/" or "./"), will attempt to load the contents 
of the file and return it as a string value. If no valid filepath is detected, or file is not found,
will just return the argument string:

```golang

myPass := StrOrFile("some password") // myPass = "some password"
myPass := StrOrFile("./credentials.txt") // myPass = contents of credentials.txt
```

