// Code generated by MockGen. DO NOT EDIT.
// Source: engine.go
//
// Generated by this command:
//
//	mockgen -package chasm -source engine.go -destination engine_mock.go
//

// Package chasm is a generated GoMock package.
package chasm

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockEngine is a mock of Engine interface.
type MockEngine struct {
	ctrl     *gomock.Controller
	recorder *MockEngineMockRecorder
	isgomock struct{}
}

// MockEngineMockRecorder is the mock recorder for MockEngine.
type MockEngineMockRecorder struct {
	mock *MockEngine
}

// NewMockEngine creates a new mock instance.
func NewMockEngine(ctrl *gomock.Controller) *MockEngine {
	mock := &MockEngine{ctrl: ctrl}
	mock.recorder = &MockEngineMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEngine) EXPECT() *MockEngineMockRecorder {
	return m.recorder
}

// newEntity mocks base method.
func (m *MockEngine) newEntity(arg0 context.Context, arg1 ComponentRef, arg2 func(MutableContext) (Component, error), arg3 ...TransitionOption) (EntityKey, []byte, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "newEntity", varargs...)
	ret0, _ := ret[0].(EntityKey)
	ret1, _ := ret[1].([]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// newEntity indicates an expected call of newEntity.
func (mr *MockEngineMockRecorder) newEntity(arg0, arg1, arg2 any, arg3 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "newEntity", reflect.TypeOf((*MockEngine)(nil).newEntity), varargs...)
}

// pollComponent mocks base method.
func (m *MockEngine) pollComponent(arg0 context.Context, arg1 ComponentRef, arg2 func(Context, Component) (any, bool, error), arg3 func(MutableContext, Component, any) error, arg4 ...TransitionOption) ([]byte, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2, arg3}
	for _, a := range arg4 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "pollComponent", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// pollComponent indicates an expected call of pollComponent.
func (mr *MockEngineMockRecorder) pollComponent(arg0, arg1, arg2, arg3 any, arg4 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2, arg3}, arg4...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "pollComponent", reflect.TypeOf((*MockEngine)(nil).pollComponent), varargs...)
}

// readComponent mocks base method.
func (m *MockEngine) readComponent(arg0 context.Context, arg1 ComponentRef, arg2 func(Context, Component) error, arg3 ...TransitionOption) error {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "readComponent", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// readComponent indicates an expected call of readComponent.
func (mr *MockEngineMockRecorder) readComponent(arg0, arg1, arg2 any, arg3 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "readComponent", reflect.TypeOf((*MockEngine)(nil).readComponent), varargs...)
}

// updateComponent mocks base method.
func (m *MockEngine) updateComponent(arg0 context.Context, arg1 ComponentRef, arg2 func(MutableContext, Component) error, arg3 ...TransitionOption) ([]byte, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "updateComponent", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// updateComponent indicates an expected call of updateComponent.
func (mr *MockEngineMockRecorder) updateComponent(arg0, arg1, arg2 any, arg3 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "updateComponent", reflect.TypeOf((*MockEngine)(nil).updateComponent), varargs...)
}

// updateWithNewEntity mocks base method.
func (m *MockEngine) updateWithNewEntity(arg0 context.Context, arg1 ComponentRef, arg2 func(MutableContext) (Component, error), arg3 func(MutableContext, Component) error, arg4 ...TransitionOption) (EntityKey, []byte, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2, arg3}
	for _, a := range arg4 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "updateWithNewEntity", varargs...)
	ret0, _ := ret[0].(EntityKey)
	ret1, _ := ret[1].([]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// updateWithNewEntity indicates an expected call of updateWithNewEntity.
func (mr *MockEngineMockRecorder) updateWithNewEntity(arg0, arg1, arg2, arg3 any, arg4 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2, arg3}, arg4...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "updateWithNewEntity", reflect.TypeOf((*MockEngine)(nil).updateWithNewEntity), varargs...)
}
