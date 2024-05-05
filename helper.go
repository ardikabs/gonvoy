package envoy

import (
	"encoding/json"
	"time"
)

// MustGetProperty is an extended of GetProperty, only panic if value is not in acceptable format.
func MustGetProperty(c Context, name, defaultVal string) string {
	value, err := c.GetProperty(name, defaultVal)
	if err != nil {
		panic(err)
	}

	return value
}

// NewMinimalJSONResponse creates a minimal JSON body as a form of bytes.
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
