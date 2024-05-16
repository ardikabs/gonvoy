package gonvoy

import (
	"encoding/json"
	"time"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
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

// checkBodyAccessibility checks the accessibility of the request/response body based on the provided parameters.
// If strict is false, it determines the accessibility based on the allowRead and allowWrite flags.
// If strict is true, it checks the accessibility based on the operation specified in the header.
// The read and write flags indicate whether the body is readable and writable, respectively.
// The header parameter contains the request/response header information.
func checkBodyAccessibility(strict, allowRead, allowWrite bool, header api.HeaderMap) (read, write bool) {
	access := isBodyAccessible(header)

	if !strict {
		read = access && (allowRead || allowWrite)
		write = access && allowWrite
		return
	}

	operation, ok := header.Get(HeaderXContentOperation)
	if !ok {
		return
	}

	if util.In(operation, ContentOperationReadOnly, ContentOperationRO) {
		read = access && allowRead
		return
	}

	if util.In(operation, ContentOperationReadWrite, ContentOperationRW) {
		write = access && allowWrite
		read = write
		return
	}

	return
}

// isBodyAccessible checks if the body is accessible based on the provided header.
// It returns true if the body is accessible, otherwise false.
func isBodyAccessible(header api.HeaderMap) bool {
	contentLength, ok := header.Get(HeaderContentLength)
	if !ok {
		return false
	}

	isEmpty := contentLength == "" || contentLength == "0"
	return !isEmpty
}

// isRequestBodyAccessible checks if the request body is accessible for reading or writing.
// It returns true if the request body is readable or writeable, otherwise it returns false.
func isRequestBodyAccessible(c Context) bool {
	return c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
}

// isResponseBodyAccessible checks if the response body is accessible.
// It returns true if the response body is readable or writeable, otherwise false.
func isResponseBodyAccessible(c Context) bool {
	return c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
}
