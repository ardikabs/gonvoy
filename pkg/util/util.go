package util

import (
	"fmt"
	"reflect"
	"strings"
)

// CastTo casting a value to another target, it basically share the same value or uses the same address.
// A target must be a pointer to be able to receive the value from the source.
func CastTo(target interface{}, source interface{}) bool {
	t := reflect.ValueOf(target).Elem()
	src := reflect.ValueOf(source)
	if !src.Type().AssignableTo(t.Type()) {
		return false
	}
	t.Set(src)
	return true
}

// ReplaceAllEmptySpace replaces all empty space as well as reserved escape characters such as
// tab, newline, carriage return, and so forth.
func ReplaceAllEmptySpace(s string) string {
	replacementMaps := []string{
		" ", "_",
		"\t", "_",
		"\n", "_",
		"\v", "_",
		"\r", "_",
		"\f", "_",
	}

	replacer := strings.NewReplacer(replacementMaps...)

	return replacer.Replace(s)
}

// NewFrom dynamically creates a new variable from the specified data type.
// However, the returned Value's Type is always a PointerTo{dataType}.
func NewFrom(in interface{}) (out interface{}, err error) {
	f := reflect.TypeOf(in)
	var v reflect.Value
	switch f.Kind() {
	case reflect.Ptr:
		v = reflect.New(f.Elem())
	case reflect.Struct:
		v = reflect.New(f)
	default:
		return out, fmt.Errorf("data type must be either pointer to struct or literal struct. Got %s instead", f.Name())
	}

	return v.Interface(), nil
}

func IsNil(i interface{}) bool {
	v := reflect.ValueOf(i)
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// In returns true if a given value exists in the list.
func In[T comparable](value T, list ...T) bool {
	for i := range list {
		if value == list[i] {
			return true
		}
	}
	return false
}

func NotIn[T comparable](value T, list ...T) bool {
	return !In(value, list...)
}
