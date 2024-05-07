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

type ConfigOptions struct {
	BaseConfig interface{}

	// AlwaysReplaceRootConfig specifies that on merge, it will replace with the child filter configuration, regardless
	// with/out `mergeable` tag.
	AlwaysReplaceRootConfig bool

	// IgnoreMergeError specifies during a merge error, instead of panicking, it will fallback to the root configuration.
	IgnoreMergeError bool

	// EnabledHttpFilterPhases lists the HttpFilterPhase options enabled for the filter.
	// If both EnabledHttpFilterPhases and DisabledHttpFilterPhases are present,
	// only EnabledHttpFilterPhases is used, as they are mutually exclusive.
	EnabledHttpFilterPhases []HttpFilterPhase

	// DisabledHttpFilterPhases lists the HttpFilterPhase options disabled for the filter.
	// If both EnabledHttpFilterPhases and DisabledHttpFilterPhases are present,
	// only EnabledHttpFilterPhases is used, as they are mutually exclusive.
	DisabledHttpFilterPhases []HttpFilterPhase
}

type configParser struct {
	options ConfigOptions

	globalConfig *globalConfig
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

	if p.globalConfig == nil {
		p.globalConfig = newGlobalConfig(cc, p.options)
		p.globalConfig.filterCfg = filterCfg
		return p.globalConfig, nil
	}

	copyGlobalConfig := *p.globalConfig
	copyGlobalConfig.filterCfg = filterCfg
	return &copyGlobalConfig, nil
}

func (p *configParser) Merge(parent, child interface{}) interface{} {
	origParentGlobalConfig := parent.(*globalConfig)
	origChildGlobalConfig := child.(*globalConfig)

	if util.IsNil(origParentGlobalConfig.filterCfg) {
		return parent
	}

	mergedGlobalConfig := *origParentGlobalConfig
	mergedFilterCfg, err := p.mergeStruct(mergedGlobalConfig.filterCfg, origChildGlobalConfig.filterCfg)
	if err != nil {
		if p.options.IgnoreMergeError {
			panic(err)
		}

	}

	mergedGlobalConfig.filterCfg = mergedFilterCfg
	return &mergedGlobalConfig
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

	if p.options.AlwaysReplaceRootConfig {
		parentValue.Set(childPtr.Elem())
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
