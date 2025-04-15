// The code on this file is mostly imported from the clickhouse-go project
// in https://github.com/ClickHouse/clickhouse-go/blob/main/struct_map.go;
// The original license is maintained
//
// Licensed to ClickHouse, Inc. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. ClickHouse, Inc. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package clickhouse

import (
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"reflect"
	"sync"
)

type structMap struct {
	cache sync.Map
}

func (m *structMap) Map(op string, s any, ptr bool) ([]string, []any, error) {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr {
		return nil, nil, &clickhouse.OpError{
			Op:  op,
			Err: fmt.Errorf("must pass a pointer, not a value, to %s destination", op),
		}
	}
	if v.IsNil() {
		return nil, nil, &clickhouse.OpError{
			Op:  op,
			Err: fmt.Errorf("nil pointer passed to %s destination", op),
		}
	}
	t := reflect.TypeOf(s)
	if v = reflect.Indirect(v); t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, nil, &clickhouse.OpError{
			Op:  op,
			Err: fmt.Errorf("%s expects a struct dest", op),
		}
	}

	var index map[string][]int
	switch idx, found := m.cache.Load(t); {
	case found:
		index = idx.(map[string][]int)
	default:
		index = structIdx(t)
		m.cache.Store(t, index)
	}

	cols := make([]string, 0, len(index))
	values := make([]any, 0, len(index))
	for c, idx := range index {
		cols = append(cols, c)
		switch field := v.FieldByIndex(idx); {
		case ptr:
			values = append(values, field.Addr().Interface())
		default:
			values = append(values, field.Interface())
		}
	}
	return cols, values, nil
}

func structIdx(t reflect.Type) map[string][]int {
	fields := make(map[string][]int)
	for i := 0; i < t.NumField(); i++ {
		var (
			f    = t.Field(i)
			name = f.Name
		)
		if tn := f.Tag.Get("ch"); len(tn) != 0 {
			name = tn
		}
		switch {
		case name == "-", len(f.PkgPath) != 0 && !f.Anonymous:
			continue
		}
		switch {
		case f.Anonymous:
			if f.Type.Kind() != reflect.Ptr {
				for k, idx := range structIdx(f.Type) {
					fields[k] = append(f.Index, idx...)
				}
			}
		default:
			fields[name] = f.Index
		}
	}
	return fields
}
