// The MIT License
//
// Copyright (c) 2024 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2024 Uber Technologies, Inc.
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

package workerdeployment

import (
	"time"

	deploymentpb "go.temporal.io/api/deployment/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/workflow"
	deploymentspb "go.temporal.io/server/api/deployment/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultVisibilityRefresh = 5 * time.Minute
	defaultVisibilityGrace   = 3 * time.Minute
)

func DrainageWorkflow(
	ctx workflow.Context,
	unsafeRefreshIntervalGetter func() any,
	unsafeVisibilityGracePeriodGetter func() any,
	args *deploymentspb.DrainageWorkflowArgs,
) error {
	if args.Version == nil {
		return serviceerror.NewInvalidArgument("version cannot be nil")
	}
	activityCtx := workflow.WithActivityOptions(ctx, defaultActivityOptions)
	var a *DrainageActivities

	// listen for done signal sent by parent if started accepting new executions or continued-as-new
	done := false
	workflow.Go(ctx, func(ctx workflow.Context) {
		terminateChan := workflow.GetSignalChannel(ctx, TerminateDrainageSignal)
		terminateChan.Receive(ctx, nil)
		done = true
	})

	// Set status = DRAINING and then sleep for visibilityGracePeriod (to let recently-started workflows arrive in visibility)
	if !args.IsCan { // skip if resuming after the parent continued-as-new
		v := workflow.GetVersion(ctx, "Step1", workflow.DefaultVersion, 1)
		if v == workflow.DefaultVersion { // needs patching because we removed a Signal call
			parentWf := workflow.GetInfo(ctx).ParentWorkflowExecution
			now := timestamppb.New(workflow.Now(ctx))
			drainingInfo := &deploymentpb.VersionDrainageInfo{
				Status:          enumspb.VERSION_DRAINAGE_STATUS_DRAINING,
				LastChangedTime: now,
				LastCheckedTime: now,
			}
			err := workflow.SignalExternalWorkflow(ctx, parentWf.ID, parentWf.RunID, SyncDrainageSignalName, drainingInfo).Get(ctx, nil)
			if err != nil {
				return err
			}
		}
		grace, err := getSafeDurationConfig(ctx, "getVisibilityGracePeriod", unsafeVisibilityGracePeriodGetter, defaultVisibilityGrace)
		if err != nil {
			return err
		}
		_ = workflow.Sleep(ctx, grace)
	}

	for {
		if done {
			return nil
		}
		var info *deploymentpb.VersionDrainageInfo
		err := workflow.ExecuteActivity(
			activityCtx,
			a.GetVersionDrainageStatus,
			args.Version,
		).Get(ctx, &info)
		if err != nil {
			return err
		}

		parentWf := workflow.GetInfo(ctx).ParentWorkflowExecution
		err = workflow.SignalExternalWorkflow(ctx, parentWf.ID, parentWf.RunID, SyncDrainageSignalName, info).Get(ctx, nil)
		if err != nil {
			return err
		}
		if info.Status == enumspb.VERSION_DRAINAGE_STATUS_DRAINED {
			return nil
		}
		refresh, err := getSafeDurationConfig(ctx, "getDrainageRefreshInterval", unsafeRefreshIntervalGetter, defaultVisibilityRefresh)
		if err != nil {
			return err
		}
		_ = workflow.Sleep(ctx, refresh)

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			args.IsCan = true
			return workflow.NewContinueAsNewError(ctx, WorkerDeploymentDrainageWorkflowType, args)
		}
	}
}
