package gonvoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	xds "github.com/cncf/xds/go/xds/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type foo struct {
	A string `json:"a"`
	B int    `json:"b"`
}

type any interface{}

type dummyConfig struct {
	A string `json:"a"`
	B int    `json:"b" envoy:"mergeable"`
	C string `json:"c" envoy:"mergeable"`
	S *foo   `json:"s" envoy:"mergeable"`

	Arrays []string `json:"arrays" envoy:"mergeable,preserve"`

	Any interface{} `json:"any" envoy:"mergeable"`
	any interface{}
}

func TestConfigParser(t *testing.T) {
	mockCC := mock_envoy.NewConfigCallbackHandler(t)

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

	childValue, err := structpb.NewStruct(map[string]interface{}{
		"a": "child value",
		"b": 500,
		"c": "child value",
		"any": map[string]interface{}{
			"valueFrom": "child",
		},
		"s": map[string]interface{}{
			"a": "foo",
			"b": 100,
		},
	})
	require.Nil(t, err)
	childConfigAny, err := anypb.New(&xds.TypedStruct{
		Value: childValue,
	})
	assert.NoError(t, err)
	assert.NotNil(t, childConfigAny)

	t.Run("no config", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{})
		parentCfg, err := cp.Parse(nil, mockCC)
		require.NoError(t, err)

		pConfig, ok := parentCfg.(Configuration)
		assert.True(t, ok)
		assert.Nil(t, pConfig.GetFilterConfig())
	})

	t.Run("normal config | Parent only", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{
			BaseConfig: new(dummyConfig),
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)

		pConfig, ok := parentCfg.(Configuration)
		assert.True(t, ok)

		pFilterCfg, ok := (pConfig.GetFilterConfig()).(*dummyConfig)
		assert.True(t, ok)
		assert.Equal(t, 300, pFilterCfg.B)
		assert.Equal(t, []string{"parent", "value"}, pFilterCfg.Arrays)
	})

	t.Run("normal config | Parent and Child", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{
			BaseConfig: new(dummyConfig),
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)
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

		assert.Same(t, parentCfg.(Configuration).Cache(), mConfig.Cache())
		assert.Same(t, parentCfg.(Configuration).Cache(), childCfg.(Configuration).Cache())
		assert.Same(t, childCfg.(Configuration).Cache(), mConfig.Cache())
		assert.NotSame(t, parentCfg.(Configuration).GetFilterConfig(), mConfig.GetFilterConfig())
		assert.NotSame(t, parentCfg.(Configuration).GetFilterConfig(), childCfg.(Configuration).GetFilterConfig())
		assert.NotSame(t, childCfg.(Configuration).GetFilterConfig(), mConfig.GetFilterConfig())

	})
}

func TestConfigParser_mergeStruct(t *testing.T) {
	parent := &dummyConfig{
		A:      "THIS VALUE IN UNCHANGEABLE",
		B:      500,
		C:      "DEFAULT GIVEN FROM PARENT",
		S:      &foo{},
		Arrays: []string{"1", "2", "3"},
		Any:    "string",
		any:    "string",
	}
	child := &dummyConfig{
		A:   "UNMERGEABLE; it will be ignored",
		B:   1000,
		C:   "MERGEABLE; value from child",
		S:   &foo{},
		Any: "nonstring",
		any: "i make it wrong",
	}

	configParser := &configParser{}

	merged, err := configParser.mergeStruct(parent, child)
	assert.NoError(t, err)

	mergedConfig, ok := merged.(*dummyConfig)
	assert.True(t, ok)
	assert.NotEqualValues(t, parent, mergedConfig)
	assert.NotSame(t, parent.S, mergedConfig.S)

	assert.EqualValues(t, parent.A, mergedConfig.A)
	assert.EqualValues(t, child.B, mergedConfig.B)
	assert.EqualValues(t, child.C, mergedConfig.C)
	assert.EqualValues(t, parent.Arrays, mergedConfig.Arrays)
	assert.EqualValues(t, child.Any, mergedConfig.Any)

	child2 := &dummyConfig{
		A:   "UNMERGEABLE; it will be ignored",
		B:   2000,
		C:   "MERGEABLE; value from child2",
		S:   &foo{},
		Any: "nonstring",
		any: "i make it wrong",
	}

	merged2, err := configParser.mergeStruct(parent, child2)
	assert.NoError(t, err)

	mergedConfig2, ok := merged2.(*dummyConfig)
	assert.True(t, ok)
	assert.True(t, ok)
	assert.NotEqualValues(t, mergedConfig, mergedConfig2)
	assert.NotEqualValues(t, parent, mergedConfig2)

	assert.NotSame(t, parent.S, mergedConfig2.S)
	assert.NotSame(t, mergedConfig.S, mergedConfig2.S)
}
