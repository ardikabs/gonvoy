package types_test

import (
	"net/http"
	"testing"

	. "github.com/ardikabs/gaetway/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestResponse(t *testing.T) {
	header := map[string][]string{
		"content-length": {"97"},
		"content-type":   {"text/plain"},
		"date":           {"Sat, 07 Oct 2023 15:21:38 GMT"},
		"server":         {"envoy"},
	}

	headerRangeMock := &headerRangeFuncMock{header}
	res, err := NewResponse(http.StatusOK, WithResponseHeaderRangeSetter(headerRangeMock))
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "envoy", res.Header.Get("SERVER"))
	assert.Equal(t, "text/plain", res.Header.Get("content-Type"))
}
