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

//go:generate mockgen -copyright_file ../LICENSE -package $GOPACKAGE -source $GOFILE -destination task_mock.go

package chasm

import (
	"context"
	"time"
)

type (
	TaskAttributes struct {
		ScheduledTime time.Time
		Destination   string
	}

	SideEffectTaskExecutor[C any, T any] interface {
		Execute(context.Context, ComponentRef, T) error
	}

	PureTaskExecutor[C any, T any] interface {
		Execute(Context, C, T) error
	}

	TaskValidator[C any, T any] interface {
		Validate(Context, C, T) (bool, error)
	}
)

var TaskScheduledTimeImmediate = time.Time{}
