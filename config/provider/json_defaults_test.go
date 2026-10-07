package provider

import (
	"testing"
)

type issue100Inner struct {
	A int `json:"a"`
}

type issue100Defaults struct {
	Host string `json:"host" default:"localhost"`
}

type issue100Options struct {
	Retries int `json:"retries" default:"3"`
}

// getNoPanic runs Get and turns a panic into a test failure on this line
func getNoPanic(t *testing.T, data string, dest interface{}) error {
	t.Helper()
	p, err := NewJsonProvider([]byte(data))
	if err != nil {
		t.Fatal("NewJsonProvider():", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Get() panicked: %v", r)
		}
	}()
	return p.Get(dest)
}

func TestJsonProvider_Issue100_UnexportedNestedStructDoesNotPanic(t *testing.T) {
	type cfg struct {
		Name string           `json:"name"`
		priv issue100Defaults //nolint:unused // the unexported field under test
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Name != "x" {
		t.Errorf("Name = %q, want x", out.Name)
	}
}

func TestJsonProvider_Issue100_UnexportedPointerDoesNotPanic(t *testing.T) {
	type cfg struct {
		Name string `json:"name"`
		priv *issue100Defaults
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.priv != nil {
		t.Error("unexported pointer was allocated")
	}
}

func TestJsonProvider_Issue100_GetKeyUnexportedNestedStructDoesNotPanic(t *testing.T) {
	type section struct {
		Name string           `json:"name" default:"n"`
		priv issue100Defaults //nolint:unused // the unexported field under test
	}
	p, err := NewJsonProvider([]byte(`{"section":{}}`))
	if err != nil {
		t.Fatal("NewJsonProvider():", err)
	}
	out := &section{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("GetKey() panicked: %v", r)
			}
		}()
		err = p.GetKey("section", out)
	}()
	if err != nil {
		t.Fatal("GetKey():", err)
	}
	if out.Name != "n" {
		t.Errorf("Name = %q, want default n", out.Name)
	}
}

func TestJsonProvider_Issue100_SkippedPointerStaysNil(t *testing.T) {
	type cfg struct {
		Name   string            `json:"name"`
		Native *issue100Defaults `json:"-"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Native != nil {
		t.Error("json:\"-\" pointer was allocated")
	}
}

func TestJsonProvider_Issue100_AbsentSectionWithoutDefaultsStaysNil(t *testing.T) {
	type cfg struct {
		Name     string         `json:"name"`
		Optional *issue100Inner `json:"optional"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Optional != nil {
		t.Error("absent optional section without defaults was allocated")
	}
}

func TestJsonProvider_Issue100_SkippedFieldGetsNoDefault(t *testing.T) {
	type cfg struct {
		Name     string `json:"name"`
		Internal string `json:"-" default:"set"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Internal != "" {
		t.Errorf("json:\"-\" field got default %q", out.Internal)
	}
}

func TestJsonProvider_Issue100_AbsentSectionWithNestedDefaultsIsAllocated(t *testing.T) {
	type middle struct {
		Inner *issue100Defaults `json:"inner"`
	}
	type cfg struct {
		Outer *middle `json:"outer"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Outer == nil || out.Outer.Inner == nil {
		t.Fatal("section with nested defaults was not allocated")
	}
	if out.Outer.Inner.Host != "localhost" {
		t.Errorf("Host = %q, want default localhost", out.Outer.Inner.Host)
	}
}

func TestJsonProvider_Issue100_PresentSectionGetsDefaults(t *testing.T) {
	type section struct {
		Name string `json:"name"`
		Host string `json:"host" default:"localhost"`
	}
	type cfg struct {
		Section *section `json:"section"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"section":{"name":"s"}}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Section == nil || out.Section.Name != "s" {
		t.Fatal("section not decoded")
	}
	if out.Section.Host != "localhost" {
		t.Errorf("Host = %q, want default localhost", out.Section.Host)
	}
}

func TestJsonProvider_Issue100_EmbeddedUnexportedTypeGetsDefaults(t *testing.T) {
	type cfg struct {
		Name string `json:"name"`
		issue100Options
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Retries != 3 {
		t.Errorf("Retries = %d, want default 3", out.Retries)
	}
}

func TestJsonProvider_Issue100_DashNamedFieldGetsDefault(t *testing.T) {
	type cfg struct {
		Dash string `json:"-," default:"d"` //nolint:staticcheck // a field named "-" is the case under test
	}
	out := &cfg{}
	if err := getNoPanic(t, `{}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Dash != "d" {
		t.Errorf("Dash = %q, want default d", out.Dash)
	}
}

type issue100Port int

func TestJsonProvider_Issue100_EmbeddedUnexportedNonStructSkipped(t *testing.T) {
	type cfg struct {
		Name         string `json:"name"`
		issue100Port `default:"5"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.issue100Port != 0 {
		t.Errorf("unexported embedded field got default %d", out.issue100Port)
	}
}

func TestJsonProvider_Issue100_AbsentSectionWithZeroDefaultsStaysNil(t *testing.T) {
	type section struct {
		Debug bool `json:"debug" default:"false"`
		Port  int  `json:"port" default:"0"`
	}
	type cfg struct {
		Section *section `json:"section"`
	}
	out := &cfg{}
	if err := getNoPanic(t, `{}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.Section != nil {
		t.Error("section whose defaults are all zero values was allocated")
	}
}

type issue100Embedded struct {
	Retries int `json:"retries" default:"3"`
}

func TestJsonProvider_Issue100_EmbeddedUnexportedPointerSkipped(t *testing.T) {
	type cfg struct {
		Name string `json:"name"`
		*issue100Embedded
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.issue100Embedded != nil {
		t.Error("embedded pointer to an unexported struct was allocated")
	}
}

func TestJsonProvider_Issue100_UnexportedScalarSkipped(t *testing.T) {
	type cfg struct {
		Name string `json:"name"`
		mode string `default:"m"` //nolint:unused // the unexported field under test
	}
	out := &cfg{}
	if err := getNoPanic(t, `{"name":"x"}`, out); err != nil {
		t.Fatal("Get():", err)
	}
	if out.mode != "" {
		t.Errorf("unexported field got default %q", out.mode)
	}
}
