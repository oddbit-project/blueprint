package provider

import (
	"encoding/json"
	"github.com/oddbit-project/blueprint/config"
	"github.com/oddbit-project/blueprint/utils"
	"io"
	"os"
	"reflect"
	"strconv"
	"sync"
)

const (
	ErrJsonInvalidSource = utils.Error("NewJsonProvider: Invalid source type")
)

type JsonProvider struct {
	config.ConfigProvider
	configData map[string]json.RawMessage
	m          sync.RWMutex
}

func NewJsonProvider(src interface{}) (config.ConfigProvider, error) {
	provider := &JsonProvider{
		configData: make(map[string]json.RawMessage),
	}
	switch src.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(src.(json.RawMessage), &provider.configData); err != nil {
			return nil, err
		}

	case io.Reader:
		if err := provider.fromReader(src.(io.Reader)); err != nil {
			return nil, err
		}

	case string:
		if err := provider.fromFile(src.(string)); err != nil {
			return nil, err
		}

	case []byte:
		if err := json.Unmarshal(src.([]byte), &provider.configData); err != nil {
			return nil, err
		}

	default:
		return nil, ErrJsonInvalidSource
	}
	return provider, nil
}

func (j *JsonProvider) fromReader(src io.Reader) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &j.configData); err != nil {
		return err
	}
	return nil
}

func (j *JsonProvider) fromFile(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	return j.fromReader(f)
}

// applyDefaults applies default values to struct fields that have zero values
func applyDefaults(dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldValue := v.Field(i)

		// Check if field has a default value and is zero
		if defaultVal := field.Tag.Get("default"); defaultVal != "" && fieldValue.IsZero() {
			switch fieldValue.Kind() {
			case reflect.String:
				fieldValue.SetString(defaultVal)
			case reflect.Int:
				if intVal, err := strconv.Atoi(defaultVal); err == nil {
					fieldValue.SetInt(int64(intVal))
				}
			case reflect.Bool:
				if boolVal, err := strconv.ParseBool(defaultVal); err == nil {
					fieldValue.SetBool(boolVal)
				}
			case reflect.Float64:
				if floatVal, err := strconv.ParseFloat(defaultVal, 64); err == nil {
					fieldValue.SetFloat(floatVal)
				}
			}
		}

		// Recursively apply defaults to nested structs
		if fieldValue.Kind() == reflect.Struct {
			if fieldValue.CanAddr() {
				applyDefaults(fieldValue.Addr().Interface())
			}
		}
	}
	return nil
}

func (j *JsonProvider) GetKey(key string, dest interface{}) error {
	j.m.RLock()
	defer j.m.RUnlock()
	if v, ok := j.configData[key]; ok {
		if err := json.Unmarshal(v, dest); err != nil {
			return err
		}
		return applyDefaults(dest)
	}
	return config.ErrNoKey
}

// Get de-serializes everything to dest
func (j *JsonProvider) Get(dest interface{}) error {
	j.m.RLock()
	defer j.m.RUnlock()
	data, err := json.Marshal(j.configData)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return err
	}
	return applyDefaults(dest)
}

func (j *JsonProvider) GetStringKey(key string) (string, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	var result string
	if v, ok := j.configData[key]; ok {
		if err := json.Unmarshal(v, &result); err != nil {
			return "", err
		}
		return result, nil
	}
	return "", config.ErrNoKey
}

func (j *JsonProvider) GetBoolKey(key string) (bool, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	var result bool
	var err error
	if v, ok := j.configData[key]; ok {
		err = json.Unmarshal(v, &result)
	} else {
		err = config.ErrNoKey
	}
	return result, err
}

func (j *JsonProvider) GetIntKey(key string) (int, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	var result int
	var err error
	if v, ok := j.configData[key]; ok {
		err = json.Unmarshal(v, &result)
	} else {
		err = config.ErrNoKey
	}
	return result, err
}

func (j *JsonProvider) GetFloat64Key(key string) (float64, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	var result float64
	var err error
	if v, ok := j.configData[key]; ok {
		err = json.Unmarshal(v, &result)
	} else {
		err = config.ErrNoKey
	}
	return result, err
}

// GetSliceKey note: separator is ignored
func (j *JsonProvider) GetSliceKey(key, separator string) ([]string, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	buf := make([]string, 0)
	if v, ok := j.configData[key]; ok {
		err := json.Unmarshal(v, &buf)
		return buf, err
	}
	return nil, config.ErrNoKey
}

func (j *JsonProvider) GetConfigNode(key string) (config.ConfigProvider, error) {
	j.m.RLock()
	defer j.m.RUnlock()

	if v, ok := j.configData[key]; ok {
		return NewJsonProvider(v)
	}
	return nil, config.ErrNoKey
}

func (j *JsonProvider) KeyExists(key string) bool {
	j.m.RLock()
	defer j.m.RUnlock()

	_, ok := j.configData[key]
	return ok
}

func (j *JsonProvider) KeyListExists(keys []string) bool {
	for _, k := range keys {
		if !j.KeyExists(k) {
			return false
		}
	}
	return true
}
