package spew

import (
	"reflect"
)

func (d *dumpState) isZero(v reflect.Value) bool {
	kind := v.Kind()
	if kind == reflect.Invalid {
		return false
	}

	// Handle pointers specially.
	if kind == reflect.Ptr {
		ve := v
		for ve.Kind() == reflect.Ptr {
			if ve.IsNil() {
				return true
			}
			return false
		}
	}

	switch kind {
	case reflect.Invalid:
		return false
	case reflect.Bool:
		return v.Bool() == false
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return v.Int() == 0
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return v.Uint() == 0
	case reflect.Float32:
		return v.Float() == 0
	case reflect.Float64:
		return v.Float() == 0
	case reflect.Slice:
		return v.IsNil()
	case reflect.Array:
		return v.Len() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Interface:
		return v.IsNil()
	case reflect.Map:
		return v.IsNil()
	case reflect.Struct:
		numFields := v.NumField()
		for i := 0; i < numFields; i++ {
			if !d.isZero(d.unpackValue(v.Field(i))) {
				return false
			}
		}
		return true
	case reflect.Uintptr:
		num := uint64(v.Uint())
		return num == 0
	}

	return false
}
