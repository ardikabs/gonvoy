// Code generated by mockery v2.46.1. DO NOT EDIT.

package mock_envoy

import (
	api "github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	mock "github.com/stretchr/testify/mock"
)

// FilterCallbackHandler is an autogenerated mock type for the FilterCallbackHandler type
type FilterCallbackHandler struct {
	mock.Mock
}

type FilterCallbackHandler_Expecter struct {
	mock *mock.Mock
}

func (_m *FilterCallbackHandler) EXPECT() *FilterCallbackHandler_Expecter {
	return &FilterCallbackHandler_Expecter{mock: &_m.Mock}
}

// ClearRouteCache provides a mock function with given fields:
func (_m *FilterCallbackHandler) ClearRouteCache() {
	_m.Called()
}

// FilterCallbackHandler_ClearRouteCache_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ClearRouteCache'
type FilterCallbackHandler_ClearRouteCache_Call struct {
	*mock.Call
}

// ClearRouteCache is a helper method to define mock.On call
func (_e *FilterCallbackHandler_Expecter) ClearRouteCache() *FilterCallbackHandler_ClearRouteCache_Call {
	return &FilterCallbackHandler_ClearRouteCache_Call{Call: _e.mock.On("ClearRouteCache")}
}

func (_c *FilterCallbackHandler_ClearRouteCache_Call) Run(run func()) *FilterCallbackHandler_ClearRouteCache_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *FilterCallbackHandler_ClearRouteCache_Call) Return() *FilterCallbackHandler_ClearRouteCache_Call {
	_c.Call.Return()
	return _c
}

func (_c *FilterCallbackHandler_ClearRouteCache_Call) RunAndReturn(run func()) *FilterCallbackHandler_ClearRouteCache_Call {
	_c.Call.Return(run)
	return _c
}

// DecoderFilterCallbacks provides a mock function with given fields:
func (_m *FilterCallbackHandler) DecoderFilterCallbacks() api.DecoderFilterCallbacks {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for DecoderFilterCallbacks")
	}

	var r0 api.DecoderFilterCallbacks
	if rf, ok := ret.Get(0).(func() api.DecoderFilterCallbacks); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(api.DecoderFilterCallbacks)
		}
	}

	return r0
}

// FilterCallbackHandler_DecoderFilterCallbacks_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DecoderFilterCallbacks'
type FilterCallbackHandler_DecoderFilterCallbacks_Call struct {
	*mock.Call
}

// DecoderFilterCallbacks is a helper method to define mock.On call
func (_e *FilterCallbackHandler_Expecter) DecoderFilterCallbacks() *FilterCallbackHandler_DecoderFilterCallbacks_Call {
	return &FilterCallbackHandler_DecoderFilterCallbacks_Call{Call: _e.mock.On("DecoderFilterCallbacks")}
}

func (_c *FilterCallbackHandler_DecoderFilterCallbacks_Call) Run(run func()) *FilterCallbackHandler_DecoderFilterCallbacks_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *FilterCallbackHandler_DecoderFilterCallbacks_Call) Return(_a0 api.DecoderFilterCallbacks) *FilterCallbackHandler_DecoderFilterCallbacks_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *FilterCallbackHandler_DecoderFilterCallbacks_Call) RunAndReturn(run func() api.DecoderFilterCallbacks) *FilterCallbackHandler_DecoderFilterCallbacks_Call {
	_c.Call.Return(run)
	return _c
}

// EncoderFilterCallbacks provides a mock function with given fields:
func (_m *FilterCallbackHandler) EncoderFilterCallbacks() api.EncoderFilterCallbacks {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for EncoderFilterCallbacks")
	}

	var r0 api.EncoderFilterCallbacks
	if rf, ok := ret.Get(0).(func() api.EncoderFilterCallbacks); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(api.EncoderFilterCallbacks)
		}
	}

	return r0
}

// FilterCallbackHandler_EncoderFilterCallbacks_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'EncoderFilterCallbacks'
type FilterCallbackHandler_EncoderFilterCallbacks_Call struct {
	*mock.Call
}

// EncoderFilterCallbacks is a helper method to define mock.On call
func (_e *FilterCallbackHandler_Expecter) EncoderFilterCallbacks() *FilterCallbackHandler_EncoderFilterCallbacks_Call {
	return &FilterCallbackHandler_EncoderFilterCallbacks_Call{Call: _e.mock.On("EncoderFilterCallbacks")}
}

func (_c *FilterCallbackHandler_EncoderFilterCallbacks_Call) Run(run func()) *FilterCallbackHandler_EncoderFilterCallbacks_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *FilterCallbackHandler_EncoderFilterCallbacks_Call) Return(_a0 api.EncoderFilterCallbacks) *FilterCallbackHandler_EncoderFilterCallbacks_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *FilterCallbackHandler_EncoderFilterCallbacks_Call) RunAndReturn(run func() api.EncoderFilterCallbacks) *FilterCallbackHandler_EncoderFilterCallbacks_Call {
	_c.Call.Return(run)
	return _c
}

// GetProperty provides a mock function with given fields: key
func (_m *FilterCallbackHandler) GetProperty(key string) (string, error) {
	ret := _m.Called(key)

	if len(ret) == 0 {
		panic("no return value specified for GetProperty")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (string, error)); ok {
		return rf(key)
	}
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(key)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(key)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FilterCallbackHandler_GetProperty_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetProperty'
type FilterCallbackHandler_GetProperty_Call struct {
	*mock.Call
}

// GetProperty is a helper method to define mock.On call
//   - key string
func (_e *FilterCallbackHandler_Expecter) GetProperty(key interface{}) *FilterCallbackHandler_GetProperty_Call {
	return &FilterCallbackHandler_GetProperty_Call{Call: _e.mock.On("GetProperty", key)}
}

func (_c *FilterCallbackHandler_GetProperty_Call) Run(run func(key string)) *FilterCallbackHandler_GetProperty_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *FilterCallbackHandler_GetProperty_Call) Return(_a0 string, _a1 error) *FilterCallbackHandler_GetProperty_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *FilterCallbackHandler_GetProperty_Call) RunAndReturn(run func(string) (string, error)) *FilterCallbackHandler_GetProperty_Call {
	_c.Call.Return(run)
	return _c
}

// Log provides a mock function with given fields: level, msg
func (_m *FilterCallbackHandler) Log(level api.LogType, msg string) {
	_m.Called(level, msg)
}

// FilterCallbackHandler_Log_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Log'
type FilterCallbackHandler_Log_Call struct {
	*mock.Call
}

// Log is a helper method to define mock.On call
//   - level api.LogType
//   - msg string
func (_e *FilterCallbackHandler_Expecter) Log(level interface{}, msg interface{}) *FilterCallbackHandler_Log_Call {
	return &FilterCallbackHandler_Log_Call{Call: _e.mock.On("Log", level, msg)}
}

func (_c *FilterCallbackHandler_Log_Call) Run(run func(level api.LogType, msg string)) *FilterCallbackHandler_Log_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(api.LogType), args[1].(string))
	})
	return _c
}

func (_c *FilterCallbackHandler_Log_Call) Return() *FilterCallbackHandler_Log_Call {
	_c.Call.Return()
	return _c
}

func (_c *FilterCallbackHandler_Log_Call) RunAndReturn(run func(api.LogType, string)) *FilterCallbackHandler_Log_Call {
	_c.Call.Return(run)
	return _c
}

// LogLevel provides a mock function with given fields:
func (_m *FilterCallbackHandler) LogLevel() api.LogType {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for LogLevel")
	}

	var r0 api.LogType
	if rf, ok := ret.Get(0).(func() api.LogType); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(api.LogType)
	}

	return r0
}

// FilterCallbackHandler_LogLevel_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'LogLevel'
type FilterCallbackHandler_LogLevel_Call struct {
	*mock.Call
}

// LogLevel is a helper method to define mock.On call
func (_e *FilterCallbackHandler_Expecter) LogLevel() *FilterCallbackHandler_LogLevel_Call {
	return &FilterCallbackHandler_LogLevel_Call{Call: _e.mock.On("LogLevel")}
}

func (_c *FilterCallbackHandler_LogLevel_Call) Run(run func()) *FilterCallbackHandler_LogLevel_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *FilterCallbackHandler_LogLevel_Call) Return(_a0 api.LogType) *FilterCallbackHandler_LogLevel_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *FilterCallbackHandler_LogLevel_Call) RunAndReturn(run func() api.LogType) *FilterCallbackHandler_LogLevel_Call {
	_c.Call.Return(run)
	return _c
}

// StreamInfo provides a mock function with given fields:
func (_m *FilterCallbackHandler) StreamInfo() api.StreamInfo {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for StreamInfo")
	}

	var r0 api.StreamInfo
	if rf, ok := ret.Get(0).(func() api.StreamInfo); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(api.StreamInfo)
		}
	}

	return r0
}

// FilterCallbackHandler_StreamInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StreamInfo'
type FilterCallbackHandler_StreamInfo_Call struct {
	*mock.Call
}

// StreamInfo is a helper method to define mock.On call
func (_e *FilterCallbackHandler_Expecter) StreamInfo() *FilterCallbackHandler_StreamInfo_Call {
	return &FilterCallbackHandler_StreamInfo_Call{Call: _e.mock.On("StreamInfo")}
}

func (_c *FilterCallbackHandler_StreamInfo_Call) Run(run func()) *FilterCallbackHandler_StreamInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *FilterCallbackHandler_StreamInfo_Call) Return(_a0 api.StreamInfo) *FilterCallbackHandler_StreamInfo_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *FilterCallbackHandler_StreamInfo_Call) RunAndReturn(run func() api.StreamInfo) *FilterCallbackHandler_StreamInfo_Call {
	_c.Call.Return(run)
	return _c
}

// NewFilterCallbackHandler creates a new instance of FilterCallbackHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewFilterCallbackHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *FilterCallbackHandler {
	mock := &FilterCallbackHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
