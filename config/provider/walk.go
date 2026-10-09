package provider

import "reflect"

// structPath holds the structs on the current path of a config walk, so that a walk into a
// recursive config type, or into a cycle of pointers, terminates
type structPath struct {
	types    map[reflect.Type]int
	addrs    map[structAddr]bool
	sections map[structSection]int
}

type structAddr struct {
	t reflect.Type
	p uintptr
}

// structSection is a struct type read under a key prefix
type structSection struct {
	t      reflect.Type
	prefix string
}

func newStructPath() *structPath {
	return &structPath{
		types:    make(map[reflect.Type]int),
		addrs:    make(map[structAddr]bool),
		sections: make(map[structSection]int),
	}
}

// push records the struct v, read under prefix, on the path and returns the function that
// removes it
func (p *structPath) push(v reflect.Value, prefix string) func() {
	t := v.Type()
	p.types[t]++
	section := structSection{t: t, prefix: prefix}
	p.sections[section]++
	addressable := v.CanAddr()
	var key structAddr
	if addressable {
		key = structAddr{t: t, p: v.Addr().Pointer()}
		p.addrs[key] = true
	}
	return func() {
		if p.types[t]--; p.types[t] == 0 {
			delete(p.types, t)
		}
		if p.sections[section]--; p.sections[section] == 0 {
			delete(p.sections, section)
		}
		if addressable {
			delete(p.addrs, key)
		}
	}
}

// hasType reports whether a struct of type t is on the path
func (p *structPath) hasType(t reflect.Type) bool {
	return p.types[t] > 0
}

// hasSection reports whether a struct of type t read under prefix is on the path
func (p *structPath) hasSection(t reflect.Type, prefix string) bool {
	return p.sections[structSection{t: t, prefix: prefix}] > 0
}

// hasPointer reports whether the struct ptr points to is on the path
func (p *structPath) hasPointer(ptr reflect.Value) bool {
	return p.addrs[structAddr{t: ptr.Type().Elem(), p: ptr.Pointer()}]
}
