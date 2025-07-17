package config

import "github.com/oddbit-project/blueprint/utils"

const (
	ErrNoKey          = utils.Error("Config key does not exist")
	ErrNotImplemented = utils.Error("Config method or type not implemented")
	ErrInvalidType    = utils.Error("Invalid destination type")
)

type ConfigProvider interface {
	Get(dest interface{}) error
	GetKey(key string, dest interface{}) error
	GetStringKey(key string) (string, error)
	GetBoolKey(key string) (bool, error)
	GetIntKey(key string) (int, error)
	GetFloat64Key(key string) (float64, error)
	GetSliceKey(key, separator string) ([]string, error)
	GetConfigNode(key string) (ConfigProvider, error)
	KeyExists(key string) bool
	KeyListExists(keys []string) bool
}
