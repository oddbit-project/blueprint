package provider

import (
	"encoding/json"
	"github.com/oddbit-project/blueprint/config"
	"github.com/oddbit-project/blueprint/utils"
	"io"
	"os"
)

const (
	ErrJsonInvalidSource = utils.Error("NewJsonProvider: Invalid source type")
)

type JsonProvider struct {
	config.ConfigInterface
	configData map[string]json.RawMessage
}

func NewJsonProvider(src interface{}) (config.ConfigInterface, error) {
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

func (j *JsonProvider) GetKey(key string, dest interface{}) error {
	if v, ok := j.configData[key]; ok {
		return json.Unmarshal(v, dest)
	}
	return config.ErrNoKey
}

func (j *JsonProvider) GetStringKey(key string) (string, error) {
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
	buf := make([]string, 0)
	if v, ok := j.configData[key]; ok {
		err := json.Unmarshal(v, &buf)
		return buf, err
	}
	return nil, config.ErrNoKey
}

func (j *JsonProvider) GetConfigNode(key string) (config.ConfigInterface, error) {
	if v, ok := j.configData[key]; ok {
		return NewJsonProvider(v)
	}
	return nil, config.ErrNoKey
}

func (j *JsonProvider) KeyExists(key string) bool {
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
