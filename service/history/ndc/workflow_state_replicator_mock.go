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
// Source: workflow_state_replicator.go
//
// Generated by this command:
//
//	mockgen -copyright_file ../../../LICENSE -package ndc -source workflow_state_replicator.go -destination workflow_state_replicator_mock.go
//

// Package ndc is a generated GoMock package.
package ndc

import (
	context "context"
	reflect "reflect"

	historyservice "go.temporal.io/server/api/historyservice/v1"
	repication "go.temporal.io/server/api/replication/v1"
	gomock "go.uber.org/mock/gomock"
)

// MockWorkflowStateReplicator is a mock of WorkflowStateReplicator interface.
type MockWorkflowStateReplicator struct {
	ctrl     *gomock.Controller
	recorder *MockWorkflowStateReplicatorMockRecorder
	isgomock struct{}
}

// MockWorkflowStateReplicatorMockRecorder is the mock recorder for MockWorkflowStateReplicator.
type MockWorkflowStateReplicatorMockRecorder struct {
	mock *MockWorkflowStateReplicator
}

// NewMockWorkflowStateReplicator creates a new mock instance.
func NewMockWorkflowStateReplicator(ctrl *gomock.Controller) *MockWorkflowStateReplicator {
	mock := &MockWorkflowStateReplicator{ctrl: ctrl}
	mock.recorder = &MockWorkflowStateReplicatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWorkflowStateReplicator) EXPECT() *MockWorkflowStateReplicatorMockRecorder {
	return m.recorder
}

// ReplicateVersionedTransition mocks base method.
func (m *MockWorkflowStateReplicator) ReplicateVersionedTransition(ctx context.Context, versionedTransition *repication.VersionedTransitionArtifact, sourceClusterName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplicateVersionedTransition", ctx, versionedTransition, sourceClusterName)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReplicateVersionedTransition indicates an expected call of ReplicateVersionedTransition.
func (mr *MockWorkflowStateReplicatorMockRecorder) ReplicateVersionedTransition(ctx, versionedTransition, sourceClusterName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplicateVersionedTransition", reflect.TypeOf((*MockWorkflowStateReplicator)(nil).ReplicateVersionedTransition), ctx, versionedTransition, sourceClusterName)
}

// SyncWorkflowState mocks base method.
func (m *MockWorkflowStateReplicator) SyncWorkflowState(ctx context.Context, request *historyservice.ReplicateWorkflowStateRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncWorkflowState", ctx, request)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncWorkflowState indicates an expected call of SyncWorkflowState.
func (mr *MockWorkflowStateReplicatorMockRecorder) SyncWorkflowState(ctx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncWorkflowState", reflect.TypeOf((*MockWorkflowStateReplicator)(nil).SyncWorkflowState), ctx, request)
}
