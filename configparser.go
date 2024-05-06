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

type configParser struct {
	filterConfig interface{}
}

func newConfigParser(filterConfig interface{}) *configParser {
	return &configParser{filterConfig}
}

func (p *configParser) Parse(any *anypb.Any, cc api.ConfigCallbackHandler) (interface{}, error) {
	if p.filterConfig == nil {
		return newConfig(nil, cc), nil
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

	filterCfg, err := util.NewFrom(p.filterConfig)
	if err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	if err := json.Unmarshal(b, &filterCfg); err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	cfg := newConfig(filterCfg, cc)
	return cfg, nil
}

func (p *configParser) Merge(parent, child interface{}) interface{} {
	origParentCfg := parent.(*config)
	if origParentCfg.filterCfg == nil {
		return parent
	}

	parentCfg := *origParentCfg
	childCfg := child.(*config)
	mergedFilterCfg, err := p.mergeStruct(parentCfg.filterCfg, childCfg.filterCfg)
	if err != nil {
		panic(err)
	}

	parentCfg.filterCfg = mergedFilterCfg
	return &parentCfg
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

	parentValue.Set(origParentValue)

	switch {
	case parentValue.Type() != childValue.Type():
		return nil, fmt.Errorf("configparser: merge failed; parent(%s) and child(%s) configs have different data types", parentValue.Type(), childValue.Type())
	case parentValue.Kind() != reflect.Struct || childValue.Kind() != reflect.Struct:
		return nil, fmt.Errorf("configparser: merge failed; both parent(%s) and child(%s) configs MUST be references to a struct", parentValue.Kind(), childValue.Kind())
	}

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
