package provider

import (
	"github.com/oddbit-project/blueprint/config"
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

type envStructRegular struct {
	String string
	Bool   bool
	Int    int
	Float  float64
	List   []string
}

type envStructCamelCase struct {
	CamelCaseString string
	CamelCaseBool   bool
	CamelCaseInt    int
	CamelCaseFloat  float64
	CamelCaseList   []string
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
	"TEST_String":            envStrValue,   // envStructRegular, camelCase=false
	"TEST_Bool":              envBoolValue,  // envStructRegular, camelCase=false
	"TEST_Int":               envIntValue,   // envStructRegular, camelCase=false
	"TEST_Float":             envFloatValue, // envStructRegular, camelCase=false
	"TEST_List":              envListValue,  // envStructRegular, camelCase=false

}

func setEnvVars(t *testing.T) {
	for k, v := range envVars {
		if _, exists := os.LookupEnv(k); exists {
			t.Fatalf("setEnvVars(): env var '%s' already exists", k)
		}
		os.Setenv(k, v)
	}
}

func resetEnvVars() {
	for k, _ := range envVars {
		os.Unsetenv(k)
	}
}

func TestNewEnvProvider(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	keys := make([]string, 0)
	for k, _ := range envVars {
		keys = append(keys, k)
	}
	if !cfg.KeyListExists(keys) {
		t.Error("NewEnvProvider(): failed loading env vars")
	}
}

func TestEnvProvider_GetBoolKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	b, err := cfg.GetBoolKey("TEST_BOOL")
	if err != nil {
		t.Error("EnvProvider_GetBoolKey():", err)
	}
	if b != true {
		t.Error("EnvProvider_GetBoolKey(): value mismatch")
	}

	// attempt to read invalid value
	b, err = cfg.GetBoolKey("TEST_STR")
	if err == nil {
		t.Error("EnvProvider_GetBoolKey(): non-bool should return error")
	}

	// attempt to read camelcase
	cfg = NewEnvProvider(envPrefix, true)
	b, err = cfg.GetBoolKey("testCamelCaseBool")
	if err != nil {
		t.Error("EnvProvider_GetBoolKey():", err)
	}
	if b != true {
		t.Error("EnvProvider_GetBoolKey(): value mismatch")
	}
}

func TestEnvProvider_GetConfigNode(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	node, err := cfg.GetConfigNode("TEST_STR")
	if err != config.ErrNotImplemented || node != nil {
		t.Error("EnvProvider_GetConfigNode(): invalid result")
	}
}

func TestEnvProvider_GetFloat64Key(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetFloat64Key("TEST_FLOAT")
	if err != nil {
		t.Error("EnvProvider_GetFloat64Key():", err)
	}

	expected, _ := strconv.ParseFloat(envFloatValue, 64)
	if v != expected {
		t.Error("EnvProvider_GetFloat64Key(): value mismatch")
	}

	// attempt to read invalid value
	v, err = cfg.GetFloat64Key("TEST_STR")
	if err == nil {
		t.Error("EnvProvider_GetFloat64Key(): non-float64 should return error")
	}

	// read camelCase key
	cfg = NewEnvProvider(envPrefix, true)
	v, err = cfg.GetFloat64Key("testCamelCaseFloat")
	if err != nil {
		t.Error("EnvProvider_GetFloat64Key():", err)
	}
}

func TestEnvProvider_GetIntKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

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
	v, err = cfg.GetIntKey("TEST_STR")
	if err == nil {
		t.Error("EnvProvider_GetIntKey(): non-int should return error")
	}

	// read camelCase key
	cfg = NewEnvProvider(envPrefix, true)
	v, err = cfg.GetIntKey("testCamelCaseInt")
	if err != nil {
		t.Error("EnvProvider_GetIntKey():", err)
	}
}

func TestEnvProvider_GetKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

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
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	structRegular := &envStructRegular{}
	if err := cfg.GetKey("test", structRegular); err != nil {
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
	structCamelCase := &envStructCamelCase{}
	if err := cfg.GetKey("test", structCamelCase); err != nil {
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
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetSliceKey("TEST_LIST", ",")
	if err != nil {
		t.Error("EnvProvider_GetSliceKey():", err)
	}
	if !reflect.DeepEqual(v, expectedVar5Value) {
		t.Error("EnvProvider_GetSliceKey(): value mismatch")
	}

	cfg = NewEnvProvider(envPrefix, true)
	v, err = cfg.GetSliceKey("testCamelCaseList", ",")
	if err != nil {
		t.Error("EnvProvider_GetSliceKey():", err)
	}
}

func TestEnvProvider_GetStringKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	v, err := cfg.GetStringKey("TEST_STRING")
	if err != nil {
		t.Error("EnvProvider_GetStringKey():", err)
	}

	if v != envStrValue {
		t.Error("EnvProvider_GetStringKey(): value mismatch")
	}

	cfg = NewEnvProvider(envPrefix, true)
	v, err = cfg.GetStringKey("testCamelCaseString")
	if err != nil {
		t.Error("EnvProvider_GetStringKey():", err)
	}
}

func TestEnvProvider_KeyExists(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	for k, _ := range envVars {
		if !cfg.KeyExists(k) {
			t.Error("EnvProvider_KeyExists(): existing key not found")
		}
	}
	if cfg.KeyExists("NON-EXISTING") {
		t.Error("EnvProvider_KeyExists(): non-existing key mismatch")
	}
	// camelCase
	cfg = NewEnvProvider(envPrefix, true)
	for k, _ := range envVars {
		if !cfg.KeyExists(k) {
			t.Error("EnvProvider_KeyExists(): existing key not found")
		}
	}
	if cfg.KeyExists("NON-EXISTING") {
		t.Error("EnvProvider_KeyExists(): non-existing key mismatch")
	}
}

func TestEnvProvider_KeyListExists(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix, false)
	keys := make([]string, 0)
	for k, _ := range envVars {
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
	for k, _ := range envVars {
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
