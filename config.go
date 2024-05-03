package envoy

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	xds "github.com/cncf/xds/go/xds/type/v3"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"google.golang.org/protobuf/types/known/anypb"
)

type Configuration interface {
	GetFilterConfig() interface{}
	GetConfigCallbacks() api.ConfigCallbacks

	// Store allows you to save a value of any type under a key of any type,
	// It designed for sharing data throughout the lifetime of Envoy.
	//
	// Please be cautious! The Store function overwrites any existing data.
	Store(key any, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	// It designed for sharing data throughout the lifetime of Envoy.
	//
	// It returns true if a compatible value is successfully loaded,
	// and false if no value is found or an error occurs during the process.
	Load(key any, receiver interface{}) (ok bool, err error)

	metricCounter(name string) api.CounterMetric
	metricGauge(name string) api.GaugeMetric
	metricHistogram(name string) api.HistogramMetric
}

type config struct {
	filterCfg interface{}
	callbacks api.ConfigCallbacks

	gaugeMetric     map[string]api.GaugeMetric
	counterMetric   map[string]api.CounterMetric
	histogramMetric map[string]api.HistogramMetric

	storage *sync.Map
}

func newConfig(filterCfg interface{}, cc api.ConfigCallbacks) *config {
	return &config{
		filterCfg:       filterCfg,
		callbacks:       cc,
		gaugeMetric:     make(map[string]api.GaugeMetric),
		counterMetric:   make(map[string]api.CounterMetric),
		histogramMetric: make(map[string]api.HistogramMetric),
		storage:         new(sync.Map),
	}
}

func (c *config) GetFilterConfig() interface{} {
	return c.filterCfg
}

func (c *config) GetConfigCallbacks() api.ConfigCallbacks {
	return c.callbacks
}

func (c *config) metricCounter(name string) api.CounterMetric {
	counter, ok := c.counterMetric[name]
	if !ok {
		counter = c.callbacks.DefineCounterMetric(name)
		c.counterMetric[name] = counter
	}

	return counter
}

func (c *config) metricGauge(name string) api.GaugeMetric {
	gauge, ok := c.gaugeMetric[name]
	if !ok {
		gauge = c.callbacks.DefineGaugeMetric(name)
		c.gaugeMetric[name] = gauge
	}

	return gauge
}

func (c *config) metricHistogram(name string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

func (c *config) Store(key any, value any) {
	c.storage.Store(key, value)
}

func (c *config) Load(key any, receiver interface{}) (bool, error) {
	if receiver == nil {
		return false, errors.New("config: receiver should not be nil")
	}

	v, ok := c.storage.Load(key)
	if !ok {
		return false, nil
	}

	if !CastTo(receiver, v) {
		return false, errors.New("config: receiver and value has an incompatible type")
	}

	return true, nil
}

func (c *config) DeepCopyInto(out *config) {
	*out = *c
}

func (c *config) DeepCopy() *config {
	out := new(config)
	c.DeepCopyInto(out)
	return out
}

type configParser struct {
	config interface{}
}

func newConfigParser(config interface{}) *configParser {
	return &configParser{
		config: config,
	}
}

func (p *configParser) Parse(any *anypb.Any, cc api.ConfigCallbackHandler) (interface{}, error) {
	configStruct := &xds.TypedStruct{}
	if err := any.UnmarshalTo(configStruct); err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	v := configStruct.Value
	b, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	var filterCfg reflect.Value
	cfgType := reflect.TypeOf(p.config)
	if cfgType.Kind() == reflect.Ptr {
		filterCfg = reflect.New(cfgType.Elem())
	} else if cfgType.Kind() == reflect.Struct {
		filterCfg = reflect.New(cfgType)
	}

	filterCfgIface := filterCfg.Interface()
	if err := json.Unmarshal(b, &filterCfgIface); err != nil {
		return nil, fmt.Errorf("configparser: parse failed; %w", err)
	}

	type validator interface {
		Validate() error
	}

	if validate, ok := filterCfgIface.(validator); ok {
		validateErr := validate.Validate()
		if validateErr != nil {
			return nil, fmt.Errorf("configparser: parse failed; %w", validateErr)
		}
	}

	cfg := newConfig(filterCfgIface, cc)
	return cfg, nil
}

func (p *configParser) Merge(parent, child interface{}) interface{} {
	parentCfg := parent.(*config)
	childCfg := child.(*config)

	copyParentCfg := parentCfg.DeepCopy()

	mergedFilterCfg, err := p.mergeStruct(copyParentCfg.filterCfg, childCfg.filterCfg)
	if err != nil {
		panic(err)
	}

	copyParentCfg.filterCfg = mergedFilterCfg
	return copyParentCfg
}

func (p *configParser) mergeStruct(parent, child interface{}) (interface{}, error) {
	parentPtr := reflect.ValueOf(parent)
	childPtr := reflect.ValueOf(child)

	switch {
	case parentPtr.Kind() != reflect.Ptr &&
		childPtr.Kind() != reflect.Ptr:
		return nil, errors.New("configparser: merge failed; both parent and child config MUST be a pointer")
	case parentPtr.Type() != childPtr.Type():
		return nil, errors.New("configparser: merge failed; parent and child config has different data type")
	}

	parentValue := parentPtr.Elem()
	childValue := childPtr.Elem()

	if parentValue.Kind() != reflect.Struct &&
		childValue.Kind() != reflect.Struct {
		return nil, errors.New("configparser: merge failed; parent and child config MUST be a struct")
	}

	for i := 0; i < childValue.NumField(); i++ {
		tags, ok := childPtr.Type().Elem().Field(i).Tag.Lookup("envoy")
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

	return parent, nil
}
