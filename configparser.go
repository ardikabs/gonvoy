package gonvoy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ardikabs/gonvoy/pkg/util"
	xds "github.com/cncf/xds/go/xds/type/v3"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"google.golang.org/protobuf/types/known/anypb"
)

// ConfigOptions represents the configuration options for the filters.
type ConfigOptions struct {
	BaseConfig interface{}

	// AlwaysUseChildConfig intend to disable merge behavior, ensuring that it always references the child filter configuration.
	//
	AlwaysUseChildConfig bool

	// IgnoreMergeError specifies during a merge error, instead of panicking, it will fallback to the root configuration.
	//
	IgnoreMergeError bool

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

	// MetricPrefix specifies the prefix used for metrics.
	MetricPrefix string
}

type configParser struct {
	options ConfigOptions

	rootGlobalConfig *globalConfig
}

func newConfigParser(options ConfigOptions) *configParser {
	return &configParser{
		options: options,
	}
}

func (p *configParser) Parse(any *anypb.Any, cc api.ConfigCallbackHandler) (interface{}, error) {
	if util.IsNil(p.options.BaseConfig) {
		return newGlobalConfig(cc, p.options), nil
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

	filterCfg, err := util.NewFrom(p.options.BaseConfig)
	if err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	if err := json.Unmarshal(b, &filterCfg); err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	if p.rootGlobalConfig == nil {
		p.rootGlobalConfig = newGlobalConfig(cc, p.options)
		p.rootGlobalConfig.filterConfig = filterCfg
		return p.rootGlobalConfig, nil
	}

	copyGlobalConfig := *p.rootGlobalConfig
	copyGlobalConfig.filterConfig = filterCfg
	return &copyGlobalConfig, nil
}

func (p *configParser) Merge(parent, child interface{}) interface{} {
	origParentGlobalConfig := parent.(*globalConfig)
	origChildGlobalConfig := child.(*globalConfig)

	if util.IsNil(origParentGlobalConfig.filterConfig) {
		return parent
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
		isPreserveable := strings.Contains(tags, "preserve") && v.IsZero()
		if !isValidField ||
			!isMergeable ||
			isPreserveable {
			continue
		}

		parentValue.Field(i).Set(v)
	}

	return parentPtr.Interface(), nil
}
