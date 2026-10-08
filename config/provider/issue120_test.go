package provider

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue120Node struct {
	Name string        `json:"name" env:"NAME"`
	Next *issue120Node `json:"next" env:"NEXT"`
}

type issue120DefNode struct {
	Name string           `json:"name" env:"NAME" default:"n"`
	Next *issue120DefNode `json:"next" env:"NEXT"`
}

type issue120CamelNode struct {
	NodeName string
	NextNode *issue120CamelNode
}

type issue120A struct {
	Name string     `json:"name" env:"NAME"`
	B    *issue120B `json:"b" env:"B"`
}

type issue120B struct {
	Name string     `json:"name" env:"NAME"`
	A    *issue120A `json:"a" env:"A"`
}

type issue120DefA struct {
	Name string        `json:"name" env:"NAME" default:"a"`
	B    *issue120DefB `json:"b" env:"B"`
}

type issue120DefB struct {
	Name string        `json:"name" env:"NAME" default:"b"`
	A    *issue120DefA `json:"a" env:"A"`
}

// issue120CapStack makes an unbounded walk fail fast instead of exhausting memory
func issue120CapStack(t *testing.T) {
	t.Helper()
	old := debug.SetMaxStack(16 << 20)
	t.Cleanup(func() { debug.SetMaxStack(old) })
}

func issue120EnvGet(t *testing.T, vars map[string]string, convertCase bool, dest interface{}) error {
	t.Helper()
	issue120CapStack(t)
	t.Cleanup(func() { resetEnvVars(vars) })
	setEnvVars(t, vars)
	return NewEnvProvider("ZZ120_", convertCase).Get(dest)
}

func issue120JsonGet(t *testing.T, data string, dest interface{}) error {
	t.Helper()
	issue120CapStack(t)
	return getNoPanic(t, data, dest)
}

// claim 1
func TestEnvProvider_Issue120_SelfReferentialTypeTerminates(t *testing.T) {
	out := &issue120DefNode{}
	require.NoError(t, issue120EnvGet(t, nil, false, out))
	assert.Equal(t, "n", out.Name)
	assert.Nil(t, out.Next, "the nil self-pointer was allocated")
}

// claim 2
func TestJsonProvider_Issue120_SelfReferentialTypeTerminates(t *testing.T) {
	out := &issue120DefNode{}
	require.NoError(t, issue120JsonGet(t, `{}`, out))
	assert.Equal(t, "n", out.Name)
	assert.Nil(t, out.Next, "the nil self-pointer was allocated")
}

// claim 3
func TestEnvProvider_Issue120_MutualRecursionTerminates(t *testing.T) {
	out := &issue120A{}
	require.NoError(t, issue120EnvGet(t, nil, false, out))
	assert.Nil(t, out.B)
}

// claim 7
func TestEnvProvider_Issue120_MutualRecursionSectionGetsDefaultsOnce(t *testing.T) {
	def := &issue120DefA{}
	require.NoError(t, issue120EnvGet(t, nil, false, def))
	require.NotNil(t, def.B, "a section with non-zero defaults, not yet on the path, was left nil")
	assert.Equal(t, "b", def.B.Name)
	assert.Nil(t, def.B.A, "a section whose type is already on the path was allocated")
}

// claim 3
func TestJsonProvider_Issue120_MutualRecursionTerminates(t *testing.T) {
	out := &issue120A{}
	require.NoError(t, issue120JsonGet(t, `{}`, out))
	assert.Nil(t, out.B)
}

// claim 7
func TestJsonProvider_Issue120_MutualRecursionSectionGetsDefaultsOnce(t *testing.T) {
	def := &issue120DefA{}
	require.NoError(t, issue120JsonGet(t, `{}`, def))
	require.NotNil(t, def.B, "a section with non-zero defaults, not yet on the path, was left nil")
	assert.Equal(t, "b", def.B.Name)
	assert.Nil(t, def.B.A, "a section whose type is already on the path was allocated")
}

// claim 4
func TestEnvProvider_Issue120_CallerBuiltCycleTerminates(t *testing.T) {
	n := &issue120DefNode{}
	n.Next = n
	require.NoError(t, issue120EnvGet(t, nil, false, n))
	assert.Equal(t, "n", n.Name)
	assert.Same(t, n, n.Next)

	a, b := &issue120DefNode{}, &issue120DefNode{}
	a.Next, b.Next = b, a
	require.NoError(t, issue120EnvGet(t, nil, false, a))
	assert.Equal(t, "n", a.Name)
	assert.Equal(t, "n", b.Name, "the second node of a cycle was not walked")
	assert.Same(t, b, a.Next)
	assert.Same(t, a, b.Next)
}

// claim 4
func TestJsonProvider_Issue120_CallerBuiltCycleTerminates(t *testing.T) {
	n := &issue120DefNode{}
	n.Next = n
	require.NoError(t, issue120JsonGet(t, `{}`, n))
	assert.Equal(t, "n", n.Name)
	assert.Same(t, n, n.Next)

	a, b := &issue120DefNode{}, &issue120DefNode{}
	a.Next, b.Next = b, a
	require.NoError(t, issue120JsonGet(t, `{}`, a))
	assert.Equal(t, "n", a.Name)
	assert.Equal(t, "n", b.Name, "the second node of a cycle was not walked")
	assert.Same(t, b, a.Next)
	assert.Same(t, a, b.Next)
}

// claim 5
func TestEnvProvider_Issue120_VariableFillsRecursiveSection(t *testing.T) {
	out := &issue120Node{}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_NEXT_NAME": "b"}, false, out))
	require.NotNil(t, out.Next, "a variable inside a recursive section did not allocate it")
	assert.Equal(t, "b", out.Next.Name)
	assert.Nil(t, out.Next.Next)
}

// claim 5: a variable two levels down allocates both sections
func TestEnvProvider_Issue120_DeepVariableFillsRecursiveSections(t *testing.T) {
	deep := &issue120Node{}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_NEXT_NEXT_NAME": "c"}, false, deep))
	require.NotNil(t, deep.Next)
	require.NotNil(t, deep.Next.Next, "a variable two levels down did not allocate its section")
	assert.Equal(t, "c", deep.Next.Next.Name)
	assert.Nil(t, deep.Next.Next.Next)
}

// claim 5, with camelCase conversion
func TestEnvProvider_Issue120_VariableFillsRecursiveSectionConvertCase(t *testing.T) {
	out := &issue120CamelNode{}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_NEXT_NODE_NEXT_NODE_NODE_NAME": "c"}, true, out))
	require.NotNil(t, out.NextNode)
	require.NotNil(t, out.NextNode.NextNode, "a converted variable two levels down did not allocate its section")
	assert.Equal(t, "c", out.NextNode.NextNode.NodeName)
	assert.Nil(t, out.NextNode.NextNode.NextNode)
}

// claim 5: a zero-valued variable still allocates a recursive section
func TestEnvProvider_Issue120_ZeroValuedVariableFillsRecursiveSection(t *testing.T) {
	out := &issue120UnwrittenNode{}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_NEXT_PORT": "0"}, false, out))
	require.NotNil(t, out.Next, "a zero-valued variable inside a recursive section did not allocate it")
	assert.Equal(t, 0, out.Next.Port)
	assert.Nil(t, out.Next.Next)
}

type issue120CollapsedNode struct {
	Name string
	Next *issue120CollapsedNode `env:"-"`
}

// claim 8: "HEAD_-" converts back to "HEAD", so the key of the recursive section does not grow
func TestEnvProvider_Issue120_CollapsingKeyTerminates(t *testing.T) {
	type cfg struct {
		Head *issue120CollapsedNode `env:"HEAD"`
	}
	out := &cfg{}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_HEAD_NAME": "h"}, true, out))
	require.NotNil(t, out.Head)
	assert.Equal(t, "h", out.Head.Name)
	assert.Nil(t, out.Head.Next)
}

type issue120UnwrittenNode struct {
	Name    string                 `env:"NAME" default:"n"`
	Port    int                    `env:"PORT"`
	NextHop string                 `env:"NEXT_HOP"`
	Next    *issue120UnwrittenNode `env:"NEXT"`
}

// claim 9
func TestEnvProvider_Issue120_UnwrittenVariableLeavesRecursiveSectionNil(t *testing.T) {
	out := &issue120UnwrittenNode{}
	require.NoError(t, issue120EnvGet(t, map[string]string{
		"ZZ120_NEXT_HOP":   "x",
		"ZZ120_NEXT_PORT":  "abc",
		"ZZ120_NEXT_BOGUS": "1",
		"ZZ120_NEXT_NEXT":  "x",
	}, false, out))
	assert.Equal(t, "x", out.NextHop)
	assert.Equal(t, "n", out.Name)
	assert.Nil(t, out.Next, "a variable that writes nothing allocated a recursive section through its defaults")
}

// claim 6
func TestEnvProvider_Issue120_CallerChainGetsDefaults(t *testing.T) {
	out := &issue120DefNode{Next: &issue120DefNode{}}
	require.NoError(t, issue120EnvGet(t, nil, false, out))
	assert.Equal(t, "n", out.Name)
	require.NotNil(t, out.Next)
	assert.Equal(t, "n", out.Next.Name, "the second node of a caller-supplied chain got no defaults")
	assert.Nil(t, out.Next.Next)
}

// claim 6
func TestJsonProvider_Issue120_CallerChainGetsDefaults(t *testing.T) {
	out := &issue120DefNode{Next: &issue120DefNode{}}
	require.NoError(t, issue120JsonGet(t, `{}`, out))
	assert.Equal(t, "n", out.Name)
	require.NotNil(t, out.Next)
	assert.Equal(t, "n", out.Next.Name, "the second node of a caller-supplied chain got no defaults")
	assert.Nil(t, out.Next.Next)
}

type issue120Holder struct {
	Head *issue120DefNode `json:"head" env:"HEAD"`
	Tail *issue120DefNode `json:"tail" env:"TAIL"`
}

// claim 7
func TestEnvProvider_Issue120_RecursiveSectionGetsDefaultsOnce(t *testing.T) {
	out := &issue120Holder{}
	require.NoError(t, issue120EnvGet(t, nil, false, out))
	for name, s := range map[string]*issue120DefNode{"Head": out.Head, "Tail": out.Tail} {
		require.NotNil(t, s, "%s: a recursive section with non-zero defaults was left nil", name)
		assert.Equal(t, "n", s.Name, name)
		assert.Nil(t, s.Next, name)
	}
}

// claim 7
func TestJsonProvider_Issue120_RecursiveSectionGetsDefaultsOnce(t *testing.T) {
	out := &issue120Holder{}
	require.NoError(t, issue120JsonGet(t, `{}`, out))
	for name, s := range map[string]*issue120DefNode{"Head": out.Head, "Tail": out.Tail} {
		require.NotNil(t, s, "%s: a recursive section with non-zero defaults was left nil", name)
		assert.Equal(t, "n", s.Name, name)
		assert.Nil(t, s.Next, name)
	}
}

// guard G1: a pointer shared by two fields gets both fields' variables
func TestEnvProvider_Issue120_SharedPointerGetsBothFields(t *testing.T) {
	type cfg struct {
		X *issue107Section `env:"X"`
		Y *issue107Section `env:"Y"`
	}
	s := &issue107Section{}
	out := &cfg{X: s, Y: s}
	require.NoError(t, issue120EnvGet(t, map[string]string{"ZZ120_X_HOST": "h", "ZZ120_Y_PORT": "5"}, false, out))
	assert.Equal(t, "h", s.Host)
	assert.Equal(t, 5, s.Port, "the second field sharing a pointer was skipped")
}

// guard G2: sibling sections of one type are both allocated for their defaults
func TestEnvProvider_Issue120_SiblingSectionsOfOneType(t *testing.T) {
	type cfg struct {
		A *issue107Defaults `env:"A"`
		B *issue107Defaults `env:"B"`
	}
	out := &cfg{}
	require.NoError(t, issue120EnvGet(t, nil, false, out))
	require.NotNil(t, out.A)
	require.NotNil(t, out.B, "a sibling section of the same type was treated as recursive")
	assert.Equal(t, "localhost", out.B.Host)
}

// guard G2
func TestJsonProvider_Issue120_SiblingSectionsOfOneType(t *testing.T) {
	type cfg struct {
		A *issue100Defaults `json:"a"`
		B *issue100Defaults `json:"b"`
	}
	out := &cfg{}
	require.NoError(t, issue120JsonGet(t, `{}`, out))
	require.NotNil(t, out.A)
	require.NotNil(t, out.B, "a sibling section of the same type was treated as recursive")
	assert.Equal(t, "localhost", out.B.Host)
}
