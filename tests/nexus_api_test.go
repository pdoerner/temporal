// The MIT License
//
// Copyright (c) 2024 Temporal Technologies Inc.  All rights reserved.
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

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/assert"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	nexuspb "go.temporal.io/api/nexus/v1"
	"go.temporal.io/api/operatorservice/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	sdkclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/types/known/structpb"

	tokenspb "go.temporal.io/server/api/token/v1"
	"go.temporal.io/server/common/authorization"
	"go.temporal.io/server/common/metrics"
	"go.temporal.io/server/common/metrics/metricstest"
	cnexus "go.temporal.io/server/common/nexus"
	"go.temporal.io/server/service/frontend/configs"
)

var op = nexus.NewOperationReference[string, string]("my-operation")

func (s *ClientFunctionalSuite) mustToPayload(v any) *commonpb.Payload {
	conv := converter.GetDefaultDataConverter()
	payload, err := conv.ToPayload(v)
	s.NoError(err)
	return payload
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_Outcomes() {
	type testcase struct {
		outcome         string
		incomingService *nexuspb.IncomingService
		timeout         time.Duration
		handler         func(*workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError)
		assertion       func(*nexus.ClientStartOperationResult[string], error)
	}

	testCases := []testcase{
		{
			outcome:         "sync_success",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler:         nexusEchoHandler,
			assertion: func(res *nexus.ClientStartOperationResult[string], err error) {
				s.NoError(err)
				s.Equal("input", res.Successful)
			},
		},
		{
			outcome:         "async_success",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				// Choose an arbitrary test case to assert that all of the input is delivered to the
				// poll response.
				start := res.Request.Variant.(*nexuspb.Request_StartOperation).StartOperation
				s.Equal(op.Name(), start.Operation)
				s.Equal("http://localhost/callback", start.Callback)
				s.Equal("request-id", start.RequestId)
				s.Equal("value", res.Request.Header["key"])
				return &nexuspb.Response{
					Variant: &nexuspb.Response_StartOperation{
						StartOperation: &nexuspb.StartOperationResponse{
							Variant: &nexuspb.StartOperationResponse_AsyncSuccess{
								AsyncSuccess: &nexuspb.StartOperationResponse_Async{
									OperationId: "test-id",
								},
							},
						},
					},
				}, nil
			},
			assertion: func(res *nexus.ClientStartOperationResult[string], err error) {
				s.NoError(err)
				s.Equal("test-id", res.Pending.ID)
			},
		},
		{
			outcome:         "operation_error",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				return &nexuspb.Response{
					Variant: &nexuspb.Response_StartOperation{
						StartOperation: &nexuspb.StartOperationResponse{
							Variant: &nexuspb.StartOperationResponse_OperationError{
								OperationError: &nexuspb.UnsuccessfulOperationError{
									OperationState: string(nexus.OperationStateFailed),
									Failure: &nexuspb.Failure{
										Message:  "deliberate test failure",
										Metadata: map[string]string{"k": "v"},
										Details:  structpb.NewStringValue("details"),
									},
								},
							},
						},
					},
				}, nil
			},
			assertion: func(res *nexus.ClientStartOperationResult[string], err error) {
				var operationError *nexus.UnsuccessfulOperationError
				s.ErrorAs(err, &operationError)
				s.Equal(nexus.OperationStateFailed, operationError.State)
				s.Equal("deliberate test failure", operationError.Failure.Message)
				s.Equal(map[string]string{"k": "v"}, operationError.Failure.Metadata)
				var details string
				err = json.Unmarshal(operationError.Failure.Details, &details)
				s.NoError(err)
				s.Equal("details", details)
			},
		},
		{
			outcome:         "handler_error",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				return nil, &nexuspb.HandlerError{
					ErrorType: string(nexus.HandlerErrorTypeInternal),
					Failure:   &nexuspb.Failure{Message: "deliberate internal failure"},
				}
			},
			assertion: func(res *nexus.ClientStartOperationResult[string], err error) {
				var unexpectedError *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedError)
				// TODO: nexus should export this
				s.Equal(520, unexpectedError.Response.StatusCode)
				s.Equal("deliberate internal failure", unexpectedError.Failure.Message)
			},
		},
		{
			outcome:         "handler_timeout",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			timeout:         1100 * time.Millisecond,
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				timeoutStr, set := res.Request.Header[nexus.HeaderRequestTimeout]
				s.True(set)
				timeout, err := time.ParseDuration(timeoutStr)
				s.NoError(err)
				time.Sleep(timeout)
				return nil, nil
			},
			assertion: func(res *nexus.ClientStartOperationResult[string], err error) {
				var unexpectedError *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedError)
				// TODO: nexus should export this
				s.Equal(521, unexpectedError.Response.StatusCode)
				s.Equal("downstream timeout", unexpectedError.Failure.Message)
			},
		},
	}

	testFn := func(t *testing.T, tc testcase, dispatchURL string) {
		ctx := NewContext()

		client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: dispatchURL})
		s.NoError(err)
		capture := s.testCluster.host.captureMetricsHandler.StartCapture()
		defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)

		go s.nexusTaskPoller(ctx, tc.incomingService.Spec.TaskQueue, tc.handler)

		eventuallyTick := 500 * time.Millisecond
		header := nexus.Header{"key": "value"}
		if tc.timeout > 0 {
			eventuallyTick = tc.timeout + (100 * time.Millisecond)
			header[nexus.HeaderRequestTimeout] = tc.timeout.String()
		}

		// Use EventuallyWithT to retry if incoming service has not been loaded into memory when the test starts.
		s.EventuallyWithT(func(c *assert.CollectT) {
			result, err := nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{
				CallbackURL: "http://localhost/callback",
				RequestID:   "request-id",
				Header:      header,
			})
			tc.assertion(result, err)
		}, 5*time.Second, eventuallyTick)

		snap := capture.Snapshot()

		s.Equal(1, len(snap["nexus_requests"]))
		s.Subset(snap["nexus_requests"][0].Tags, map[string]string{"namespace": s.namespace, "method": "StartOperation", "outcome": tc.outcome})
		s.Contains(snap["nexus_requests"][0].Tags, "service")
		s.Equal(int64(1), snap["nexus_requests"][0].Value)
		s.Equal(metrics.MetricUnit(""), snap["nexus_requests"][0].Unit)

		s.Equal(1, len(snap["nexus_latency"]))
		s.Subset(snap["nexus_latency"][0].Tags, map[string]string{"namespace": s.namespace, "method": "StartOperation", "outcome": tc.outcome})
		s.Contains(snap["nexus_latency"][0].Tags, "service")
		s.Equal(metrics.MetricUnit(metrics.Milliseconds), snap["nexus_latency"][0].Unit)
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.outcome, func(t *testing.T) {
			t.Run("ByNamespaceAndTaskQueue", func(t *testing.T) {
				testFn(t, tc, getDispatchByNsAndTqURL(s.httpAPIAddress, s.namespace, tc.incomingService.Spec.TaskQueue))
			})
			t.Run("ByService", func(t *testing.T) {
				testFn(t, tc, getDispatchByServiceURL(s.httpAPIAddress, tc.incomingService.Id))
			})
		})
	}
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_WithNamespaceAndTaskQueue_NamespaceNotFound() {
	// Also use this test to verify that namespaces are unescaped in the path.
	taskQueue := s.randomizeStr("task-queue")
	namespace := "namespace not/found"
	u := getDispatchByNsAndTqURL(s.httpAPIAddress, namespace, taskQueue)
	client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: u})
	s.NoError(err)
	ctx := NewContext()
	capture := s.testCluster.host.captureMetricsHandler.StartCapture()
	defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)
	_, err = nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
	var unexpectedResponse *nexus.UnexpectedResponseError
	s.ErrorAs(err, &unexpectedResponse)
	s.Equal(http.StatusNotFound, unexpectedResponse.Response.StatusCode)
	s.Equal(fmt.Sprintf("namespace not found: %q", namespace), unexpectedResponse.Failure.Message)

	snap := capture.Snapshot()

	s.Equal(1, len(snap["nexus_requests"]))
	s.Equal(map[string]string{"namespace": namespace, "method": "StartOperation", "outcome": "namespace_not_found", "service": "_unknown_"}, snap["nexus_requests"][0].Tags)
	s.Equal(int64(1), snap["nexus_requests"][0].Value)
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_WithNamespaceAndTaskQueue_NamespaceTooLong() {
	taskQueue := s.randomizeStr("task-queue")

	var namespace string
	for i := 0; i < 500; i++ {
		namespace += "namespace-is-a-very-long-string"
	}

	u := getDispatchByNsAndTqURL(s.httpAPIAddress, namespace, taskQueue)
	client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: u})
	s.NoError(err)
	ctx := NewContext()
	capture := s.testCluster.host.captureMetricsHandler.StartCapture()
	defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)
	_, err = nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
	var unexpectedResponse *nexus.UnexpectedResponseError
	s.ErrorAs(err, &unexpectedResponse)
	s.Equal(http.StatusBadRequest, unexpectedResponse.Response.StatusCode)
	// I wish we'd never put periods in error messages :(
	s.Equal("Namespace length exceeds limit.", unexpectedResponse.Failure.Message)

	snap := capture.Snapshot()

	s.Equal(1, len(snap["nexus_request_preprocess_errors"]))
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_Forbidden() {
	taskQueue := s.randomizeStr("task-queue")
	testService := s.createNexusIncomingService(s.randomizeStr("test-service"), taskQueue)

	type testcase struct {
		name           string
		onAuthorize    func(context.Context, *authorization.Claims, *authorization.CallTarget) (authorization.Result, error)
		failureMessage string
	}
	testCases := []testcase{
		{
			name: "deny with reason",
			onAuthorize: func(ctx context.Context, c *authorization.Claims, ct *authorization.CallTarget) (authorization.Result, error) {
				if ct.APIName == configs.DispatchNexusTaskByNamespaceAndTaskQueueAPIName {
					return authorization.Result{Decision: authorization.DecisionDeny, Reason: "unauthorized in test"}, nil
				}
				if ct.APIName == configs.DispatchNexusTaskByServiceAPIName {
					return authorization.Result{Decision: authorization.DecisionDeny, Reason: "unauthorized in test"}, nil
				}
				return authorization.Result{Decision: authorization.DecisionAllow}, nil
			},
			failureMessage: `permission denied: unauthorized in test`,
		},
		{
			name: "deny without reason",
			onAuthorize: func(ctx context.Context, c *authorization.Claims, ct *authorization.CallTarget) (authorization.Result, error) {
				if ct.APIName == configs.DispatchNexusTaskByNamespaceAndTaskQueueAPIName {
					return authorization.Result{Decision: authorization.DecisionDeny}, nil
				}
				if ct.APIName == configs.DispatchNexusTaskByServiceAPIName {
					return authorization.Result{Decision: authorization.DecisionDeny}, nil
				}
				return authorization.Result{Decision: authorization.DecisionAllow}, nil
			},
			failureMessage: "permission denied",
		},
		{
			name: "deny with generic error",
			onAuthorize: func(ctx context.Context, c *authorization.Claims, ct *authorization.CallTarget) (authorization.Result, error) {
				if ct.APIName == configs.DispatchNexusTaskByNamespaceAndTaskQueueAPIName {
					return authorization.Result{}, errors.New("some generic error")
				}
				if ct.APIName == configs.DispatchNexusTaskByServiceAPIName {
					return authorization.Result{}, errors.New("some generic error")
				}
				return authorization.Result{Decision: authorization.DecisionAllow}, nil
			},
			failureMessage: "permission denied",
		},
	}

	testFn := func(t *testing.T, tc testcase, dispatchURL string) {
		client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: dispatchURL})
		s.NoError(err)
		ctx := NewContext()

		capture := s.testCluster.host.captureMetricsHandler.StartCapture()
		defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)

		// Use EventuallyWithT to retry if incoming service has not been loaded into memory when the test starts.
		s.EventuallyWithT(func(c *assert.CollectT) {
			_, err = nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
			var unexpectedResponse *nexus.UnexpectedResponseError
			s.ErrorAs(err, &unexpectedResponse)
			s.Equal(http.StatusForbidden, unexpectedResponse.Response.StatusCode)
			s.Equal(tc.failureMessage, unexpectedResponse.Failure.Message)
		}, 5*time.Second, 500*time.Millisecond)

		snap := capture.Snapshot()

		s.Equal(1, len(snap["nexus_requests"]))
		s.Subset(snap["nexus_requests"][0].Tags, map[string]string{"namespace": s.namespace, "method": "StartOperation", "outcome": "unauthorized"})
		s.Equal(int64(1), snap["nexus_requests"][0].Value)
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			s.testCluster.host.SetOnAuthorize(tc.onAuthorize)
			defer s.testCluster.host.SetOnAuthorize(nil)

			t.Run("ByNamespaceAndTaskQueue", func(t *testing.T) {
				testFn(t, tc, getDispatchByNsAndTqURL(s.httpAPIAddress, s.namespace, taskQueue))
			})
			t.Run("ByService", func(t *testing.T) {
				testFn(t, tc, getDispatchByServiceURL(s.httpAPIAddress, testService.Id))
			})
		})
	}
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_Claims() {
	taskQueue := s.randomizeStr("task-queue")
	testService := s.createNexusIncomingService(s.randomizeStr("test-service"), taskQueue)

	type testcase struct {
		name      string
		header    nexus.Header
		handler   func(*workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError)
		assertion func(*nexus.ClientStartOperationResult[string], error, map[string][]*metricstest.CapturedRecording)
	}
	testCases := []testcase{
		{
			name: "no header",
			assertion: func(res *nexus.ClientStartOperationResult[string], err error, snap map[string][]*metricstest.CapturedRecording) {
				var unexpectedResponse *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedResponse)
				s.Equal(http.StatusForbidden, unexpectedResponse.Response.StatusCode)
				s.Equal("permission denied", unexpectedResponse.Failure.Message)
				s.Equal(0, len(snap["nexus_request_preprocess_errors"]))
			},
		},
		{
			name: "invalid bearer",
			header: nexus.Header{
				"authorization": "Bearer invalid",
			},
			assertion: func(res *nexus.ClientStartOperationResult[string], err error, snap map[string][]*metricstest.CapturedRecording) {
				var unexpectedResponse *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedResponse)
				s.Equal(http.StatusUnauthorized, unexpectedResponse.Response.StatusCode)
				s.Equal("unauthorized", unexpectedResponse.Failure.Message)
				s.Equal(1, len(snap["nexus_request_preprocess_errors"]))
			},
		},
		{
			name: "valid bearer",
			header: nexus.Header{
				"authorization": "Bearer test",
			},
			handler: nexusEchoHandler,
			assertion: func(res *nexus.ClientStartOperationResult[string], err error, snap map[string][]*metricstest.CapturedRecording) {
				s.NoError(err)
				s.Equal("input", res.Successful)
				s.Equal(0, len(snap["nexus_request_preprocess_errors"]))
			},
		},
	}

	s.testCluster.host.SetOnAuthorize(func(ctx context.Context, c *authorization.Claims, ct *authorization.CallTarget) (authorization.Result, error) {
		if ct.APIName == configs.DispatchNexusTaskByNamespaceAndTaskQueueAPIName && (c == nil || c.Subject != "test") {
			return authorization.Result{Decision: authorization.DecisionDeny}, nil
		}
		if ct.APIName == configs.DispatchNexusTaskByServiceAPIName && (c == nil || c.Subject != "test") {
			return authorization.Result{Decision: authorization.DecisionDeny}, nil
		}
		return authorization.Result{Decision: authorization.DecisionAllow}, nil
	})
	defer s.testCluster.host.SetOnAuthorize(nil)

	s.testCluster.host.SetOnGetClaims(func(ai *authorization.AuthInfo) (*authorization.Claims, error) {
		if ai.AuthToken != "Bearer test" {
			return nil, errors.New("invalid auth token")
		}
		return &authorization.Claims{Subject: "test"}, nil
	})
	defer s.testCluster.host.SetOnGetClaims(nil)

	testFn := func(t *testing.T, tc testcase, dispatchURL string) {
		ctx := NewContext()

		client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: dispatchURL})
		s.NoError(err)

		if tc.handler != nil {
			// only set on valid request
			go s.nexusTaskPoller(ctx, taskQueue, tc.handler)
		}

		capture := s.testCluster.host.captureMetricsHandler.StartCapture()
		defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)

		// Use EventuallyWithT to retry if incoming service has not been loaded into memory when the test starts.
		s.EventuallyWithT(func(c *assert.CollectT) {
			result, err := nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{
				Header: tc.header,
			})

			snap := capture.Snapshot()
			tc.assertion(result, err, snap)
		}, 5*time.Second, 500*time.Millisecond)
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			t.Run("ByNamespaceAndTaskQueue", func(t *testing.T) {
				testFn(t, tc, getDispatchByNsAndTqURL(s.httpAPIAddress, s.namespace, taskQueue))
			})
			t.Run("ByService", func(t *testing.T) {
				testFn(t, tc, getDispatchByServiceURL(s.httpAPIAddress, testService.Id))
			})
		})
	}
}

func (s *ClientFunctionalSuite) TestNexusCancelOperation_Outcomes() {
	type testcase struct {
		outcome         string
		incomingService *nexuspb.IncomingService
		timeout         time.Duration
		handler         func(*workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError)
		assertion       func(error)
	}

	testCases := []testcase{
		{
			outcome:         "success",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				// Choose an arbitrary test case to assert that all of the input is delivered to the
				// poll response.
				op := res.Request.Variant.(*nexuspb.Request_CancelOperation).CancelOperation
				s.Equal("operation", op.Operation)
				s.Equal("id", op.OperationId)
				s.Equal("value", res.Request.Header["key"])
				return &nexuspb.Response{
					Variant: &nexuspb.Response_CancelOperation{
						CancelOperation: &nexuspb.CancelOperationResponse{},
					},
				}, nil
			},
			assertion: func(err error) {
				s.NoError(err)
			},
		},
		{
			outcome:         "handler_error",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				return nil, &nexuspb.HandlerError{
					ErrorType: string(nexus.HandlerErrorTypeInternal),
					Failure:   &nexuspb.Failure{Message: "deliberate internal failure"},
				}
			},
			assertion: func(err error) {
				var unexpectedError *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedError)
				// TODO: nexus should export this
				s.Equal(520, unexpectedError.Response.StatusCode)
				s.Equal("deliberate internal failure", unexpectedError.Failure.Message)
			},
		},
		{
			outcome:         "handler_timeout",
			incomingService: s.createNexusIncomingService(s.randomizeStr("test-service"), s.randomizeStr("task-queue")),
			timeout:         1100 * time.Millisecond,
			handler: func(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
				timeoutStr, set := res.Request.Header[nexus.HeaderRequestTimeout]
				s.True(set)
				timeout, err := time.ParseDuration(timeoutStr)
				s.NoError(err)
				time.Sleep(timeout)
				return nil, nil
			},
			assertion: func(err error) {
				var unexpectedError *nexus.UnexpectedResponseError
				s.ErrorAs(err, &unexpectedError)
				// TODO: nexus should export this
				s.Equal(521, unexpectedError.Response.StatusCode)
				s.Equal("downstream timeout", unexpectedError.Failure.Message)
			},
		},
	}

	testFn := func(t *testing.T, tc testcase, dispatchURL string) {
		ctx := NewContext()

		client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: dispatchURL})
		s.NoError(err)
		capture := s.testCluster.host.captureMetricsHandler.StartCapture()
		defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)

		go s.nexusTaskPoller(ctx, tc.incomingService.Spec.TaskQueue, tc.handler)

		handle, err := client.NewHandle("operation", "id")
		s.NoError(err)

		eventuallyTick := 500 * time.Millisecond
		header := nexus.Header{"key": "value"}
		if tc.timeout > 0 {
			eventuallyTick = tc.timeout + (100 * time.Millisecond)
			header[nexus.HeaderRequestTimeout] = tc.timeout.String()
		}

		// Use EventuallyWithT to retry if incoming service has not been loaded into memory when the test starts.
		s.EventuallyWithT(func(c *assert.CollectT) {
			err = handle.Cancel(ctx, nexus.CancelOperationOptions{Header: header})
			tc.assertion(err)
		}, 5*time.Second, eventuallyTick)

		snap := capture.Snapshot()

		s.Equal(1, len(snap["nexus_requests"]))
		s.Subset(snap["nexus_requests"][0].Tags, map[string]string{"namespace": s.namespace, "method": "CancelOperation", "outcome": tc.outcome})
		s.Contains(snap["nexus_requests"][0].Tags, "service")
		s.Equal(int64(1), snap["nexus_requests"][0].Value)
		s.Equal(metrics.MetricUnit(""), snap["nexus_requests"][0].Unit)

		s.Equal(1, len(snap["nexus_latency"]))
		s.Subset(snap["nexus_latency"][0].Tags, map[string]string{"namespace": s.namespace, "method": "CancelOperation", "outcome": tc.outcome})
		s.Contains(snap["nexus_latency"][0].Tags, "service")
		s.Equal(metrics.MetricUnit(metrics.Milliseconds), snap["nexus_latency"][0].Unit)
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.outcome, func(t *testing.T) {
			t.Run("ByNamespaceAndTaskQueue", func(t *testing.T) {
				testFn(t, tc, getDispatchByNsAndTqURL(s.httpAPIAddress, s.namespace, tc.incomingService.Spec.TaskQueue))
			})
			t.Run("ByService", func(t *testing.T) {
				testFn(t, tc, getDispatchByServiceURL(s.httpAPIAddress, tc.incomingService.Id))
			})
		})
	}
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_WithNamespaceAndTaskQueue_SupportsVersioning() {
	ctx, cancel := context.WithCancel(NewContext())
	defer cancel()
	taskQueue := s.randomizeStr("task-queue")
	err := s.sdkClient.UpdateWorkerBuildIdCompatibility(ctx, &sdkclient.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: taskQueue,
		Operation: &sdkclient.BuildIDOpAddNewIDInNewDefaultSet{BuildID: "old-build-id"},
	})
	s.NoError(err)
	err = s.sdkClient.UpdateWorkerBuildIdCompatibility(ctx, &sdkclient.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: taskQueue,
		Operation: &sdkclient.BuildIDOpAddNewIDInNewDefaultSet{BuildID: "new-build-id"},
	})
	s.NoError(err)

	u := getDispatchByNsAndTqURL(s.httpAPIAddress, s.namespace, taskQueue)
	client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: u})
	s.NoError(err)
	// Versioned poller gets task
	go s.versionedNexusTaskPoller(ctx, taskQueue, "new-build-id", nexusEchoHandler)

	result, err := nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
	s.NoError(err)
	s.Equal("input", result.Successful)

	// Unversioned poller doesn't get a task
	go s.nexusTaskPoller(ctx, taskQueue, nexusEchoHandler)
	// Versioned poller gets task with wrong build ID
	go s.versionedNexusTaskPoller(ctx, taskQueue, "old-build-id", nexusEchoHandler)

	ctx, cancel = context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	_, err = nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
	s.ErrorIs(err, context.DeadlineExceeded)
}

func (s *ClientFunctionalSuite) TestNexus_RespondNexusTaskMethods_VerifiesTaskTokenMatchesRequestNamespace() {
	ctx := NewContext()

	tt := tokenspb.NexusTask{
		NamespaceId: s.getNamespaceID(s.namespace),
		TaskQueue:   "test",
		TaskId:      uuid.NewString(),
	}
	ttBytes, err := tt.Marshal()
	s.NoError(err)

	_, err = s.testCluster.GetFrontendClient().RespondNexusTaskCompleted(ctx, &workflowservice.RespondNexusTaskCompletedRequest{
		Namespace: s.foreignNamespace,
		Identity:  uuid.NewString(),
		TaskToken: ttBytes,
		Response:  &nexuspb.Response{},
	})
	s.ErrorContains(err, "Operation requested with a token from a different namespace.")

	_, err = s.testCluster.GetFrontendClient().RespondNexusTaskFailed(ctx, &workflowservice.RespondNexusTaskFailedRequest{
		Namespace: s.foreignNamespace,
		Identity:  uuid.NewString(),
		TaskToken: ttBytes,
		Error:     &nexuspb.HandlerError{},
	})
	s.ErrorContains(err, "Operation requested with a token from a different namespace.")
}

func (s *ClientFunctionalSuite) TestNexusStartOperation_ByService_ServiceNotFound() {
	u := getDispatchByServiceURL(s.httpAPIAddress, uuid.NewString())
	client, err := nexus.NewClient(nexus.ClientOptions{ServiceBaseURL: u})
	s.NoError(err)
	ctx := NewContext()
	capture := s.testCluster.host.captureMetricsHandler.StartCapture()
	defer s.testCluster.host.captureMetricsHandler.StopCapture(capture)
	_, err = nexus.StartOperation(ctx, client, op, "input", nexus.StartOperationOptions{})
	var unexpectedResponse *nexus.UnexpectedResponseError
	s.ErrorAs(err, &unexpectedResponse)
	s.Equal(http.StatusNotFound, unexpectedResponse.Response.StatusCode)
	s.Equal("nexus service not found", unexpectedResponse.Failure.Message)
	snap := capture.Snapshot()
	s.Equal(1, len(snap["nexus_request_preprocess_errors"]))
}

func (s *ClientFunctionalSuite) echoNexusTaskPoller(ctx context.Context, taskQueue string) {
	s.nexusTaskPoller(ctx, taskQueue, nexusEchoHandler)
}

func (s *ClientFunctionalSuite) nexusTaskPoller(ctx context.Context, taskQueue string, handler func(*workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError)) {
	s.versionedNexusTaskPoller(ctx, taskQueue, "", handler)
}

func (s *ClientFunctionalSuite) versionedNexusTaskPoller(ctx context.Context, taskQueue, buildID string, handler func(*workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError)) {
	var vc *commonpb.WorkerVersionCapabilities

	if buildID != "" {
		vc = &commonpb.WorkerVersionCapabilities{
			BuildId:       buildID,
			UseVersioning: true,
		}
	}
	res, err := s.testCluster.GetFrontendClient().PollNexusTaskQueue(ctx, &workflowservice.PollNexusTaskQueueRequest{
		Namespace: s.namespace,
		Identity:  uuid.NewString(),
		TaskQueue: &taskqueuepb.TaskQueue{
			Name: taskQueue,
			Kind: enumspb.TASK_QUEUE_KIND_NORMAL,
		},
		WorkerVersionCapabilities: vc,
	})
	// The test is written in a way that it doesn't expect the poll to be unblocked and it may cancel this context when it completes.
	if ctx.Err() != nil {
		return
	}
	// There's no clean way to propagate this error back to the test that's worthwhile. Panic is good enough.
	if err != nil {
		panic(err)
	}
	response, handlerError := handler(res)
	if handlerError != nil {
		_, err = s.testCluster.GetFrontendClient().RespondNexusTaskFailed(ctx, &workflowservice.RespondNexusTaskFailedRequest{
			Namespace: s.namespace,
			Identity:  uuid.NewString(),
			TaskToken: res.TaskToken,
			Error:     handlerError,
		})
		// There's no clean way to propagate this error back to the test that's worthwhile. Panic is good enough.
		if err != nil {
			panic(err)
		}
	} else if response != nil {
		_, err = s.testCluster.GetFrontendClient().RespondNexusTaskCompleted(ctx, &workflowservice.RespondNexusTaskCompletedRequest{
			Namespace: s.namespace,
			Identity:  uuid.NewString(),
			TaskToken: res.TaskToken,
			Response:  response,
		})
		// There's no clean way to propagate this error back to the test that's worthwhile. Panic is good enough.
		if err != nil {
			panic(err)
		}
	}
}

func nexusEchoHandler(res *workflowservice.PollNexusTaskQueueResponse) (*nexuspb.Response, *nexuspb.HandlerError) {
	return &nexuspb.Response{
		Variant: &nexuspb.Response_StartOperation{
			StartOperation: &nexuspb.StartOperationResponse{
				Variant: &nexuspb.StartOperationResponse_SyncSuccess{
					SyncSuccess: &nexuspb.StartOperationResponse_Sync{
						Payload: res.Request.GetStartOperation().GetPayload(),
					},
				},
			},
		},
	}, nil
}

func getDispatchByNsAndTqURL(address string, namespace string, taskQueue string) string {
	return fmt.Sprintf(
		"http://%s/%s",
		address,
		cnexus.RouteDispatchNexusTaskByNamespaceAndTaskQueue.
			Path(cnexus.NamespaceAndTaskQueue{
				Namespace: url.PathEscape(namespace),
				TaskQueue: taskQueue,
			}),
	)
}

func (s *ClientFunctionalSuite) createNexusIncomingService(name string, taskQueue string) *nexuspb.IncomingService {
	resp, err := s.operatorClient.CreateNexusIncomingService(NewContext(), &operatorservice.CreateNexusIncomingServiceRequest{
		Spec: &nexuspb.IncomingServiceSpec{
			Name:      name,
			Namespace: s.namespace,
			TaskQueue: taskQueue,
		},
	})
	s.NoError(err)
	return resp.Service
}

func getDispatchByServiceURL(address string, service string) string {
	return fmt.Sprintf("http://%s/%s", address, cnexus.RouteDispatchNexusTaskByService.Path(service))
}
