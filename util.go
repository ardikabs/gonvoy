package gonvoy

import (
	"encoding/json"
	"time"
)

// MustGetProperty retrieves the value of a property with the given name from the provided RuntimeContext.
// If the property is not found, it returns the default value. If an error occurs during retrieval, it panics.
func MustGetProperty(c RuntimeContext, name, defaultVal string) string {
	value, err := c.GetProperty(name, defaultVal)
	if err != nil {
		panic(err)
	}

	return value
}

// NewMinimalJSONResponse creates a minimal JSON response with the given code, message, and optional errors.
// It returns the JSON response as a byte slice.
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
