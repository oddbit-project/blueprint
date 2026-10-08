package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue107Defaults struct {
	Host string `env:"HOST" default:"localhost"`
}

type issue107Section struct {
	Host string `env:"HOST"`
	Port int    `env:"PORT"`
}

type issue107ZeroDefaults struct {
	Port int `env:"PORT" default:"0"`
}

type issue107Plain struct {
	X int `env:"X"`
}

type issue107Embedded struct {
	Host string `env:"HOST"`
}

// envGetNoPanic sets vars, runs Get and turns a panic into a test failure on this line
func envGetNoPanic(t *testing.T, vars map[string]string, dest interface{}) error {
	t.Helper()
	t.Cleanup(func() { resetEnvVars(vars) })
	setEnvVars(t, vars)
	p := NewEnvProvider("ZZ107_", false)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Get() panicked: %v", r)
		}
	}()
	return p.Get(dest)
}

func TestEnvProvider_Issue107_UnexportedNestedStructDoesNotPanic(t *testing.T) {
	type cfg struct {
		Name string           `env:"NAME"`
		priv issue107Defaults //nolint:unused // the unexported field under test
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x"}, out))
	assert.Equal(t, "x", out.Name)
}

func TestEnvProvider_Issue107_UnexportedPointerDoesNotPanic(t *testing.T) {
	type cfg struct {
		Name string `env:"NAME"`
		priv *issue107Defaults
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x"}, out))
	assert.Nil(t, out.priv, "unexported pointer was allocated")
}

func TestEnvProvider_Issue107_UnexportedScalarGetsNoDefault(t *testing.T) {
	type cfg struct {
		Name string `env:"NAME"`
		mode string `default:"m"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x"}, out))
	assert.Empty(t, out.mode)
}

func TestEnvProvider_Issue107_UnexportedScalarIgnoresVariable(t *testing.T) {
	type cfg struct {
		Name string `env:"NAME"`
		mode string `env:"MODE"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x", "ZZ107_MODE": "y"}, out))
	assert.Empty(t, out.mode)
}

func TestEnvProvider_Issue107_AbsentSectionStaysNil(t *testing.T) {
	type cfg struct {
		Name     string                `env:"NAME"`
		Database *issue107Section      `env:"DATABASE"`
		Native   *issue107Plain        `env:"NATIVE"`
		Zero     *issue107ZeroDefaults `env:"ZERO"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x"}, out))
	assert.Nil(t, out.Database)
	assert.Nil(t, out.Native)
	assert.Nil(t, out.Zero, "a section whose defaults are all zero values was allocated")
}

func TestEnvProvider_Issue107_AbsentSectionWithNonZeroDefaultIsAllocated(t *testing.T) {
	type cfg struct {
		Name     string            `env:"NAME"`
		Database *issue107Defaults `env:"DATABASE"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_NAME": "x"}, out))
	require.NotNil(t, out.Database)
	assert.Equal(t, "localhost", out.Database.Host)
}

func TestEnvProvider_Issue107_EmbeddedUnexportedTypeIsRead(t *testing.T) {
	type cfg struct {
		issue107Embedded
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_ISSUE107EMBEDDED_HOST": "h"}, out))
	assert.Equal(t, "h", out.Host)
}

func TestEnvProvider_Issue107_ZeroValuedVariableAllocatesSection(t *testing.T) {
	type cfg struct {
		Database *issue107Section `env:"DATABASE"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_DATABASE_PORT": "0"}, out))
	require.NotNil(t, out.Database, "a section with a variable present was left nil")
	assert.Equal(t, 0, out.Database.Port)
}

func TestEnvProvider_Issue107_PresentSectionIsFilled(t *testing.T) {
	type cfg struct {
		Database *issue107Section `env:"DATABASE"`
	}
	out := &cfg{Database: &issue107Section{Port: 1}}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_DATABASE_HOST": "h"}, out))
	assert.Equal(t, "h", out.Database.Host)
	assert.Equal(t, 1, out.Database.Port)
}

type issue107Outer struct {
	Value issue107Section  `env:"VALUE"`
	Inner *issue107Section `env:"INNER"`
}

// a variable two levels down allocates every nil section above it
func TestEnvProvider_Issue107_NestedVariableAllocatesParents(t *testing.T) {
	type cfg struct {
		ByValue   *issue107Outer `env:"BYVALUE"`
		ByPointer *issue107Outer `env:"BYPOINTER"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{
		"ZZ107_BYVALUE_VALUE_PORT":   "5",
		"ZZ107_BYPOINTER_INNER_HOST": "h",
	}, out))
	require.NotNil(t, out.ByValue, "a variable in a nested struct value did not allocate its section")
	assert.Equal(t, 5, out.ByValue.Value.Port)
	require.NotNil(t, out.ByPointer, "a variable in a nested pointer section did not allocate its parent")
	require.NotNil(t, out.ByPointer.Inner)
	assert.Equal(t, "h", out.ByPointer.Inner.Host)
}

type issue107Unwritable struct {
	Count uint `env:"COUNT"`
	Port  int  `env:"PORT"`
}

// a variable that writes nothing (unsupported kind, invalid value, or the key of a section
// itself) does not allocate its section
func TestEnvProvider_Issue107_UnwrittenVariableLeavesSectionNil(t *testing.T) {
	type cfg struct {
		Kind    *issue107Unwritable `env:"KIND"`
		Invalid *issue107Unwritable `env:"INVALID"`
		Outer   *issue107Outer      `env:"OUTER"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{
		"ZZ107_KIND_COUNT":   "3",
		"ZZ107_INVALID_PORT": "abc",
		"ZZ107_OUTER_INNER":  "x",
	}, out))
	assert.Nil(t, out.Kind, "an unsupported kind allocated its section")
	assert.Nil(t, out.Invalid, "an invalid value allocated its section")
	assert.Nil(t, out.Outer, "the key of a nested section allocated its parent")
}

// the key of a section itself writes nothing, and its fields are still read
func TestEnvProvider_Issue107_SectionKeyDoesNotHideItsFields(t *testing.T) {
	type cfg struct {
		Database *issue107Section `env:"DATABASE"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{
		"ZZ107_DATABASE":      "x",
		"ZZ107_DATABASE_HOST": "h",
	}, out))
	require.NotNil(t, out.Database)
	assert.Equal(t, "h", out.Database.Host)
}

// a slice of a non-string element type is a kind the provider does not parse
func TestEnvProvider_Issue107_NonStringSliceIsIgnored(t *testing.T) {
	type cfg struct {
		Ports []int `env:"PORTS"`
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, map[string]string{"ZZ107_PORTS": "1,2"}, out))
	assert.Nil(t, out.Ports)
}

func TestEnvProvider_Issue107_EmbeddedUnexportedPointerIsSkipped(t *testing.T) {
	type cfg struct {
		*issue107Defaults
	}
	out := &cfg{}
	require.NoError(t, envGetNoPanic(t, nil, out))
	assert.Nil(t, out.issue107Defaults, "an embedded pointer to an unexported type was allocated")
}
