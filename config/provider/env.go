package provider

import (
	"github.com/gobeam/stringy"
	"github.com/oddbit-project/blueprint/config"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

const CommaSeparator = ","

type EnvProvider struct {
	config.ConfigProvider
	prefix      string
	configData  map[string]string
	convertCase bool // if true, key lookups are converted from localDef -> LOCAL_DEF
	mx          sync.RWMutex
}

var DefaultSeparator = CommaSeparator

// NewEnvProvider builds a new config.ConfigProvider object from system Environment variables.
// The parameter prefix defines the key prefix to use. All existing Environment variables matching the prefix are loaded on creation.
// If convertCamelCase is enabled, field names are automatically parsed from camelCase format to SNAKE_CASE
func NewEnvProvider(prefix string, convertCamelCase bool) *EnvProvider {
	provider := &EnvProvider{
		prefix:      prefix,
		configData:  make(map[string]string),
		convertCase: convertCamelCase,
	}
	provider.load()
	return provider
}

func (e *EnvProvider) load() {
	for _, env := range os.Environ() {
		toks := strings.SplitN(env, "=", 2)
		if strings.HasPrefix(toks[0], e.prefix) {
			// Store without the prefix for easier lookup
			key := strings.TrimPrefix(toks[0], e.prefix)
			e.configData[key] = toks[1]
		}
	}
}

func (e *EnvProvider) convertKey(key string) string {
	if e.convertCase {
		tmp := stringy.New(key)
		return tmp.SnakeCase("?", "").ToUpper()
	}
	return key
}

// readPrefixedStruct reads configuration values with a specified prefix and maps them to fields in a destination struct.
// The prefix and field names are converted to uppercase before searching for the corresponding configuration keys.
// If a configuration key is found, its value is set in the corresponding field of the destination struct based on its data type.
// The supported data types are string, int, bool, float64, and []string.
// If the destination is a pointer to a struct, the function unwraps the pointer to operate on the struct itself.
// The function returns config.ErrInvalidType if the destination is not a struct.
//
// Parameters:
//   - prefix: The prefix for the configuration keys to search for.
//   - dest: A pointer to the destination struct where the configuration values will be mapped to.
//
// Returns:
//   - error: Returns an error if the destination is not a struct or if there is an error setting the field value.
//
// Example usage:
//
//		type Config struct {
//		    Name     string `env:"APP_NAME"`
//		    Port     int    `env:"APP_PORT"`
//		    Debug    bool   `env:"APP_DEBUG"`
//		    Timeout  string `env:"APP_TIMEOUT"`
//		    Database []string `env:"APP_DB_CONNECTIONS"`
//		}
//
//	 cfg := &Config{}
//		e := NewEnvProvider("APP", false)
//
//		err := e.readPrefixedStruct("", &cfg) // in practice, this is publicly called from Get() / GetKey()
//		if err != nil {
//		    // handle error
//		}
//
//		fmt.Println(cfg.Name)     // Output: MyApp
//		fmt.Println(cfg.Port)     // Output: 8080
//		fmt.Println(cfg.Debug)    // Output: true
//		fmt.Println(cfg.Timeout)  // Output: 10s
//		fmt.Println(cfg.Database) // Output: [mysql postgres]
//
//		// Assuming the following environment variables are set:
//		// APP_NAME=MyApp
//		// APP_PORT=8080
//		// APP_DEBUG=true
//		// APP_TIMEOUT=10s
//		// APP_DB_CONNECTIONS=mysql,postgres
func (e *EnvProvider) readPrefixedStruct(prefix string, dest interface{}) error {
	v := reflect.ValueOf(dest)
	// unwrap pointer
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return config.ErrInvalidType
	}
	// Convert prefix if needed
	if prefix != "" {
		prefix = strings.ToUpper(e.convertKey(prefix))
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldName := field.Tag.Get("env")
		fieldValue := v.Field(i)
		if fieldName == "" {
			fieldName = field.Name
		}
		// Build the key without the initial prefix (already stripped in load())
		var envKey string
		if prefix != "" {
			envKey = prefix + "_" + strings.ToUpper(e.convertKey(fieldName))
		} else {
			envKey = strings.ToUpper(e.convertKey(fieldName))
		}

		val, ok := e.configData[envKey]
		if !ok {
			// Check for default value in struct tag
			if defaultVal := field.Tag.Get("default"); defaultVal != "" {
				val = defaultVal
				ok = true
			}
		}

		if ok {
			switch fieldValue.Kind() {
			case reflect.String:
				fieldValue.SetString(val)
			case reflect.Int:
				intVal, err := strconv.Atoi(val)
				if err == nil {
					fieldValue.SetInt(int64(intVal))
				}
			case reflect.Bool:
				boolVal, err := strconv.ParseBool(val)
				if err == nil {
					fieldValue.SetBool(boolVal)
				}
			case reflect.Float64:
				floatVal, err := strconv.ParseFloat(val, 64)
				if err == nil {
					fieldValue.SetFloat(floatVal)
				}
			case reflect.Slice:
				sliceVal := reflect.MakeSlice(fieldValue.Type(), 0, 0)
				for _, s := range strings.Split(val, DefaultSeparator) {
					sliceVal = reflect.Append(sliceVal, reflect.ValueOf(strings.TrimSpace(s)))
				}
				fieldValue.Set(sliceVal)
			}
		} else {
			// if its struct, recurse
			if fieldValue.Kind() == reflect.Struct {
				if fieldValue.CanAddr() {
					if err := e.readPrefixedStruct(envKey, fieldValue.Addr().Interface()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// readKeyInterface reads a configuration value for the specified key
// and maps it to the destination variable based on its data type.
// Supported data types are string, int, bool, float64, and []string.
// If the key does not exist in the configData map, it returns config.ErrNoKey.
// The destination variable must be a pointer type for proper modification.
// Returns an error if there is an error setting the variable value or if the data type is not supported.
func (e *EnvProvider) readKeyInterface(key string, dest interface{}) error {
	key = e.convertKey(key)
	if _, ok := e.configData[key]; !ok {
		return config.ErrNoKey
	}
	switch dest.(type) {
	case *string:
		v, err := e.GetStringKey(key)
		if err == nil {
			reflect.ValueOf(dest).Elem().SetString(v)
		}
		return err

	case *int:
		v, err := e.GetIntKey(key)
		if err == nil {
			reflect.ValueOf(dest).Elem().SetInt(int64(v))
		}
		return err

	case *bool:
		v, err := e.GetBoolKey(key)
		if err == nil {
			reflect.ValueOf(dest).Elem().SetBool(v)
		}
		return err

	case *float64:
		v, err := e.GetFloat64Key(key)
		if err == nil {
			reflect.ValueOf(dest).Elem().SetFloat(v)
		}
		return err

	case *[]string:
		v, err := e.GetSliceKey(key, DefaultSeparator)
		if err == nil {
			de := reflect.ValueOf(dest).Elem()
			de.Set(reflect.ValueOf(v))
		}
		return err
	}
	return config.ErrNotImplemented
}

// Get reads all keys to the destination interface
func (e *EnvProvider) Get(dest interface{}) error {
	return e.GetKey("", dest)
}

// GetKey reads an env key to an interface. if dest is a pointer to a struct, key is used as a prefix,
// and it will attempt to extract key+fieldName or key+field_env into the different struct fields.
// if dest is a valid type and key is a valid env var, it will extract the env value into the var.
// key must include the prefix, if any
func (e *EnvProvider) GetKey(key string, dest interface{}) error {
	e.mx.RLock()
	defer e.mx.RUnlock()

	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}

	destType := reflect.TypeOf(dest)
	if destType.Kind() == reflect.Ptr {
		v := destType.Elem()
		if v.Kind() == reflect.Struct {
			// For structs, pass the key as prefix
			return e.readPrefixedStruct(key, dest)
		}
	}
	// For non-struct values, read directly
	return e.readKeyInterface(key, dest)
}

func (e *EnvProvider) GetStringKey(key string) (string, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()
	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}

	v, ok := e.configData[key]
	if !ok {
		return "", config.ErrNoKey
	}
	return v, nil
}

func (e *EnvProvider) GetBoolKey(key string) (bool, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()
	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}

	if v, ok := e.configData[key]; ok {
		return strconv.ParseBool(v)
	}
	return false, config.ErrNoKey
}

func (e *EnvProvider) GetIntKey(key string) (int, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()

	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}
	if v, ok := e.configData[key]; ok {
		return strconv.Atoi(v)
	}
	return 0, config.ErrNoKey
}

func (e *EnvProvider) GetFloat64Key(key string) (float64, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()
	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}
	if v, ok := e.configData[key]; ok {
		return strconv.ParseFloat(v, 64)
	}
	return 0, config.ErrNoKey
}

func (e *EnvProvider) GetSliceKey(key, separator string) ([]string, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()
	key = e.convertKey(key)
	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}
	if v, ok := e.configData[key]; ok {
		buf := make([]string, 0)
		for _, s := range strings.Split(v, separator) {
			buf = append(buf, strings.TrimSpace(s))
		}
		return buf, nil
	}
	return nil, config.ErrNoKey
}

func (e *EnvProvider) GetConfigNode(key string) (config.ConfigProvider, error) {
	return nil, config.ErrNotImplemented
}

// KeyExists checks if a key exists
// key must include the prefix, if any
func (e *EnvProvider) KeyExists(key string) bool {
	e.mx.RLock()
	defer e.mx.RUnlock()
	key = e.convertKey(key)

	if e.prefix != "" {
		key = strings.TrimPrefix(key, e.prefix)
	}

	_, exists := e.configData[key]
	return exists
}

func (e *EnvProvider) KeyListExists(keys []string) bool {
	for _, k := range keys {
		if !e.KeyExists(e.convertKey(k)) {
			return false
		}
	}
	return true
}
