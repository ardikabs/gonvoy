package gonvoy

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// Body represents the body of an HTTP request or response.
type Body interface {
	io.Writer

	// Bytes returns the body content as a byte slice.
	Bytes() []byte

	// WriteString writes a string to the body and returns the number of bytes written and any error encountered.
	WriteString(s string) (n int, err error)

	// String returns the body content as a string.
	String() string
}

var _ Body = &bodyWriter{}

type bodyWriter struct {
	writable bool

	bytes  []byte
	buffer api.BufferInstance

	header                api.HeaderMap
	preserveContentLength bool
}

func (b *bodyWriter) Write(p []byte) (n int, err error) {
	if !b.writable {
		return 0, fmt.Errorf("body is not writable, %w", errs.ErrOperationNotPermitted)
	}

	err = b.buffer.Set(p)
	n = b.buffer.Len()

	b.resetContentLength()
	return
}

func (b *bodyWriter) WriteString(s string) (n int, err error) {
	if !b.writable {
		return 0, fmt.Errorf("body is not writable, %w", errs.ErrOperationNotPermitted)
	}

	err = b.buffer.SetString(s)
	n = b.buffer.Len()

	b.resetContentLength()
	return
}

func (b *bodyWriter) String() string {
	if b.buffer == nil {
		return ""
	}

	return string(b.bytes)
}

func (b *bodyWriter) Bytes() []byte {
	if b.buffer == nil {
		return nil
	}

	return b.bytes
}

func (b *bodyWriter) resetContentLength() {
	if !b.preserveContentLength {
		// if content-length is not preserved, do nothing.
		return
	}

	if _, ok := b.header.Get(HeaderContentLength); ok {
		b.header.Set(HeaderContentLength, strconv.Itoa(b.buffer.Len()))
	}
}
