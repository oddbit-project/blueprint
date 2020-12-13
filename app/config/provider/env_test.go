package provider

import (
	"github.com/oddbit-project/blueprint/app/config"
	"os"
	"reflect"
	"strconv"
	"testing"
)

const (
	envPrefix    = "TEST_"
	envVar1Value = "simple string"
	envVar2Value = "true"
	envVar3Value = "45"
	envVar4Value = "72.95"
	envVar5Value = "A, b,C,d"
)

var expectedVar5Value = []string{"A", "b", "C", "d"}

var envVars = map[string]string{
	"TEST_VAR1": envVar1Value,
	"TEST_VAR2": envVar2Value,
	"TEST_VAR3": envVar3Value,
	"TEST_VAR4": envVar4Value,
	"TEST_VAR5": envVar5Value,
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

	cfg := NewEnvProvider(envPrefix)
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

	cfg := NewEnvProvider(envPrefix)
	b, err := cfg.GetBoolKey("TEST_VAR2")
	if err != nil {
		t.Error("EnvProvider_GetBoolKey():", err)
	}
	if b != true {
		t.Error("EnvProvider_GetBoolKey(): value mismatch")
	}

	// attempt to read invalid value
	b, err = cfg.GetBoolKey("TEST_VAR1")
	if err == nil {
		t.Error("EnvProvider_GetBoolKey(): non-bool should return error")
	}
}

func TestEnvProvider_GetConfigNode(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
	node, err := cfg.GetConfigNode("TEST_VAR1")
	if err != config.ErrNotImplemented || node != nil {
		t.Error("EnvProvider_GetConfigNode(): invalid result")
	}
}

func TestEnvProvider_GetFloat64Key(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
	v, err := cfg.GetFloat64Key("TEST_VAR4")
	if err != nil {
		t.Error("EnvProvider_GetFloat64Key():", err)
	}

	expected, _ := strconv.ParseFloat(envVar4Value, 64)
	if v != expected {
		t.Error("EnvProvider_GetFloat64Key(): value mismatch")
	}

	// attempt to read invalid value
	v, err = cfg.GetFloat64Key("TEST_VAR1")
	if err == nil {
		t.Error("EnvProvider_GetFloat64Key(): non-float64 should return error")
	}
}

func TestEnvProvider_GetIntKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
	v, err := cfg.GetIntKey("TEST_VAR3")
	if err != nil {
		t.Error("EnvProvider_GetIntKey():", err)
	}

	expected, _ := strconv.Atoi(envVar3Value)
	if v != expected {
		t.Error("EnvProvider_GetIntKey(): value mismatch")
	}

	// attempt to read invalid value
	v, err = cfg.GetIntKey("TEST_VAR1")
	if err == nil {
		t.Error("EnvProvider_GetIntKey(): non-int should return error")
	}
}

func TestEnvProvider_GetKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)

	var var1 string
	var var2 bool
	var var3 int
	var var4 float64
	var var5 []string
	var err error

	// string
	if err = cfg.GetKey("TEST_VAR1", &var1); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		if var1 != envVar1Value {
			t.Error("EnvProvider_GetKey(): string value mismatch")
		}
	}

	// bool
	if err = cfg.GetKey("TEST_VAR2", &var2); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		b, _ := strconv.ParseBool(envVar2Value)
		if var2 != b {
			t.Error("EnvProvider_GetKey(): bool value mismatch")
		}
	}

	// int
	if err = cfg.GetKey("TEST_VAR3", &var3); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		i, _ := strconv.Atoi(envVar3Value)
		if var3 != i {
			t.Error("EnvProvider_GetKey(): int value mismatch")
		}
	}

	// float64
	if err = cfg.GetKey("TEST_VAR4", &var4); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		f, _ := strconv.ParseFloat(envVar4Value, 64)
		if var4 != f {
			t.Error("EnvProvider_GetKey(): float value mismatch")
		}
	}

	// string slice
	if err = cfg.GetKey("TEST_VAR5", &var5); err != nil {
		t.Error("EnvProvider_GetKey():", err)
	} else {
		if !reflect.DeepEqual(var5, expectedVar5Value) {
			t.Error("EnvProvider_GetKey(): string slice value mismatch")
		}
	}
}

func TestEnvProvider_GetSliceKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
	v, err := cfg.GetSliceKey("TEST_VAR5", ",")
	if err != nil {
		t.Error("EnvProvider_GetSliceKey():", err)
	}
	if !reflect.DeepEqual(v, expectedVar5Value) {
		t.Error("EnvProvider_GetSliceKey(): value mismatch")
	}
}

func TestEnvProvider_GetStringKey(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
	v, err := cfg.GetStringKey("TEST_VAR1")
	if err != nil {
		t.Error("EnvProvider_GetStringKey():", err)
	}

	if v != envVar1Value {
		t.Error("EnvProvider_GetStringKey(): value mismatch")
	}
}

func TestEnvProvider_KeyExists(t *testing.T) {
	setEnvVars(t)
	defer resetEnvVars()

	cfg := NewEnvProvider(envPrefix)
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

	cfg := NewEnvProvider(envPrefix)
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
}
