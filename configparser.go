package gonvoy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ardikabs/gonvoy/pkg/util"
	xds "github.com/cncf/xds/go/xds/type/v3"
	"github.com/tidwall/gjson"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"google.golang.org/protobuf/types/known/anypb"
)

// ConfigOptions represents the configuration options for the filters.
type ConfigOptions struct {
	// FilterConfig represents the filter configuration.
	FilterConfig interface{}

	// AlwaysUseChildConfig intend to disable merge behavior, ensuring that it always references the child filter configuration.
	//
	AlwaysUseChildConfig bool

	// IgnoreMergeError specifies during a merge error, instead of panicking, it will fallback to the root configuration.
	//
	IgnoreMergeError bool

	// MetricsPrefix specifies the prefix used for metrics.
	MetricsPrefix string

	// AutoReloadRoute specifies whether the route should be auto reloaded when the request headers changes.
	// It recommends to set this to true when the filter is used in a route configuration and the route is expected to change dynamically within certain conditions.
	AutoReloadRoute bool

	// DisableStrictBodyAccess specifies whether HTTP body access follows strict rules.
	// As its name goes, it defaults to strict, which mean that HTTP body access and manipulation is only possible
	// with the presence of the `X-Content-Operation` header, with accepted values being `ReadOnly` and `ReadWrite`.
	// Even when disabled, users must explicitly enable body access, either for a Request or Response, to carry out operations.
	// See EnableRequestRead, EnableRequestWrite, EnableResponseRead, EnableRequestWrite variables.
	//
	DisableStrictBodyAccess bool

	// EnableRequestBodyRead specifies whether an HTTP Request Body can be accessed.
	// It defaults to false, meaning any operation on OnRequestBody will be ignored.
	// When enabled, operations on OnRequestBody are allowed,
	// but attempts to modify the HTTP Request body will result in a panic, which returns with 502 (Bad Gateway).
	//
	// Warning! Use this option only when necessary, as enabling it is equivalent to granting access
	// to potentially sensitive information that shouldn't be visible to the middleware otherwise.
	EnableRequestBodyRead bool

	// EnableRequestBodyWrite specifies whether an HTTP Request Body can be modified.
	// It defaults to false, meaning any operation on OnRequestBody will be ignored.
	// When enabled, operations on OnRequestBody, including modifications to the HTTP Request body, are permitted.
	//
	// Warning! Use this option only when necessary, as enabling it is equivalent to granting access
	// to potentially sensitive information that shouldn't be visible to the middleware otherwise.
	EnableRequestBodyWrite bool

	// EnableResponseBodyRead specifies whether an HTTP Response Body can be accessed or not.
	// It defaults to false, meaning any operation on OnResponseBody will be ignored.
	// When enabled, operations on OnResponseBody are allowed,
	// but attempts to modify the HTTP Response body will result in a panic, which returns with 502 (Bad Gateway).
	//
	// Warning! Use this option only when necessary, as enabling it is equivalent to granting access
	// to potentially sensitive information that shouldn't be visible to the middleware otherwise.
	EnableResponseBodyRead bool

	// EnableResponseBodyWrite specifies whether an HTTP Response Body can be accessed or not
	// It defaults to false, meaning any operation on OnResponseBody will be ignored.
	// When enabled, operations on OnResponseBody, including modifications to the HTTP Response body, are permitted.
	//
	// Warning! Use this option only when necessary, as enabling it is equivalent to granting access
	// to potentially sensitive information that shouldn't be visible to the middleware otherwise.
	EnableResponseBodyWrite bool

	// DisableChunkedEncodingRequest specifies whether the request body should not be chunked during OnRequestBody phases.
	// This setting applies when EnableRequestBodyWrite is enabled.
	// It defaults to false, meaning if EnableRequestBodyWrite is enabled, the filter is expected to modify the request body, hence it will be chunked following the Content-Length header removal.
	// However, this setting only relevant for protocols prior to HTTP/2, as it will be treated as having chunked encoding (Transfer-Encoding: chunked).
	// Therefore turning this on will preserve the Content-Length header.
	//
	// Note that when this setting is turned on (re: disabled), the request headers will be buffered into the filter manager.
	// Hence, you can modify request headers as well in the OnRequestBody phase.
	DisableChunkedEncodingRequest bool

	// DisableChunkedEncodingResponse specifies whether the response should not be chunked during OnResponseBody phases.
	// This setting applies when EnableResponseBodyWrite is enabled.
	// It defaults to false, meaning if EnableResponseBodyWrite is enabled,
	// the filter is expected to modify the response body, hence it will be chunked following the Content-Length header removal.
	// However, this setting only relevant for protocols prior to HTTP/2, as it will be treated as having chunked encoding (Transfer-Encoding: chunked).
	// Therefore turning this on will preserve the Content-Length header.
	//
	DisableChunkedEncodingResponse bool
}

type configParser struct {
	options          ConfigOptions
	rootGlobalConfig *internalConfig
}

func newConfigParser(options ConfigOptions) *configParser {
	return &configParser{
		options:          options,
		rootGlobalConfig: newInternalConfig(options),
	}
}

func (p *configParser) Parse(any *anypb.Any, callbacks api.ConfigCallbackHandler) (interface{}, error) {
	// Parse the filter configuration if it is provided
	filterConfig, err := p.parseFilterConfig(any)
	if err != nil {
		return nil, err
	}

	// Handle the root (parent) plugin configuration
	if callbacks != nil {
		// Renew the config callbacks and filter config once root plugin configuration updated
		p.rootGlobalConfig.callbacks = callbacks
		p.rootGlobalConfig.filterConfig = filterConfig
		return p.rootGlobalConfig, nil
	}

	// Create a copy of the root global config for the child filter config
	// This shares all attributes except the filter config
	copyGlobalConfig := *p.rootGlobalConfig
	copyGlobalConfig.filterConfig = filterConfig
	return &copyGlobalConfig, nil
}

func (p *configParser) parseFilterConfig(any *anypb.Any) (filterCfg interface{}, err error) {
	if any.GetValue() == nil {
		return nil, nil
	}

	configStruct := &xds.TypedStruct{}
	if err := any.UnmarshalTo(configStruct); err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	v := configStruct.Value
	b, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	if util.IsNil(p.options.FilterConfig) {
		filterCfg = gjson.ParseBytes(b)
	} else {
		filterCfg, err = util.NewFrom(p.options.FilterConfig)
		if err != nil {
			return nil, fmt.Errorf("configparser: parse failed; %w", err)
		}

		if err := json.Unmarshal(b, &filterCfg); err != nil {
			return nil, fmt.Errorf("configparser: parse failed; %w", err)
		}
	}

	return filterCfg, nil
}

func (p *configParser) Merge(parent, child interface{}) interface{} {
	origParentGlobalConfig, parentOK := parent.(*internalConfig)
	origChildGlobalConfig, childOK := child.(*internalConfig)

	if !parentOK && !childOK {
		panic("configparser: merge failed; both parent and child configs uses unknown data types")
	}

	if util.IsNil(p.options.FilterConfig) {
		origChildGlobalConfig.filterConfig = p.mergeLiteral(origParentGlobalConfig.filterConfig, origChildGlobalConfig.filterConfig)
		return origChildGlobalConfig
	}

	mergedFilterCfg, err := p.mergeStruct(origParentGlobalConfig.filterConfig, origChildGlobalConfig.filterConfig)
	if err != nil {
		if p.options.IgnoreMergeError {
			return origParentGlobalConfig
		}

		panic(err)
	}

	origChildGlobalConfig.filterConfig = mergedFilterCfg
	return origChildGlobalConfig
}

func (p *configParser) mergeLiteral(parent, child interface{}) gjson.Result {
	parentJSON, parentJSONValid := parent.(gjson.Result)
	childJSON, childJSONValid := child.(gjson.Result)
	if !parentJSONValid && !childJSONValid {
		return gjson.Result{}
	}

	parentJSONMap, parentMapValid := parentJSON.Value().(map[string]interface{})
	childJSONMap, childMapValid := childJSON.Value().(map[string]interface{})
	if !parentMapValid && !childMapValid {
		return gjson.Result{}
	}

	for k, v := range parentJSONMap {
		if _, ok := childJSONMap[k]; !ok {
			childJSONMap[k] = v
		}
	}

	jsonBytes, err := json.Marshal(childJSONMap)
	if err != nil {
		return gjson.Result{}
	}

	return gjson.ParseBytes(jsonBytes)
}

func (p *configParser) mergeStruct(parent, child interface{}) (interface{}, error) {
	origParentPtr := reflect.ValueOf(parent)
	origParentValue := origParentPtr.Elem()

	parentType := origParentPtr.Type().Elem()

	parentPtr := reflect.New(parentType) // *FilterConfigDataType
	childPtr := reflect.ValueOf(child)   // *FilterConfigDataType

	if parentPtr.Kind() != reflect.Pointer || childPtr.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("configparser: merge failed; both parent(%s) and child(%s) configs MUST be pointers", parentPtr.Type(), childPtr.Type())
	}

	parentValue := parentPtr.Elem() // FilterConfigDataType
	childValue := childPtr.Elem()   // FilterConfigDataType

	switch {
	case parentValue.Type() != childValue.Type():
		return nil, fmt.Errorf("configparser: merge failed; parent(%s) and child(%s) configs have different data types", parentValue.Type(), childValue.Type())
	case parentValue.Kind() != reflect.Struct || childValue.Kind() != reflect.Struct:
		return nil, fmt.Errorf("configparser: merge failed; both parent(%s) and child(%s) configs MUST be references to a struct", parentValue.Kind(), childValue.Kind())
	}

	if p.options.AlwaysUseChildConfig {
		return child, nil
	}

	parentValue.Set(origParentValue)

	for i := 0; i < childValue.NumField(); i++ {
		tags, ok := parentType.Field(i).Tag.Lookup("envoy")
		if !ok {
			continue
		}

		v := childValue.Field(i)

		isValidField := v.IsValid() || v.CanSet()
		isMergeable := strings.Contains(tags, "mergeable")
		isPreserveable := strings.Contains(tags, "preserve_root") && v.IsZero()
		if !isValidField ||
			!isMergeable ||
			isPreserveable {
			continue
		}

		parentValue.Field(i).Set(v)
	}

	return parentPtr.Interface(), nil
}
