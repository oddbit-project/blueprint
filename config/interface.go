package config

import "github.com/oddbit-project/blueprint/utils"

const (
	ErrNoKey          = utils.Error("Config key does not exist")
	ErrNotImplemented = utils.Error("Config method or type not implemented")
	ErrInvalidType    = utils.Error("EnvProvider: Invalid destination type")
)

type ConfigInterface interface {
	GetKey(key string, dest interface{}) error
	GetStringKey(key string) (string, error)
	GetBoolKey(key string) (bool, error)
	GetIntKey(key string) (int, error)
	GetFloat64Key(key string) (float64, error)
	GetSliceKey(key, separator string) ([]string, error)
	GetConfigNode(key string) (ConfigInterface, error)
	KeyExists(key string) bool
	KeyListExists(keys []string) bool
}
