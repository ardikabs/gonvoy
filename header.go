package gonvoy

import "net/http"

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

// NewGatewayHeaders returns default gateway headers with optional
// key-value pairs appended after the reporter marker.
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
