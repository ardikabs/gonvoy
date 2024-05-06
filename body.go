package gonvoy

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Body interface {
	io.Writer

	Bytes() []byte
	WriteString(s string) (n int, err error)
	String() string
}

var _ Body = &bodyWriter{}

type bodyWriter struct {
	writeable bool
	header    api.HeaderMap
	buffer    api.BufferInstance
}

func (b *bodyWriter) Write(p []byte) (n int, err error) {
	if !b.writeable {
		return 0, fmt.Errorf("body is not writeable, %w", errs.ErrOperationNotPermitted)
	}

	err = b.buffer.Set(p)
	n = b.buffer.Len()
	b.header.Set(HeaderContentLength, strconv.Itoa(n))
	return
}

func (b *bodyWriter) WriteString(s string) (n int, err error) {
	if !b.writeable {
		return 0, fmt.Errorf("body is not writeable, %w", errs.ErrOperationNotPermitted)
	}

	err = b.buffer.SetString(s)
	n = b.buffer.Len()
	b.header.Set(HeaderContentLength, strconv.Itoa(n))
	return
}

func (b *bodyWriter) String() string {
	return b.buffer.String()
}

func (b *bodyWriter) Bytes() []byte {
	return b.buffer.Bytes()
}
