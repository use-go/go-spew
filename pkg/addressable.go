package kew

import "reflect"

// Addressable returns true if the type can be prefixed with &
func Addressable(v reflect.Value) bool {
	switch v.Type().Kind() {
	case reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return true
	}
	return false
}
