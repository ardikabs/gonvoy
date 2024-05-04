package util_test

import (
	"bytes"
	"testing"

	"github.com/ardikabs/go-envoy/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestReplaceAllEmptySpace(t *testing.T) {
	message := "via Go extension"

	replaced := util.ReplaceAllEmptySpace(message)
	assert.Equal(t, "via_Go_extension", replaced)
}

func TestCastTo(t *testing.T) {
	type dummy struct {
		String string
		Int    int
		Map    map[string]string
		a      string
	}

	sourceStruct := &dummy{
		String: "string",
		Int:    100,
		Map:    map[string]string{"foo": "bar"},
		a:      "blabla",
	}

	destStruct := &dummy{}
	assert.True(t, util.CastTo(&destStruct, sourceStruct))
	assert.Equal(t, sourceStruct, destStruct)

	source := bytes.NewReader([]byte("testing"))
	dest := new(bytes.Reader)
	assert.True(t, util.CastTo(&dest, source))
	assert.Equal(t, source, dest)
}
