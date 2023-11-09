package envoy

import (
	"io"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type BufferWriter interface {
	io.Writer

	WriteString(s string) (n int, err error)
	Bytes() []byte
	String() string
}

var _ BufferWriter = &bufferWriter{}

type bufferWriter struct {
	instance api.BufferInstance
}

func (bw *bufferWriter) Write(p []byte) (n int, err error) {
	err = bw.instance.Set(p)
	return bw.instance.Len(), err
}

func (bw *bufferWriter) WriteString(s string) (n int, err error) {
	err = bw.instance.SetString(s)
	return bw.instance.Len(), err
}

func (bw *bufferWriter) String() string {
	return bw.instance.String()
}

func (bw *bufferWriter) Bytes() []byte {
	return bw.instance.Bytes()
}
