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

package rpc

import (
	"context"
	"time"

	"github.com/temporalio/temporal/common/headers"
)

// NewContextWithTimeout creates context with timeout.
func NewContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// NewContextWithTimeoutAndHeaders creates context with timeout and version headers.
func NewContextWithTimeoutAndHeaders(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(headers.SetVersions(context.Background()), timeout)
}

// NewContextFromParentWithTimeoutAndHeaders creates context from parent context with timeout and version headers.
func NewContextFromParentWithTimeoutAndHeaders(parentCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(headers.SetVersions(parentCtx), timeout)
}

// NewContextWithCLIHeaders creates context with version headers for CLI.
func NewContextWithCLIHeaders() (context.Context, context.CancelFunc) {
	return context.WithCancel(headers.SetCLIVersions(context.Background()))
}

// NewContextWithTimeoutAndCLIHeaders creates context with timeout and version headers for CLI.
func NewContextWithTimeoutAndCLIHeaders(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(headers.SetCLIVersions(context.Background()), timeout)
}
