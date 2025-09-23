package provider

import (
	"github.com/oddbit-project/blueprint/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"strconv"
	"testing"
)

const (
	envPrefix     = "TEST_"
	envStrValue   = "simple string"
	envBoolValue  = "true"
	envIntValue   = "45"
	envFloatValue = "72.95"
	envListValue  = "A, b,C,d"
)

var expectedVar5Value = []string{"A", "b", "C", "d"}

type EnvStructRegular struct {
	String string
	Bool   bool
	Int    int
	Float  float64
	List   []string
}

type EnvStructCamelCase struct {
	CamelCaseString string
	CamelCaseBool   bool
	CamelCaseInt    int
	CamelCaseFloat  float64
	CamelCaseList   []string
}

type nestedStruct struct {
	Regular EnvStructRegular
	Camel   EnvStructCamelCase
}

// Test struct with default values
type ConfigWithDefaults struct {
	Host    string `env:"HOST" default:"localhost"`
	Port    int    `env:"PORT" default:"8080"`
	Debug   bool   `env:"DEBUG" default:"false"`
	Timeout string `env:"TIMEOUT" default:"30s"`
}

var envVars = map[string]string{
	"TEST_STRING":            envStrValue,
	"TEST_BOOL":              envBoolValue,
	"TEST_INT":               envIntValue,
	"TEST_FLOAT":             envFloatValue,
	"TEST_LIST":              envListValue,
	"TEST_CAMEL_CASE_STRING": envStrValue,
	"TEST_CAMEL_CASE_BOOL":   envBoolValue,
	"TEST_CAMEL_CASE_INT":    envIntValue,
	"TEST_CAMEL_CASE_FLOAT":  envFloatValue,
	"TEST_CAMEL_CASE_LIST":   envListValue,
	"TEST_String":            envStrValue,   // EnvStructRegular, camelCase=false
	"TEST_Bool":              envBoolValue,  // EnvStructRegular, camelCase=false
	"TEST_Int":               envIntValue,   // EnvStructRegular, camelCase=false
	"TEST_Float":             envFloatValue, // EnvStructRegular, camelCase=false
	"TEST_List":              envListValue,  // EnvStructRegular, camelCase=false

}

var nestedEnvVars = map[string]string{
	"TEST_REGULAR_STRING":          envStrValue,
	"TEST_REGULAR_BOOL":            envBoolValue,
	"TEST_REGULAR_INT":             envIntValue,
	"TEST_REGULAR_FLOAT":           envFloatValue,
	"TEST_REGULAR_LIST":            envListValue,
	"TEST_CAMEL_CAMEL_CASE_STRING": envStrValue,
	"TEST_CAMEL_CAMEL_CASE_BOOL":   envBoolValue,
	"TEST_CAMEL_CAMEL_CASE_INT":    envIntValue,
	"TEST_CAMEL_CAMEL_CASE_FLOAT":  envFloatValue,
	"TEST_CAMEL_CAMEL_CASE_LIST":   envListValue,
}

func setEnvVars(t *testing.T, vars map[string]string) {
	for k, v := range vars {
		if _, exists := os.LookupEnv(k); exists {
			t.Fatalf("setEnvVars(): env var '%s' already exists", k)
		}
		require.NoError(t, os.Setenv(k, v))
	}
}

func resetEnvVars(vars map[string]string) {
	for k := range vars {
		os.Unsetenv(k)
	}
}

func TestNewEnvProvider(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	keys := make([]string, 0)
	for k := range envVars {
		keys = append(keys, k)
	}
	assert.True(t, cfg.KeyListExists(keys), "failed loading env vars")
}

func TestEnvProvider_GetBoolKey(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	b, err := cfg.GetBoolKey("TEST_BOOL")
	require.NoError(t, err)
	assert.True(t, b)

	// attempt to read invalid value
	_, err = cfg.GetBoolKey("TEST_STR")
	assert.Error(t, err, "non-bool should return error")

	// attempt to read camelcase
	cfg = NewEnvProvider(envPrefix, true)
	b, err = cfg.GetBoolKey("testCamelCaseBool")
	require.NoError(t, err)
	assert.True(t, b)
}

func TestEnvProvider_GetConfigNode(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	node, err := cfg.GetConfigNode("TEST_STR")
	assert.ErrorIs(t, err, config.ErrNotImplemented)
	assert.Nil(t, node)
}

func TestEnvProvider_GetFloat64Key(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetFloat64Key("TEST_FLOAT")
	require.NoError(t, err)

	expected, _ := strconv.ParseFloat(envFloatValue, 64)
	assert.Equal(t, expected, v)

	// attempt to read invalid value
	_, err = cfg.GetFloat64Key("TEST_STR")
	assert.Error(t, err, "non-float64 should return error")

	// read camelCase key
	cfg = NewEnvProvider(envPrefix, true)
	_, err = cfg.GetFloat64Key("testCamelCaseFloat")
	require.NoError(t, err)
}

func TestEnvProvider_GetIntKey(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetIntKey("TEST_INT")
	if err != nil {
		t.Error("EnvProvider_GetIntKey():", err)
	}

	expected, _ := strconv.Atoi(envIntValue)
	if v != expected {
		t.Error("EnvProvider_GetIntKey(): value mismatch")
	}

	// attempt to read invalid value
	_, err = cfg.GetIntKey("TEST_STR")
	if err == nil {
		t.Error("EnvProvider_GetIntKey(): non-int should return error")
	}

	// read camelCase key
	cfg = NewEnvProvider(envPrefix, true)
	_, err = cfg.GetIntKey("testCamelCaseInt")
	if err != nil {
		t.Error("EnvProvider_GetIntKey():", err)
	}
}

func TestEnvProvider_GetKey(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)

	var var1 string
	var var2 bool
	var var3 int
	var var4 float64
	var var5 []string
	var err error

	// string
	if err = cfg.GetKey("TEST_STRING", &var1); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		if var1 != envStrValue {
			t.Error("EnvProvider_GetKey(): string value mismatch")
		}
	}

	// bool
	if err = cfg.GetKey("TEST_BOOL", &var2); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		b, _ := strconv.ParseBool(envBoolValue)
		if var2 != b {
			t.Error("EnvProvider_GetKey(): bool value mismatch")
		}
	}

	// int
	if err = cfg.GetKey("TEST_INT", &var3); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		i, _ := strconv.Atoi(envIntValue)
		if var3 != i {
			t.Error("EnvProvider_GetKey(): int value mismatch")
		}
	}

	// float64
	if err = cfg.GetKey("TEST_FLOAT", &var4); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		f, _ := strconv.ParseFloat(envFloatValue, 64)
		if var4 != f {
			t.Error("EnvProvider_GetKey(): float value mismatch")
		}
	}

	// string slice
	if err = cfg.GetKey("TEST_LIST", &var5); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		if !reflect.DeepEqual(var5, expectedVar5Value) {
			t.Error("EnvProvider_GetKey(): string slice value mismatch")
		}
	}

	// camelCase key
	cfg = NewEnvProvider(envPrefix, true)

	// camelCase key for string value
	if err = cfg.GetKey("testCamelCaseString", &var1); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		if var1 != envStrValue {
			t.Error("EnvProvider_GetKey(): string value mismatch")
		}
	}

}

func TestEnvProvider_GetKey_Struct(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	structRegular := &EnvStructRegular{}
	if err := cfg.GetKey("", structRegular); err != nil {
		t.Error("TestEnvProvider_GetKey_Struct():", err)
	}
	if structRegular.String != envStrValue {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid string value")
	}
	value, _ := strconv.ParseBool(envBoolValue)
	if structRegular.Bool != value {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid bool value")
	}
	i, _ := strconv.Atoi(envIntValue)
	if structRegular.Int != i {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid int value")
	}
	f, _ := strconv.ParseFloat(envFloatValue, 64)
	if structRegular.Float != f {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid float value")
	}
	if !reflect.DeepEqual(structRegular.List, expectedVar5Value) {
		t.Error("EnvProvider_GetKey_Struct(): string slice value mismatch")
	}

	// now with convertCamelCase = true
	cfg = NewEnvProvider(envPrefix, true)
	structCamelCase := &EnvStructCamelCase{}
	if err := cfg.GetKey("", structCamelCase); err != nil {
		t.Error("TestEnvProvider_GetKey_Struct():", err)
	}
	if structCamelCase.CamelCaseString != envStrValue {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid string value")
	}
	value, _ = strconv.ParseBool(envBoolValue)
	if structCamelCase.CamelCaseBool != value {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid bool value")
	}
	i, _ = strconv.Atoi(envIntValue)
	if structCamelCase.CamelCaseInt != i {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid int value")
	}
	f, _ = strconv.ParseFloat(envFloatValue, 64)
	if structCamelCase.CamelCaseFloat != f {
		t.Error("TestEnvProvider_GetKey_Struct(): invalid float value")
	}
	if !reflect.DeepEqual(structCamelCase.CamelCaseList, expectedVar5Value) {
		t.Error("EnvProvider_GetKey_Struct(): string slice value mismatch")
	}
}

func TestEnvProvider_GetSliceKey(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetSliceKey("TEST_LIST", ",")
	if err != nil {
		t.Error("EnvProvider_GetSliceKey():", err)
	}
	if !reflect.DeepEqual(v, expectedVar5Value) {
		t.Error("EnvProvider_GetSliceKey(): value mismatch")
	}

	cfg = NewEnvProvider(envPrefix, true)
	_, err = cfg.GetSliceKey("testCamelCaseList", ",")
	if err != nil {
		t.Error("EnvProvider_GetSliceKey():", err)
	}
}

func TestEnvProvider_GetStringKey(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetStringKey("TEST_STRING")
	if err != nil {
		t.Error("EnvProvider_GetStringKey():", err)
	}

	if v != envStrValue {
		t.Error("EnvProvider_GetStringKey(): value mismatch")
	}

	cfg = NewEnvProvider(envPrefix, true)
	_, err = cfg.GetStringKey("testCamelCaseString")
	if err != nil {
		t.Error("EnvProvider_GetStringKey():", err)
	}
}

func TestEnvProvider_KeyExists(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	for k := range envVars {
		if !cfg.KeyExists(k) {
			t.Error("EnvProvider_KeyExists(): existing key not found")
		}
	}
	if cfg.KeyExists("NON-EXISTING") {
		t.Error("EnvProvider_KeyExists(): non-existing key mismatch")
	}
	// camelCase
	cfg = NewEnvProvider(envPrefix, true)
	for k := range envVars {
		if !cfg.KeyExists(k) {
			t.Error("EnvProvider_KeyExists(): existing key not found")
		}
	}
	if cfg.KeyExists("NON-EXISTING") {
		t.Error("EnvProvider_KeyExists(): non-existing key mismatch")
	}
}

func TestEnvProvider_KeyListExists(t *testing.T) {
	setEnvVars(t, envVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	keys := make([]string, 0)
	for k := range envVars {
		keys = append(keys, k)
	}
	if !cfg.KeyListExists(keys) {
		t.Error("EnvProvider_KeyListExists(): existing keys result mismatch")
	}
	keys = append(keys, "")
	if cfg.KeyListExists(keys) {
		t.Error("EnvProvider_KeyListExists(): non-existing keys result mismatch")
	}

	cfg = NewEnvProvider(envPrefix, true)
	keys = make([]string, 0)
	for k := range envVars {
		keys = append(keys, k)
	}
	if !cfg.KeyListExists(keys) {
		t.Error("EnvProvider_KeyListExists(): existing keys result mismatch")
	}
	keys = append(keys, "")
	if cfg.KeyListExists(keys) {
		t.Error("EnvProvider_KeyListExists(): non-existing keys result mismatch")
	}
}

func TestEnvProvider_Get_Struct(t *testing.T) {
	setEnvVars(t, nestedEnvVars)
	defer resetEnvVars(envVars)

	cfg := NewEnvProvider(envPrefix, false)
	nested := &nestedStruct{}
	if err := cfg.Get(nested); err != nil {
		t.Error("failed to get nested struct", err)
	}
	if nested.Regular.String != envStrValue {
		t.Error("invalid string value")
	}
	value, _ := strconv.ParseBool(envBoolValue)
	if nested.Regular.Bool != value {
		t.Error(" invalid bool value")
	}
	i, _ := strconv.Atoi(envIntValue)
	if nested.Regular.Int != i {
		t.Error("invalid int value")
	}
	f, _ := strconv.ParseFloat(envFloatValue, 64)
	if nested.Regular.Float != f {
		t.Error("invalid float value")
	}
	if !reflect.DeepEqual(nested.Regular.List, expectedVar5Value) {
		t.Error("string slice value mismatch")
	}
}

// Test for default values functionality
func TestEnvProvider_DefaultValues(t *testing.T) {
	// Set only some env vars, leaving others to use defaults
	testVars := map[string]string{
		"TEST_HOST": "custom.host",
		"TEST_PORT": "9090",
		// DEBUG and TIMEOUT will use defaults
	}

	setEnvVars(t, testVars)
	defer resetEnvVars(testVars)

	cfg := NewEnvProvider("TEST_", false)
	config := &ConfigWithDefaults{}

	err := cfg.Get(config)
	require.NoError(t, err)

	// Check that env vars were used
	assert.Equal(t, "custom.host", config.Host)
	assert.Equal(t, 9090, config.Port)

	// Check that defaults were applied
	assert.Equal(t, false, config.Debug)
	assert.Equal(t, "30s", config.Timeout)
}
