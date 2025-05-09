// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Code generated by MockGen. DO NOT EDIT.
// Source: task.go
//
// Generated by this command:
//
//	mockgen -copyright_file ../LICENSE -package chasm -source task.go -destination task_mock.go
//

// Package chasm is a generated GoMock package.
package chasm

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockSideEffectTaskExecutor is a mock of SideEffectTaskExecutor interface.
type MockSideEffectTaskExecutor[C any, T any] struct {
	ctrl     *gomock.Controller
	recorder *MockSideEffectTaskExecutorMockRecorder[C, T]
	isgomock struct{}
}

// MockSideEffectTaskExecutorMockRecorder is the mock recorder for MockSideEffectTaskExecutor.
type MockSideEffectTaskExecutorMockRecorder[C any, T any] struct {
	mock *MockSideEffectTaskExecutor[C, T]
}

// NewMockSideEffectTaskExecutor creates a new mock instance.
func NewMockSideEffectTaskExecutor[C any, T any](ctrl *gomock.Controller) *MockSideEffectTaskExecutor[C, T] {
	mock := &MockSideEffectTaskExecutor[C, T]{ctrl: ctrl}
	mock.recorder = &MockSideEffectTaskExecutorMockRecorder[C, T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSideEffectTaskExecutor[C, T]) EXPECT() *MockSideEffectTaskExecutorMockRecorder[C, T] {
	return m.recorder
}

// Execute mocks base method.
func (m *MockSideEffectTaskExecutor[C, T]) Execute(arg0 context.Context, arg1 ComponentRef, arg2 T) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Execute indicates an expected call of Execute.
func (mr *MockSideEffectTaskExecutorMockRecorder[C, T]) Execute(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockSideEffectTaskExecutor[C, T])(nil).Execute), arg0, arg1, arg2)
}

// MockPureTaskExecutor is a mock of PureTaskExecutor interface.
type MockPureTaskExecutor[C any, T any] struct {
	ctrl     *gomock.Controller
	recorder *MockPureTaskExecutorMockRecorder[C, T]
	isgomock struct{}
}

// MockPureTaskExecutorMockRecorder is the mock recorder for MockPureTaskExecutor.
type MockPureTaskExecutorMockRecorder[C any, T any] struct {
	mock *MockPureTaskExecutor[C, T]
}

// NewMockPureTaskExecutor creates a new mock instance.
func NewMockPureTaskExecutor[C any, T any](ctrl *gomock.Controller) *MockPureTaskExecutor[C, T] {
	mock := &MockPureTaskExecutor[C, T]{ctrl: ctrl}
	mock.recorder = &MockPureTaskExecutorMockRecorder[C, T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPureTaskExecutor[C, T]) EXPECT() *MockPureTaskExecutorMockRecorder[C, T] {
	return m.recorder
}

// Execute mocks base method.
func (m *MockPureTaskExecutor[C, T]) Execute(arg0 Context, arg1 C, arg2 T) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Execute indicates an expected call of Execute.
func (mr *MockPureTaskExecutorMockRecorder[C, T]) Execute(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockPureTaskExecutor[C, T])(nil).Execute), arg0, arg1, arg2)
}

// MockTaskValidator is a mock of TaskValidator interface.
type MockTaskValidator[C any, T any] struct {
	ctrl     *gomock.Controller
	recorder *MockTaskValidatorMockRecorder[C, T]
	isgomock struct{}
}

// MockTaskValidatorMockRecorder is the mock recorder for MockTaskValidator.
type MockTaskValidatorMockRecorder[C any, T any] struct {
	mock *MockTaskValidator[C, T]
}

// NewMockTaskValidator creates a new mock instance.
func NewMockTaskValidator[C any, T any](ctrl *gomock.Controller) *MockTaskValidator[C, T] {
	mock := &MockTaskValidator[C, T]{ctrl: ctrl}
	mock.recorder = &MockTaskValidatorMockRecorder[C, T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskValidator[C, T]) EXPECT() *MockTaskValidatorMockRecorder[C, T] {
	return m.recorder
}

// Validate mocks base method.
func (m *MockTaskValidator[C, T]) Validate(arg0 Context, arg1 C, arg2 T) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Validate indicates an expected call of Validate.
func (mr *MockTaskValidatorMockRecorder[C, T]) Validate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockTaskValidator[C, T])(nil).Validate), arg0, arg1, arg2)
}
