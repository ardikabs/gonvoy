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

	// ToMap returns the header as a copy of map of string slices,
	// where each key represents a header field name and the corresponding value is a slice
	// of header field values.
	//
	// Any changes made to the returned map will not affect the original headers.
	ToHeaders() http.Header

	// Replace replaces the header with the given headers.
	Replace(headers http.Header)
}

var _ Header = &header{}

type header struct {
	api.HeaderMap
}

func (h *header) ToHeaders() http.Header {
	headers := make(http.Header)

	h.HeaderMap.Range(func(key, value string) bool {
		headers.Add(key, value)
		return true
	})

	return headers
}

func (h *header) Replace(headers http.Header) {
	sets := make(map[string]struct{})

	for key, values := range headers {
		for _, v := range values {
			if _, ok := sets[key]; ok {
				h.Add(key, v)
				continue
			}

			h.Set(key, v)
			sets[key] = struct{}{}
		}
	}
}

func NewGatewayHeadersWithEnvoyHeader(envoyheader Header, keysAndValues ...string) http.Header {
	headers := NewGatewayHeaders(keysAndValues...)

	for key, values := range envoyheader.ToHeaders() {
		headers[key] = values
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
