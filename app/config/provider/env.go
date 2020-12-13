package provider

import (
	"github.com/oddbit-project/blueprint/app/config"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const CommaSeparator = ","

type EnvProvider struct {
	prefix     string
	configData map[string]string
}

var DefaultSeparator = CommaSeparator

// NewEnvProvider builds a new config.ConfigInterface object from system Environment variables.
// The parameter prefix defines the key prefix to use. All existing Environment variables matching the prefix are loaded on creation.
func NewEnvProvider(prefix string) *EnvProvider {
	provider := &EnvProvider{
		prefix:     prefix,
		configData: make(map[string]string),
	}
	provider.load()
	return provider
}

func (e *EnvProvider) load() {
	for _, env := range os.Environ() {
		toks := strings.SplitN(env, "=", 2)
		if strings.HasPrefix(toks[0], e.prefix) {
			e.configData[toks[0]] = toks[1]
		}
	}
}

// GetKey retrieves a config key to a destination variable.
// Supported data types: string, int, bool, float64, []string.
func (e *EnvProvider) GetKey(key string, dest interface{}) error {
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

func (e *EnvProvider) GetStringKey(key string) (string, error) {
	v, ok := e.configData[key]
	if !ok {
		return "", config.ErrNoKey
	}
	return v, nil
}

func (e *EnvProvider) GetBoolKey(key string) (bool, error) {
	if v, ok := e.configData[key]; ok {
		return strconv.ParseBool(v)
	}
	return false, config.ErrNoKey
}

func (e *EnvProvider) GetIntKey(key string) (int, error) {
	if v, ok := e.configData[key]; ok {
		return strconv.Atoi(v)
	}
	return 0, config.ErrNoKey
}

func (e *EnvProvider) GetFloat64Key(key string) (float64, error) {
	if v, ok := e.configData[key]; ok {
		return strconv.ParseFloat(v, 64)
	}
	return 0, config.ErrNoKey
}

func (e *EnvProvider) GetSliceKey(key, separator string) ([]string, error) {
	if v, ok := e.configData[key]; ok {
		buf := make([]string, 0)
		for _, s := range strings.Split(v, separator) {
			buf = append(buf, strings.TrimSpace(s))
		}
		return buf, nil
	}
	return nil, config.ErrNoKey
}

func (e *EnvProvider) GetConfigNode(key string) (config.ConfigInterface, error) {
	return nil, config.ErrNotImplemented
}

func (e *EnvProvider) KeyExists(key string) bool {
	_, exists := e.configData[key]
	return exists
}

func (e *EnvProvider) KeyListExists(keys []string) bool {
	for _, k := range keys {
		if !e.KeyExists(k) {
			return false
		}
	}
	return true
}
