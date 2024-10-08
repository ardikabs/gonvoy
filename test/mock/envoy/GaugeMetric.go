// Code generated by mockery v2.46.1. DO NOT EDIT.

package mock_envoy

import mock "github.com/stretchr/testify/mock"

// GaugeMetric is an autogenerated mock type for the GaugeMetric type
type GaugeMetric struct {
	mock.Mock
}

type GaugeMetric_Expecter struct {
	mock *mock.Mock
}

func (_m *GaugeMetric) EXPECT() *GaugeMetric_Expecter {
	return &GaugeMetric_Expecter{mock: &_m.Mock}
}

// Get provides a mock function with given fields:
func (_m *GaugeMetric) Get() uint64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GaugeMetric_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type GaugeMetric_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
func (_e *GaugeMetric_Expecter) Get() *GaugeMetric_Get_Call {
	return &GaugeMetric_Get_Call{Call: _e.mock.On("Get")}
}

func (_c *GaugeMetric_Get_Call) Run(run func()) *GaugeMetric_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *GaugeMetric_Get_Call) Return(_a0 uint64) *GaugeMetric_Get_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *GaugeMetric_Get_Call) RunAndReturn(run func() uint64) *GaugeMetric_Get_Call {
	_c.Call.Return(run)
	return _c
}

// Increment provides a mock function with given fields: offset
func (_m *GaugeMetric) Increment(offset int64) {
	_m.Called(offset)
}

// GaugeMetric_Increment_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Increment'
type GaugeMetric_Increment_Call struct {
	*mock.Call
}

// Increment is a helper method to define mock.On call
//   - offset int64
func (_e *GaugeMetric_Expecter) Increment(offset interface{}) *GaugeMetric_Increment_Call {
	return &GaugeMetric_Increment_Call{Call: _e.mock.On("Increment", offset)}
}

func (_c *GaugeMetric_Increment_Call) Run(run func(offset int64)) *GaugeMetric_Increment_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(int64))
	})
	return _c
}

func (_c *GaugeMetric_Increment_Call) Return() *GaugeMetric_Increment_Call {
	_c.Call.Return()
	return _c
}

func (_c *GaugeMetric_Increment_Call) RunAndReturn(run func(int64)) *GaugeMetric_Increment_Call {
	_c.Call.Return(run)
	return _c
}

// Record provides a mock function with given fields: value
func (_m *GaugeMetric) Record(value uint64) {
	_m.Called(value)
}

// GaugeMetric_Record_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Record'
type GaugeMetric_Record_Call struct {
	*mock.Call
}

// Record is a helper method to define mock.On call
//   - value uint64
func (_e *GaugeMetric_Expecter) Record(value interface{}) *GaugeMetric_Record_Call {
	return &GaugeMetric_Record_Call{Call: _e.mock.On("Record", value)}
}

func (_c *GaugeMetric_Record_Call) Run(run func(value uint64)) *GaugeMetric_Record_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(uint64))
	})
	return _c
}

func (_c *GaugeMetric_Record_Call) Return() *GaugeMetric_Record_Call {
	_c.Call.Return()
	return _c
}

func (_c *GaugeMetric_Record_Call) RunAndReturn(run func(uint64)) *GaugeMetric_Record_Call {
	_c.Call.Return(run)
	return _c
}

// NewGaugeMetric creates a new instance of GaugeMetric. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewGaugeMetric(t interface {
	mock.TestingT
	Cleanup(func())
}) *GaugeMetric {
	mock := &GaugeMetric{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
