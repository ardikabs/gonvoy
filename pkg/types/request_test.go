package types_test

import (
	"testing"

	. "github.com/ardikabs/gonvoy/pkg/types"
	"github.com/stretchr/testify/assert"
)

type headerRangeFuncMock struct {
	header map[string][]string
}

func (h *headerRangeFuncMock) Range(f func(k, v string) bool) {
	for k, values := range h.header {
		for _, v := range values {
			if !f(k, v) {
				return
			}
		}
	}
}

func TestWithRequestHeader(t *testing.T) {
	header := map[string][]string{
		"authorization":   {"foobar"},
		"accesstoken":     {"accesstoken"},
		":authority":      {"example.com"},
		"accept-Language": {"ID"},
	}

	t.Run("direct header", func(t *testing.T) {
		req, err := NewRequest("GET", "example.com", WithRequestHeader(header))

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
		assert.NotEmpty(t, req.Header.Get("Accesstoken"))
		assert.NotEmpty(t, req.Header.Get(":authority"))
		assert.NotEmpty(t, req.Header.Get("Accept-Language"))
	})

	t.Run("from header range interface", func(t *testing.T) {
		headerRangeMock := &headerRangeFuncMock{header}
		req, err := NewRequest("GET", "example.com", WithRequestHeaderRangeSetter(headerRangeMock))

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
		assert.NotEmpty(t, req.Header.Get("Accesstoken"))
		assert.NotEmpty(t, req.Header.Get(":authority"))
		assert.NotEmpty(t, req.Header.Get("Accept-Language"))
	})
}

func TestWithRequestURI(t *testing.T) {
	req, err := NewRequest("GET", "example.com", WithRequestURI("/foobar?token=xyz"))
	assert.NoError(t, err)
	if assert.NotNil(t, req.URL) {
		assert.NotEmpty(t, req.URL.Path)
		assert.NotEmpty(t, req.URL.Query().Get("token"))
	}
}

func TestWithRequestURIError(t *testing.T) {
	_, err := NewRequest("GET", "example.com", WithRequestURI(""))
	assert.Error(t, err)
}
