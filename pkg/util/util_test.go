package util_test

import (
	"bytes"
	"testing"

	"github.com/ardikabs/go-envoy/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewFrom(t *testing.T) {
	type dummyStruct struct {
		A string
		B string
		c string
	}

	src := dummyStruct{
		A: "foo",
		B: "bar",
		c: "foobar_private",
	}

	dest, err := util.NewFrom(src)
	require.NoError(t, err)
	assert.IsType(t, &dummyStruct{}, dest)
	assert.Zero(t, dest.(*dummyStruct).A)
	assert.Zero(t, dest.(*dummyStruct).B)
	assert.Zero(t, dest.(*dummyStruct).c)

	srcPtr := &dummyStruct{
		A: "foo",
		B: "bar",
		c: "foobar_private",
	}
	destPtr, err := util.NewFrom(srcPtr)
	require.NoError(t, err)
	assert.IsType(t, srcPtr, destPtr)
	assert.Zero(t, destPtr.(*dummyStruct).A)
	assert.Zero(t, destPtr.(*dummyStruct).B)
	assert.Zero(t, destPtr.(*dummyStruct).c)
}
