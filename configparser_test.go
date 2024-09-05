package gaetway

import (
	"testing"

	mock_envoy "github.com/ardikabs/gaetway/test/mock/envoy"
	xds "github.com/cncf/xds/go/xds/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type foo struct {
	A string `json:"a"`
	B int    `json:"b"`
}

type dummyConfig struct {
	A string `json:"a"`
	B int    `json:"b" envoy:"mergeable"`
	C string `json:"c" envoy:"mergeable"`
	S *foo   `json:"s" envoy:"mergeable"`

	Arrays []string `json:"arrays" envoy:"mergeable,preserve_root"`

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

	t.Run("empty config", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{})
		parentCfg, err := cp.Parse(&anypb.Any{}, mockCC)
		require.NoError(t, err)

		pConfig, ok := parentCfg.(*internalConfig)
		assert.True(t, ok)
		assert.Nil(t, pConfig.filterConfig)
	})

	t.Run("filter config unset, fallback to map | Parent Only", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{})
		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)

		pConfig, ok := parentCfg.(*internalConfig)
		assert.True(t, ok)
		assert.NotEmpty(t, pConfig.filterConfig)
		assert.IsType(t, gjson.Result{}, pConfig.filterConfig)

		filterCfg, ok := pConfig.filterConfig.(gjson.Result)
		assert.True(t, ok)

		assert.Equal(t, "parent value", filterCfg.Get("a").Str)
		assert.Equal(t, int64(300), filterCfg.Get("b").Int())
		assert.Equal(t, "parent value", filterCfg.Get("c").Str)
	})

	t.Run("filter config unset, fallback to map | Parent and Child", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{})
		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)

		childCfg, err := cp.Parse(childConfigAny, nil)
		require.Nil(t, err)

		mergedCfg := cp.Merge(parentCfg, childCfg)
		mergedInternalConfig, ok := mergedCfg.(*internalConfig)
		assert.True(t, ok)

		filterCfg, ok := mergedInternalConfig.filterConfig.(gjson.Result)
		assert.True(t, ok)

		assert.EqualValues(t, "child value", filterCfg.Get("a").Str)
		assert.EqualValues(t, int64(500), filterCfg.Get("b").Int())
		assert.EqualValues(t, "child value", filterCfg.Get("c").Str)
		assert.EqualValues(t, map[string]interface{}{
			"a": "foo",
			"b": float64(100),
		}, filterCfg.Get("s").Value())
	})

	t.Run("with filter config | Parent only", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{
			FilterConfig: new(dummyConfig),
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)

		pConfig, ok := parentCfg.(*internalConfig)
		assert.True(t, ok)

		pFilterCfg, ok := (pConfig.filterConfig).(*dummyConfig)
		assert.True(t, ok)
		assert.Equal(t, 300, pFilterCfg.B)
		assert.Equal(t, []string{"parent", "value"}, pFilterCfg.Arrays)
	})

	t.Run("with filter config | Parent and Child", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{
			FilterConfig: new(dummyConfig),
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)
		childCfg, err := cp.Parse(childConfigAny, nil)
		require.Nil(t, err)

		mergedCfg := cp.Merge(parentCfg, childCfg)
		mergedInternalConfig, ok := mergedCfg.(*internalConfig)
		assert.True(t, ok)

		pMergedCfg, ok := (mergedInternalConfig.filterConfig).(*dummyConfig)
		assert.True(t, ok)
		assert.Equal(t, "parent value", pMergedCfg.A)
		assert.Equal(t, 500, pMergedCfg.B)
		assert.Equal(t, "child value", pMergedCfg.C)
		assert.Equal(t, []string{"parent", "value"}, pMergedCfg.Arrays)

		assert.Same(t, parentCfg.(*internalConfig).internalCache, mergedInternalConfig.internalCache)
		assert.Same(t, mergedInternalConfig.internalCache, childCfg.(*internalConfig).internalCache)
		assert.Same(t, parentCfg.(*internalConfig).callbacks, mergedInternalConfig.callbacks)
		assert.Same(t, mergedInternalConfig.callbacks, childCfg.(*internalConfig).callbacks)
		assert.NotSame(t, parentCfg.(*internalConfig).filterConfig, mergedInternalConfig.filterConfig)
		assert.Same(t, mergedInternalConfig.filterConfig, childCfg.(*internalConfig).filterConfig)
	})

	t.Run("with filter config | Always use Child config", func(t *testing.T) {
		cp := newConfigParser(ConfigOptions{
			FilterConfig:         new(dummyConfig),
			AlwaysUseChildConfig: true,
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)
		childCfg, err := cp.Parse(childConfigAny, nil)
		require.Nil(t, err)

		mergedCfg := cp.Merge(parentCfg, childCfg)
		mergedInternalConfig, ok := mergedCfg.(*internalConfig)
		assert.True(t, ok)

		pMergedCfg, ok := (mergedInternalConfig.filterConfig).(*dummyConfig)
		assert.True(t, ok)
		assert.Equal(t, "child value", pMergedCfg.A)
		assert.Equal(t, 500, pMergedCfg.B)
		assert.Equal(t, "child value", pMergedCfg.C)
		assert.Empty(t, pMergedCfg.Arrays)

		assert.Same(t, parentCfg.(*internalConfig).internalCache, mergedInternalConfig.internalCache)
		assert.Same(t, mergedInternalConfig.internalCache, childCfg.(*internalConfig).internalCache)
		assert.Same(t, parentCfg.(*internalConfig).callbacks, mergedInternalConfig.callbacks)
		assert.Same(t, mergedInternalConfig.callbacks, childCfg.(*internalConfig).callbacks)
		assert.NotSame(t, parentCfg.(*internalConfig).filterConfig, mergedInternalConfig.filterConfig)
		assert.NotSame(t, parentCfg.(*internalConfig).filterConfig, childCfg.(*internalConfig).filterConfig)
		assert.NotSame(t, parentCfg, childCfg)
		assert.NotSame(t, parentCfg, mergedInternalConfig)
		assert.Same(t, mergedInternalConfig.filterConfig, childCfg.(*internalConfig).filterConfig)
		assert.Same(t, childCfg, mergedInternalConfig)

		pParentCfg := (parentCfg.(*internalConfig).filterConfig).(*dummyConfig)
		pChildCfg := (childCfg.(*internalConfig).filterConfig).(*dummyConfig)
		assert.NotSame(t, pParentCfg.S, pChildCfg.S)
		assert.NotSame(t, pParentCfg.S, pMergedCfg.S)
		assert.Same(t, pChildCfg.S, pMergedCfg.S)
	})

	t.Run("with filter config, mergeable without preserve_root | Parent and Child", func(t *testing.T) {
		otherChildValue, err := structpb.NewStruct(map[string]interface{}{
			"a": "child value",
			"b": 500,
		})

		require.Nil(t, err)
		otherChildConfigAny, err := anypb.New(&xds.TypedStruct{
			Value: otherChildValue,
		})
		assert.NoError(t, err)
		assert.NotNil(t, otherChildConfigAny)

		cp := newConfigParser(ConfigOptions{
			FilterConfig: new(dummyConfig),
		})

		parentCfg, err := cp.Parse(parentConfigAny, mockCC)
		require.NoError(t, err)
		childCfg, err := cp.Parse(otherChildConfigAny, nil)
		require.Nil(t, err)

		mergedCfg := cp.Merge(parentCfg, childCfg)
		mergedInternalConfig, ok := mergedCfg.(*internalConfig)
		assert.True(t, ok)

		pMergedCfg, ok := (mergedInternalConfig.filterConfig).(*dummyConfig)
		assert.True(t, ok)
		assert.Equal(t, "parent value", pMergedCfg.A)
		assert.Equal(t, 500, pMergedCfg.B)
		assert.Equal(t, "", pMergedCfg.C)
		assert.Nil(t, pMergedCfg.S)
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
