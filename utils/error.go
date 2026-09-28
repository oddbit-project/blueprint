package utils

import "reflect"

type Error string

func (e Error) Error() string {
	return string(e)
}

func NotNil(v any, e Error) {
	if v == nil {
		panic(e)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		if rv.IsNil() {
			panic(e)
		}
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
