package gonvoy

import (
	"strconv"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/errs"
	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
)

func TestBodyRead(t *testing.T) {
	b := &bodyWriter{}

	assert.Zero(t, b.Bytes())
	assert.Zero(t, b.String())

	bufferMock := mock_envoy.NewBufferInstance(t)
	bufferMock.EXPECT().String().Return("lorem_ipsum")
	bufferMock.EXPECT().Bytes().Return([]byte("lorem_ipsum"))

	bw := &bodyWriter{
		buffer: bufferMock,
	}

	assert.Equal(t, "lorem_ipsum", bw.String())
	assert.Equal(t, []byte("lorem_ipsum"), bw.Bytes())
}

func TestBody_Write(t *testing.T) {
	input := []byte(`{"name":"John Doe"}`)

	t.Run("body writer is writeable, returns no error", func(t *testing.T) {
		headerMock := mock_envoy.NewRequestHeaderMap(t)
		headerMock.EXPECT().Get(HeaderContentLength).Return("", true)
		headerMock.EXPECT().Set(HeaderContentLength, strconv.Itoa(len(input)))

		bufferMock := mock_envoy.NewBufferInstance(t)
		bufferMock.EXPECT().Set(input).Return(nil)
		bufferMock.EXPECT().Len().Return(len(input))

		writer := &bodyWriter{
			writeable: true,
			header:    headerMock,
			buffer:    bufferMock,
		}

		n, err := writer.Write(input)
		assert.NoError(t, err)
		assert.Equal(t, len(input), n)
	})

	t.Run("body writer is not writeable, returns an error", func(t *testing.T) {
		headerMock := mock_envoy.NewRequestHeaderMap(t)
		bufferMock := mock_envoy.NewBufferInstance(t)
		writer := &bodyWriter{
			header: headerMock,
			buffer: bufferMock,
		}

		n, err := writer.Write(input)
		assert.ErrorIs(t, err, errs.ErrOperationNotPermitted)
		assert.Zero(t, n)
	})
}

func TestBody_WriteString(t *testing.T) {
	input := "new data"

	t.Run("body writer is writeable, returns no error", func(t *testing.T) {
		headerMock := mock_envoy.NewRequestHeaderMap(t)
		headerMock.EXPECT().Get(HeaderContentLength).Return("", true)
		headerMock.EXPECT().Set(HeaderContentLength, strconv.Itoa(len(input)))

		bufferMock := mock_envoy.NewBufferInstance(t)
		bufferMock.EXPECT().SetString(input).Return(nil)
		bufferMock.EXPECT().Len().Return(len(input))

		writer := &bodyWriter{
			writeable: true,
			header:    headerMock,
			buffer:    bufferMock,
		}

		n, err := writer.WriteString(input)
		assert.NoError(t, err)
		assert.Equal(t, len(input), n)
	})

	t.Run("body writer is not writeable, returns an error", func(t *testing.T) {
		headerMock := mock_envoy.NewRequestHeaderMap(t)
		bufferMock := mock_envoy.NewBufferInstance(t)
		writer := &bodyWriter{
			header: headerMock,
			buffer: bufferMock,
		}

		n, err := writer.WriteString(input)
		assert.ErrorIs(t, err, errs.ErrOperationNotPermitted)
		assert.Zero(t, n)
	})
}
