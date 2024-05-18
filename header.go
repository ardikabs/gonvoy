package gonvoy

import (
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

const (
	HeaderContentLength      = "Content-Length"
	HeaderContentType        = "Content-Type"
	MIMEApplicationJSON      = "application/json"
	MIMETextPlain            = "text/plain"
	MIMETextPlainCharsetUTF8 = "text/plain" + ";" + charsetUTF8

	HeaderXRequestBodyAccess    = "X-Request-Body-Access"
	HeaderXResponseBodyAccess   = "X-Response-Body-Access"
	ValueXRequestBodyAccessOff  = "Off"
	ValueXResponseBodyAccessOff = "Off"

	HeaderXContentOperation   = "X-Content-Operation"
	ContentOperationReadOnly  = "ReadOnly"
	ContentOperationRO        = "RO" // an initial from ReadOnly
	ContentOperationReadWrite = "ReadWrite"
	ContentOperationRW        = "RW" // an initial from ReadWrite

	charsetUTF8 = "charset=utf-8"
)

// Header represents an HTTP header. It extends the api.HeaderMap interface
// and provides additional methods for working with headers.
type Header interface {
	api.HeaderMap

	// AsMap returns the header as a map of string slices, where each key
	// represents a header field name and the corresponding value is a slice
	// of header field values.
	AsMap() map[string][]string
}

var _ Header = &header{}

type header struct {
	api.HeaderMap
}

func (h *header) AsMap() map[string][]string {
	headers := make(map[string][]string)

	h.HeaderMap.Range(func(key, value string) bool {
		if values, ok := headers[key]; ok {
			headers[key] = append(values, value)
			return true
		}

		headers[key] = []string{value}
		return true
	})

	return headers
}

func NewGatewayHeadersWithEnvoyHeader(envoyheader Header, keysAndValues ...string) http.Header {
	headers := NewGatewayHeaders(keysAndValues...)

	for k, values := range envoyheader.AsMap() {
		headers[http.CanonicalHeaderKey(k)] = values
	}

	return headers
}

func NewGatewayHeaders(keysAndValues ...string) http.Header {
	headers := make(http.Header)
	headers.Add("reporter", "gateway")

	if len(keysAndValues)%2 != 0 {
		return headers
	}

	for n := 0; n < len(keysAndValues); n += 2 {
		headers.Add(keysAndValues[n], keysAndValues[n+1])
	}

	return headers
}
