package gonvoy

import (
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

const (
	HeaderContentLength       = "Content-Length"
	HeaderContentType         = "Content-Type"
	HeaderXRequestBodyAccess  = "X-Request-Body-Access"
	HeaderXResponseBodyAccess = "X-Response-Body-Access"
	HeaderXContentOperation   = "X-Content-Operation"

	// Values of X-Request-Body-Access and X-Response-Body-Access
	XRequestBodyAccessOff  = "Off"
	XResponseBodyAccessOff = "Off"

	// Values of X-Content-Operation
	ContentOperationReadOnly  = "ReadOnly"
	ContentOperationRO        = "RO" // an initial from ReadOnly
	ContentOperationReadWrite = "ReadWrite"
	ContentOperationRW        = "RW" // an initial from ReadWrite

	// MIME types
	MIMEApplicationJSON            = "application/json"
	MIMEApplicationJSONCharsetUTF8 = MIMEApplicationJSON + "; " + charsetUTF8
	MIMEApplicationXML             = "application/xml"
	MIMEApplicationXMLCharsetUTF8  = MIMEApplicationXML + "; " + charsetUTF8
	MIMEApplicationGRPC            = "application/grpc"
	MIMETextXML                    = "text/xml"
	MIMETextXMLCharsetUTF8         = MIMETextXML + "; " + charsetUTF8
	MIMEApplicationForm            = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf        = "application/protobuf"
	MIMEApplicationMsgpack         = "application/msgpack"
	MIMETextHTML                   = "text/html"
	MIMETextHTMLCharsetUTF8        = MIMETextHTML + "; " + charsetUTF8
	MIMETextPlain                  = "text/plain"
	MIMETextPlainCharsetUTF8       = MIMETextPlain + "; " + charsetUTF8
	MIMEMultipartForm              = "multipart/form-data"
	MIMEOctetStream                = "application/octet-stream"

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

	clearRouteCache func()
}

func (h *header) Add(key, value string) {
	h.HeaderMap.Add(key, value)

	if h.clearRouteCache != nil {
		h.clearRouteCache()
	}
}

func (h *header) Set(key, value string) {
	h.HeaderMap.Set(key, value)

	if h.clearRouteCache != nil {
		h.clearRouteCache()
	}
}

func (h *header) Del(key string) {
	h.HeaderMap.Del(key)

	if h.clearRouteCache != nil {
		h.clearRouteCache()
	}
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
