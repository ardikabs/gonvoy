package envoy

import (
	"encoding/json"
	"net/http"
	"reflect"
	"time"
)

func CreateSimpleJSONBody(code, message string, errs ...error) []byte {
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
