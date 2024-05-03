package envoy

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"time"
)

func NewMinimalJSONResponse(code, message string, errs ...error) []byte {
	bodyMap := make(map[string]interface{})
	bodyMap["code"] = code
	bodyMap["message"] = message
	bodyMap["errors"] = nil
	bodyMap["data"] = make(map[string]interface{}, 0)
	bodyMap["serverTime"] = time.Now().UnixMilli()

	listErrs := make([]string, len(errs))
	for i, err := range errs {
		listErrs[i] = err.Error()
	}
	bodyMap["errors"] = listErrs

	bodyByte, err := json.Marshal(bodyMap)
	if err != nil {
		bodyByte = []byte("{}")
	}

	return bodyByte
}

func ToFlatHeader(header http.Header) map[string]string {
	flatHeader := make(map[string]string, len(header))

	for k := range header {
		flatHeader[k] = header.Get(k)
	}

	return flatHeader
}

func CastTo(target interface{}, value interface{}) bool {
	t := reflect.ValueOf(target).Elem()
	v := reflect.ValueOf(value)
	if !v.Type().AssignableTo(t.Type()) {
		return false
	}
	t.Set(v)
	return true
}

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

// MustGetProperty is an extended of GetProperty, only panic if value is not in acceptable format.
func MustGetProperty(c Context, name, defaultVal string) string {
	value, err := c.GetProperty(name, defaultVal)
	if err != nil {
		panic(err)
	}

	return value
}

// Clone create new object from source object, copying only the exported fields.
func Clone(in interface{}) interface{} {
	out := reflect.New(reflect.TypeOf(in).Elem())

	val := reflect.ValueOf(in).Elem()
	nVal := out.Elem()
	for i := 0; i < val.NumField(); i++ {
		nvField := nVal.Field(i)
		if nvField.CanSet() {
			nvField.Set(val.Field(i))
		}
	}

	return out.Interface()
}
