package gomplate

import (
	"fmt"
	"reflect"
)

func CreateCollFuncs() Namespace {
	return Namespace{"coll", &CollFuncs{}}
}

// Collection Functions.
type CollFuncs struct {
}

func (CollFuncs) Dict(in ...any) (map[string]any, error) {
	dict := map[string]interface{}{}
	lenv := len(in)
	for i := 0; i < lenv; i += 2 {
		key := toString(in[i])
		if i+1 >= lenv {
			dict[key] = ""
			continue
		}
		dict[key] = in[i+1]
	}
	return dict, nil
}

func (CollFuncs) Slice(args ...any) []any {
	return args
}

func (CollFuncs) Append(v any, list any) ([]any, error) {
	l, err := interfaceSlice(list)
	if err != nil {
		return nil, err
	}

	return append(l, v), nil
}

// interfaceSlice converts an array or slice of any type into an []interface{}
// for use in functions that expect this.
func interfaceSlice(slice interface{}) ([]interface{}, error) {
	// avoid all this nonsense if this is already a []interface{}...
	if s, ok := slice.([]interface{}); ok {
		return s, nil
	}
	s := reflect.ValueOf(slice)
	kind := s.Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		l := s.Len()
		ret := make([]interface{}, l)
		for i := 0; i < l; i++ {
			ret[i] = s.Index(i).Interface()
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("expected an array or slice, but got a %T", s)
	}
}
