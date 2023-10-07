package main

import (
	"encoding/json"

	xds "github.com/cncf/xds/go/xds/type/v3"

	"google.golang.org/protobuf/types/known/anypb"
)

type config struct {
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
}

type configParser struct{}

func (p *configParser) Parse(any *anypb.Any) (interface{}, error) {
	configStruct := &xds.TypedStruct{}
	if err := any.UnmarshalTo(configStruct); err != nil {
		return nil, err
	}

	v := configStruct.Value
	conf := &config{}

	data, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

func (p *configParser) Merge(parent interface{}, child interface{}) interface{} {
	parentConfig := parent.(*config)
	childConfig := child.(*config)

	mergedConfig := *parentConfig
	mergedConfig.RequestHeaders = childConfig.RequestHeaders

	return &mergedConfig
}
