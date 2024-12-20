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

package tasks

import (
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	enumsspb "go.temporal.io/server/api/enums/v1"
	"go.temporal.io/server/common/definition"
)

var _ Task = (*ActivityTimeoutTask)(nil)

type (
	ActivityTimeoutTask struct {
		definition.WorkflowKey
		VisibilityTimestamp time.Time
		TaskID              int64
		TimeoutType         enumspb.TimeoutType
		EventID             int64
		Attempt             int32
		Stamp               int32
	}
)

func (a *ActivityTimeoutTask) GetKey() Key {
	return NewKey(a.VisibilityTimestamp, a.TaskID)
}

func (a *ActivityTimeoutTask) GetTaskID() int64 {
	return a.TaskID
}

func (a *ActivityTimeoutTask) SetTaskID(id int64) {
	a.TaskID = id
}

func (a *ActivityTimeoutTask) GetVisibilityTime() time.Time {
	return a.VisibilityTimestamp
}

func (a *ActivityTimeoutTask) SetVisibilityTime(t time.Time) {
	a.VisibilityTimestamp = t
}

func (a *ActivityTimeoutTask) GetCategory() Category {
	return CategoryTimer
}

func (a *ActivityTimeoutTask) GetType() enumsspb.TaskType {
	return enumsspb.TASK_TYPE_ACTIVITY_TIMEOUT
}

func (a *ActivityTimeoutTask) GetStamp() int32 {
	return a.Stamp
}

func (a *ActivityTimeoutTask) SetStamp(stamp int32) {
	a.Stamp = stamp
}
