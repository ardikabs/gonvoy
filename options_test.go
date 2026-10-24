package gonvoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFilterConfig struct {
	Allow bool
}

func TestOptionsFromConfigOptions(t *testing.T) {
	t.Run("field mapping preserves semantics", func(t *testing.T) {
		testcases := []struct {
			name     string
			legacy   ConfigOptions
			expected Options
		}{
			{
				name:     "zero value",
				legacy:   ConfigOptions{},
				expected: Options{},
			},
			{
				name: "body access open",
				legacy: ConfigOptions{
					DisableStrictBodyAccess: true,
					EnableRequestBodyRead:   true,
					EnableRequestBodyWrite:  true,
				},
				expected: Options{
					Body: BodyPolicy{
						Access:  BodyAccessOpen,
						Request: BodyRuleReadWrite,
					},
				},
			},
			{
				name: "response read with content length kept",
				legacy: ConfigOptions{
					EnableResponseBodyRead:         true,
					DisableChunkedEncodingRequest:  true,
					DisableChunkedEncodingResponse: true,
				},
				expected: Options{
					Body: BodyPolicy{
						Access:            BodyAccessHeaderGated,
						Response:          BodyRuleRead,
						KeepContentLength: KeepLengthBoth,
					},
				},
			},
			{
				name: "route and merge",
				legacy: ConfigOptions{
					AutoReloadRoute:      true,
					AlwaysUseChildConfig: true,
					IgnoreMergeError:     true,
				},
				expected: Options{
					Route: RoutePolicy{ReloadOnHeaderChange: true},
					Merge: MergePolicy{ChildWins: true, OnError: MergeOnErrorFallbackToParent},
				},
			},
			{
				name: "config and metrics pass through",
				legacy: ConfigOptions{
					FilterConfig:  &fakeFilterConfig{Allow: true},
					MetricsPrefix: "myfilter_",
				},
				expected: Options{
					Config:        &fakeFilterConfig{Allow: true},
					MetricsPrefix: "myfilter_",
				},
			},
		}

		for _, tc := range testcases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tc.expected, OptionsFromConfigOptions(tc.legacy))
			})
		}
	})
}

type fakeTagConfig struct {
	Kept    string `json:"kept,omitempty" envoy:"mergeable,preserve_root"`
	Merged  string `json:"merged,omitempty" envoy:"mergeable"`
	Ignored string `json:"ignored,omitempty"`
}

func TestConfigParser_StructTags(t *testing.T) {
	t.Parallel()

	p := &configParser{}
	parent := &fakeTagConfig{Kept: "parent", Merged: "parent", Ignored: "parent"}
	child := &fakeTagConfig{Merged: "child", Ignored: "child"}

	merged, err := p.mergeStruct(parent, child)
	require.NoError(t, err)

	got, ok := merged.(*fakeTagConfig)
	require.True(t, ok, "expected *fakeTagConfig, got %T", merged)
	assert.Equal(t, "parent", got.Kept)
	assert.Equal(t, "child", got.Merged)
	assert.Equal(t, "parent", got.Ignored)
}

func TestConfigOptionsFromOptions_RoundTrip(t *testing.T) {
	t.Run("options survive a round trip", func(t *testing.T) {
		testcases := []ConfigOptions{
			{},
			{
				FilterConfig:                   &fakeFilterConfig{Allow: true},
				MetricsPrefix:                  "myfilter_",
				AutoReloadRoute:                true,
				AlwaysUseChildConfig:           true,
				IgnoreMergeError:               true,
				DisableStrictBodyAccess:        true,
				EnableRequestBodyRead:          true,
				EnableRequestBodyWrite:         true,
				EnableResponseBodyRead:         true,
				EnableResponseBodyWrite:        true,
				DisableChunkedEncodingRequest:  true,
				DisableChunkedEncodingResponse: true,
			},
			{
				EnableRequestBodyRead:          true,
				EnableResponseBodyRead:         true,
				EnableResponseBodyWrite:        true,
				DisableChunkedEncodingResponse: true,
			},
		}

		for _, tc := range testcases {
			tc := tc
			t.Run("round trip", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tc, ConfigOptionsFromOptions(OptionsFromConfigOptions(tc)))
			})
		}
	})
}

func TestOptionsFromConfigOptions_NormalizesWriteOnly(t *testing.T) {
	t.Parallel()

	// Write-only legacy state behaves as read-write in the runtime
	// (read is access && (allowRead || allowWrite)), so it normalizes.
	got := OptionsFromConfigOptions(ConfigOptions{EnableResponseBodyWrite: true})

	assert.Equal(t, BodyRuleReadWrite, got.Body.Response)
	assert.Equal(t, ConfigOptions{
		EnableRequestBodyRead:   false,
		EnableRequestBodyWrite:  false,
		EnableResponseBodyRead:  true,
		EnableResponseBodyWrite: true,
	}, ConfigOptionsFromOptions(got))
}
