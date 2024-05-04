package envoy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ardikabs/go-envoy/pkg/util"
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
	parentValue := reflect.ValueOf(parent)
	childValue := reflect.ValueOf(child)

	var structTags reflect.Type

	switch {
	case parentValue.Type() != childValue.Type():
		return nil, fmt.Errorf("configparser: merge failed; parent(%s) and child(%s) config has different data type", parentValue.Type(), childValue.Type())

	case parentValue.Kind() == reflect.Struct:
		structTags = parentValue.Type()

	case parentValue.Kind() == reflect.Ptr:
		structTags = parentValue.Type().Elem()

		parentValue = parentValue.Elem()
		childValue = childValue.Elem()
	}

	if parentValue.Kind() != reflect.Struct || childValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("configparser: merge failed; parent(%s) and child(%s) config MUST be a struct", parentValue.Kind(), childValue.Kind())
	}

	for i := 0; i < childValue.NumField(); i++ {
		tags, ok := structTags.Field(i).Tag.Lookup("envoy")
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

	copyParent := reflect.New(parentValue.Type())
	copyParentValue := copyParent.Elem()
	copyParentValue.Set(parentValue)
	return copyParent.Interface(), nil
}
