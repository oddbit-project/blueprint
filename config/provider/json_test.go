package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/oddbit-project/blueprint/config"
	"reflect"
	"testing"
)

type JsonConfigEntry2 struct {
	Var4 string
}
type JsonConfigEntry1 struct {
	Var3 string
	Map2 JsonConfigEntry2
}

type JsonTestConfig struct {
	Var1 int
	Var2 string
	Var5 bool
	Var6 float64
	Var7 []string
	Map1 JsonConfigEntry1
}

const (
	jsonVar1Value = 19
	jsonVar2Value = "some_value"
	jsonVar3Value = "other_value"
	jsonVar4Value = "value4"
	jsonVar5Value = true
	jsonVar6Value = 12.99
)

var jsonExpectedKeys = []string{"Var1", "Var2", "Map1", "Var5", "Var6", "Var7"}

func newJsonConfig() *JsonTestConfig {
	return &JsonTestConfig{
		Var1: jsonVar1Value,
		Var2: jsonVar2Value,
		Var5: jsonVar5Value,
		Var6: jsonVar6Value,
		Var7: jsonExpectedKeys,
		Map1: JsonConfigEntry1{
			Var3: jsonVar3Value,
			Map2: JsonConfigEntry2{
				Var4: jsonVar4Value,
			},
		},
	}
}

func TestNewJsonProvider(t *testing.T) {
	configSource := newJsonConfig()

	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("NewJsonProvider():", err)
	}

	/* Test []byte source */
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Error("NewJsonProvider():", err)
	}
	if cfg == nil {
		t.Error("NewJsonProvider(): invalid result object")
	}

	/* Test io.Reader source */
	buf := bytes.NewBuffer(cfgBuffer)
	cfg, err = NewJsonProvider(buf)
	if err != nil {
		t.Error("NewJsonProvider():", err)
	}
	if cfg == nil {
		t.Error("NewJsonProvider(): invalid result object")
	}

	/* Test json.RawMessage source */
	msg := json.RawMessage{}
	if err := json.Unmarshal(cfgBuffer, &msg); err != nil {
		t.Fatal("NewJsonProvider():", err)
	}
	cfg, err = NewJsonProvider(msg)
	if err != nil {
		t.Error("NewJsonProvider():", err)
	}
	if cfg == nil {
		t.Error("NewJsonProvider(): invalid result object")
	}
}

func TestJsonProvider_GetKey(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetKey():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetKey():", err)
	}

	// read var1
	var var1 int
	if err = cfg.GetKey("Var1", &var1); err != nil {
		t.Error("JsonProvider_GetKey():", err)
	} else {
		if var1 != jsonVar1Value {
			t.Error("JsonProvider_GetKey(): Var1 value mismatch")
		}
	}

	// read var2
	var var2 string
	if err = cfg.GetKey("Var2", &var2); err != nil {
		t.Error("JsonProvider_GetKey():", err)
	} else {
		if var2 != jsonVar2Value {
			t.Error("JsonProvider_GetKey(): Var2 value mismatch")
		}
	}

	// read map1
	cfgMap1 := JsonConfigEntry1{}
	if err = cfg.GetKey("Map1", &cfgMap1); err != nil {
		t.Error("JsonProvider_GetKey():", err)
	} else {
		if cfgMap1.Var3 != jsonVar3Value {
			t.Error("JsonProvider_GetKey(): Var3 value mismatch")
		}
		if cfgMap1.Map2.Var4 != jsonVar4Value {
			t.Error("JsonProvider_GetKey(): Var4 value mismatch")
		}
	}

	// read non-existing value
	if err = cfg.GetKey("Non-existing-key", &cfgMap1); err == nil {
		t.Error("JsonProvider_GetKey(): non-existing key result mismatch")
	} else {
		if err != config.ErrNoKey {
			t.Error("JsonProvider_GetKey(): error type mismatch")
		}
	}
}

func TestJsonProvider_GetStringKey(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetStringKey():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetStringKey():", err)
	}

	// read var2
	var var2 string
	var2, err = cfg.GetStringKey("Var2")
	if err != nil {
		t.Error("JsonProvider_GetStringKey():", err)
	} else {
		if var2 != jsonVar2Value {
			t.Error("JsonProvider_GetStringKey(): Var1 value mismatch")
		}
	}

	// read var1 (not a string)
	_, err = cfg.GetStringKey("Var1")
	if err == nil {
		t.Error("JsonProvider_GetStringKey(): reading non-string does not fail")
	}
}

func TestJsonProvider_GetIntKey(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetIntKey():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetIntKey():", err)
	}

	// read var1
	var var1 int
	var1, err = cfg.GetIntKey("Var1")
	if err != nil {
		t.Error("JsonProvider_GetIntKey():", err)
	} else {
		if var1 != jsonVar1Value {
			t.Error("JsonProvider_GetIntKey(): Var1 value mismatch")
		}
	}

	// read var2 (not an int)
	_, err = cfg.GetIntKey("Var2")
	if err == nil {
		t.Error("JsonProvider_GetIntKey(): reading non-int does not fail")
	}
}

func TestJsonProvider_GetBoolKey(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetBoolKey():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetBoolKey():", err)
	}

	// read var5
	var var5 bool
	var5, err = cfg.GetBoolKey("Var5")
	if err != nil {
		t.Error("JsonProvider_GetBoolKey():", err)
	} else {
		if var5 != jsonVar5Value {
			t.Error("JsonProvider_GetBoolKey(): Var5 value mismatch")
		}
	}

	// read var2 (not a bool)
	_, err = cfg.GetBoolKey("Var2")
	if err == nil {
		t.Error("JsonProvider_GetBoolKey(): reading non-bool does not fail")
	}
}

func TestJsonProvider_GetFloat64Key(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetFloat64Key():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetFloat64Key():", err)
	}

	// read var6
	var var6 float64
	var6, err = cfg.GetFloat64Key("Var6")
	if err != nil {
		t.Error("JsonProvider_GetFloat64Key():", err)
	} else {
		if var6 != jsonVar6Value {
			t.Error("JsonProvider_GetFloat64Key(): Var6 value mismatch")
		}
	}

	// read var2 (not a float/int)
	_, err = cfg.GetIntKey("Var2")
	if err == nil {
		t.Error("JsonProvider_GetFloat64Key(): reading non-float does not fail")
	}
}

func TestJsonProvider_GetSliceKey(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetSliceKey():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetSliceKey():", err)
	}

	// read var6
	var var7 []string
	var7, err = cfg.GetSliceKey("Var7", "")
	if err != nil {
		t.Error("JsonProvider_GetSliceKey():", err)
	} else {
		if !reflect.DeepEqual(var7, jsonExpectedKeys) {
			t.Error("JsonProvider_GetSliceKey(): Var7 value mismatch")
		}
	}
	// read var2 (not a slice)
	_, err = cfg.GetIntKey("Var2")
	if err == nil {
		t.Error("JsonProvider_GetSliceKey(): reading non-slice does not fail")
	}
}

func TestJsonProvider_GetConfigNode(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_GetConfigNode():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_GetConfigNode():", err)
	}

	cfgNode, err := cfg.GetConfigNode("Map1")
	if err != nil {
		t.Fatal("JsonProvider_GetConfigNode():", err)
	}
	if cfgNode == nil {
		t.Fatal("JsonProvider_GetConfigNode(): invalid return type")
	}
	if cfgNode.KeyExists("Var1") {
		t.Fatal("JsonProvider_GetConfigNode(): non-existing key exists")
	}
	if !cfgNode.KeyExists("Var3") {
		t.Fatal("JsonProvider_GetConfigNode(): existing key does not exist")
	}

	// non-existing key
	cfgNode, err = cfg.GetConfigNode("Map9")
	if err == nil {
		t.Fatal("JsonProvider_GetConfigNode(): non-existing key is returning data")
	}
	if !errors.Is(err, config.ErrNoKey) || cfgNode != nil {
		t.Fatal("JsonProvider_GetConfigNode(): error mismatch on non-existing key")
	}
}

func TestJsonProvider_KeyExists(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_KeyExists():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_KeyExists():", err)
	}
	for _, k := range jsonExpectedKeys {
		if !cfg.KeyExists(k) {
			t.Error("JsonProvider_KeyExists(): existing key detection failed")
		}
	}

	for _, k := range []string{"non-existing", "other-non-existing", ""} {
		if cfg.KeyExists(k) {
			t.Error("JsonProvider_KeyExists(): non-existing key detection failed")
		}
	}
}

func TestJsonProvider_KeyListExists(t *testing.T) {
	configSource := newJsonConfig()
	cfgBuffer, err := json.Marshal(configSource)
	if err != nil {
		t.Fatal("JsonProvider_KeyListExists():", err)
	}
	cfg, err := NewJsonProvider(cfgBuffer)
	if err != nil {
		t.Fatal("JsonProvider_KeyListExists():", err)
	}
	if !cfg.KeyListExists(jsonExpectedKeys) {
		t.Error("JsonProvider_KeyListExists(): existing key detection failed")
	}

	var nonExistingKeys []string
	copy(nonExistingKeys, jsonExpectedKeys)
	nonExistingKeys = append(nonExistingKeys, "non-existing")
	if cfg.KeyListExists(nonExistingKeys) {
		t.Error("JsonProvider_KeyListExists(): non-existing key detection failed")
	}
}

// Test for JSON provider default values
func TestJsonProvider_DefaultValues(t *testing.T) {
	// JSON with some missing fields
	jsonData := `{
		"host": "custom.host",
		"port": 9090
	}`
	
	type JsonConfigWithDefaults struct {
		Host    string `json:"host" default:"localhost"`
		Port    int    `json:"port" default:"8080"`
		Debug   bool   `json:"debug" default:"false"`
		Timeout string `json:"timeout" default:"30s"`
	}
	
	cfg, err := NewJsonProvider([]byte(jsonData))
	if err != nil {
		t.Fatal("NewJsonProvider():", err)
	}
	
	config := &JsonConfigWithDefaults{}
	err = cfg.Get(config)
	if err != nil {
		t.Fatal("JsonProvider Get():", err)
	}
	
	// Check that JSON values were used
	if config.Host != "custom.host" {
		t.Error("Host should be from JSON")
	}
	if config.Port != 9090 {
		t.Error("Port should be from JSON")
	}
	
	// Check that defaults were applied for missing fields
	if config.Debug != false {
		t.Error("Debug should use default value")
	}
	if config.Timeout != "30s" {
		t.Error("Timeout should use default value")
	}
}
