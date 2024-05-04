package envoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/go-envoy/test/mock/envoy"
	xds "github.com/cncf/xds/go/xds/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type some struct {
	SomeX someX
}

type someX struct{}

type any interface{}

type dummyConfig struct {
	A string `json:"a"`
	B int    `json:"b" envoy:"mergeable"`
	C string `json:"c" envoy:"mergeable"`
	S *some

	Arrays []string `json:"arrays" envoy:"mergeable,preserve"`

	Any interface{} `json:"any" envoy:"mergeable"`
	any interface{}
}

func TestConfigParser(t *testing.T) {
	mockCC := mock_envoy.NewConfigCallbackHandler(t)
	cp := newConfigParser(dummyConfig{})

	parentValue, err := structpb.NewStruct(map[string]interface{}{
		"a":      "parent value",
		"b":      300,
		"c":      "parent value",
		"arrays": []interface{}{"parent", "value"},
		"any": map[string]interface{}{
			"valueFrom": "parent",
		},
	})
	require.Nil(t, err)

	parentConfigAny, err := anypb.New(&xds.TypedStruct{
		Value: parentValue,
	})
	assert.NoError(t, err)
	assert.NotNil(t, parentConfigAny)

	parentCfg, err := cp.Parse(parentConfigAny, mockCC)
	require.NoError(t, err)

	pConfig, ok := parentCfg.(Configuration)
	assert.True(t, ok)

	pFilterCfg, ok := (pConfig.GetFilterConfig()).(*dummyConfig)
	assert.True(t, ok)
	assert.Equal(t, 300, pFilterCfg.B)
	assert.Equal(t, []string{"parent", "value"}, pFilterCfg.Arrays)

	childValue, err := structpb.NewStruct(map[string]interface{}{
		"a": "child value",
		"b": 500,
		"c": "child value",
		"any": map[string]interface{}{
			"valueFrom": "child",
		},
	})
	require.Nil(t, err)
	childConfigAny, err := anypb.New(&xds.TypedStruct{
		Value: childValue,
	})
	assert.NoError(t, err)
	assert.NotNil(t, childConfigAny)

	childCfg, err := cp.Parse(childConfigAny, mockCC)
	require.Nil(t, err)

	mergedCfg := cp.Merge(parentCfg, childCfg)
	mConfig, ok := mergedCfg.(Configuration)
	assert.True(t, ok)

	pMergedCfg, ok := (mConfig.GetFilterConfig()).(*dummyConfig)
	assert.True(t, ok)
	assert.Equal(t, "parent value", pMergedCfg.A)
	assert.Equal(t, 500, pMergedCfg.B)
	assert.Equal(t, "child value", pMergedCfg.C)
	assert.Equal(t, []string{"parent", "value"}, pMergedCfg.Arrays)
}

func TestConfigParser_mergeStruct(t *testing.T) {
	parent := &dummyConfig{
		A:      "THIS VALUE IN UNCHANGEABLE",
		B:      500,
		C:      "DEFAULT GIVEN FROM PARENT",
		S:      &some{},
		Arrays: []string{"1", "2", "3"},
		Any:    "string",
		any:    "string",
	}
	child := &dummyConfig{
		A:   "UNMERGEABLE; it will be ignored",
		B:   1000,
		C:   "MERGEABLE; value from child",
		Any: "nonstring",
	}

	configParser := &configParser{}

	merged, err := configParser.mergeStruct(parent, child)
	assert.Nil(t, err)

	mergedConfig, ok := merged.(*dummyConfig)
	assert.True(t, ok)
	assert.Equal(t, parent.A, mergedConfig.A)
	assert.Equal(t, child.B, mergedConfig.B)
	assert.Equal(t, child.C, mergedConfig.C)
	assert.Equal(t, parent.S, mergedConfig.S)
	assert.Equal(t, parent.Arrays, mergedConfig.Arrays)
	assert.Equal(t, child.Any, mergedConfig.Any)
	assert.IsType(t, "string", mergedConfig.any)
}
