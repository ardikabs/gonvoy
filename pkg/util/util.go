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

func NewCopyFromStructOrPointer[T any](in T) (out T, err error) {
	f := reflect.TypeOf(in)
	var v reflect.Value
	switch f.Kind() {
	case reflect.Ptr:
		v = reflect.New(f.Elem())
	case reflect.Struct:
		v = reflect.New(f)
	default:
		return out, fmt.Errorf("data type must be either pointer or literal struct. Got %s instead", f.Name())
	}

	return v.Interface().(T), nil
}
