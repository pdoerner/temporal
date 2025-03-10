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

package matching

import (
	"context"
	"math"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/server/api/matchingservicemock/v1"
	persistencespb "go.temporal.io/server/api/persistence/v1"
	"go.temporal.io/server/common"
	"go.temporal.io/server/common/cluster"
	"go.temporal.io/server/common/dynamicconfig"
	"go.temporal.io/server/common/metrics/metricstest"
	"go.temporal.io/server/common/namespace"
	"go.temporal.io/server/common/persistence"
	"go.temporal.io/server/common/primitives/timestamp"
	"go.temporal.io/server/common/quotas"
	"go.temporal.io/server/common/tqid"
	"go.temporal.io/server/internal/goro"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var rpsInf = math.Inf(1)

const (
	defaultNamespaceId = "deadbeef-0000-4567-890a-bcdef0123456"
	defaultRootTqID    = "tq"
)

type tqmTestOpts struct {
	config              *Config
	dbq                 *PhysicalTaskQueueKey
	matchingClientMock  *matchingservicemock.MockMatchingServiceClient
	expectUserDataError bool
}

func defaultTqmTestOpts(controller *gomock.Controller) *tqmTestOpts {
	return &tqmTestOpts{
		config:             defaultTestConfig(),
		dbq:                defaultTqId(),
		matchingClientMock: matchingservicemock.NewMockMatchingServiceClient(controller),
	}
}

type testIDBlockAlloc struct {
	rid   int64
	alloc func() (taskQueueState, error)
}

func (a *testIDBlockAlloc) RangeID() int64 {
	return a.rid
}

func (a *testIDBlockAlloc) RenewLease(_ context.Context) (taskQueueState, error) {
	s, err := a.alloc()
	if err == nil {
		a.rid = s.rangeID
	}
	return s, err
}

func makeTestBlocAlloc(f func() (taskQueueState, error)) taskQueueManagerOpt {
	return withIDBlockAllocator(&testIDBlockAlloc{alloc: f})
}

func withIDBlockAllocator(ibl idBlockAllocator) taskQueueManagerOpt {
	return func(tqm *physicalTaskQueueManagerImpl) {
		tqm.backlogMgr.taskWriter.idAlloc = ibl
	}
}

func TestForeignPartitionOwnerCausesUnload(t *testing.T) {
	cfg := NewConfig(dynamicconfig.NewNoopCollection())
	cfg.RangeSize = 1 // TaskID block size
	var leaseErr error
	tqm := mustCreateTestPhysicalTaskQueueManager(t, gomock.NewController(t),
		makeTestBlocAlloc(func() (taskQueueState, error) {
			return taskQueueState{rangeID: 1}, leaseErr
		}))
	tqm.Start()
	defer tqm.Stop(unloadCauseUnspecified)

	// TQM started succesfully with an ID block of size 1. Perform one send
	// without a poller to consume the one task ID from the reserved block.
	err := tqm.SpoolTask(&persistencespb.TaskInfo{
		CreateTime: timestamp.TimePtr(time.Now().UTC()),
	})
	require.NoError(t, err)

	// TQM's ID block should be empty so the next AddTask will trigger an
	// attempt to obtain more IDs. This specific error type indicates that
	// another service instance has become the owner of the partition
	leaseErr = &persistence.ConditionFailedError{Msg: "should kill the tqm"}

	err = tqm.SpoolTask(&persistencespb.TaskInfo{
		CreateTime: timestamp.TimePtr(time.Now().UTC()),
	})
	require.NoError(t, err)
}

/*
TODO: rewrite or delete this test
func TestReaderSignaling(t *testing.T) {
	readerNotifications := make(chan struct{}, 1)
	clearNotifications := func() {
		for len(readerNotifications) > 0 {
			<-readerNotifications
		}
	}
	tqm := mustCreateTestPhysicalTaskQueueManager(t, gomock.NewController(t))

	// redirect taskReader signals into our local channel
	tqm.backlogMgr.taskReader.notifyC = readerNotifications

	tqm.Start()
	defer tqm.Stop(unloadCauseUnspecified)

	// shut down the taskReader so it doesn't steal notifications from us
	tqm.backlogMgr.taskReader.gorogrp.Cancel()
	tqm.backlogMgr.taskReader.gorogrp.Wait()

	clearNotifications()

	err := tqm.SpoolTask(&persistencespb.TaskInfo{
		CreateTime: timestamp.TimePtr(time.Now().UTC()),
	})
	require.NoError(t, err)
	require.Len(t, readerNotifications, 1,
		"Spool task should signal taskReader")

	clearNotifications()
	poller, _ := runOneShotPoller(context.Background(), tqm)
	defer poller.Cancel()

	task := newInternalTaskForSyncMatch(&persistencespb.TaskInfo{
		CreateTime: timestamp.TimePtr(time.Now().UTC()),
	}, nil)
	sync, err := tqm.TrySyncMatch(context.TODO(), task)
	require.NoError(t, err)
	require.True(t, sync)
	require.Len(t, readerNotifications, 0,
		"Sync match should not signal taskReader")
}
*/

func makePollMetadata(rps float64) *pollMetadata {
	return &pollMetadata{taskQueueMetadata: &taskqueuepb.TaskQueueMetadata{
		MaxTasksPerSecond: &wrapperspb.DoubleValue{
			Value: rps,
		},
	}}
}

// runOneShotPoller spawns a goroutine to call tqm.PollTask on the provided tqm.
// The second return value is a channel of either error or *internalTask.
func runOneShotPoller(ctx context.Context, tqm physicalTaskQueueManager) (*goro.Handle, chan interface{}) {
	out := make(chan interface{}, 1)
	handle := goro.NewHandle(ctx).Go(func(ctx context.Context) error {
		task, err := tqm.PollTask(ctx, makePollMetadata(rpsInf))
		if task == nil {
			out <- err
			return nil
		}
		task.finish(err, true)
		out <- task
		return nil
	})
	// tqm.PollTask() needs some time to attach the goro started above to the
	// internal task channel. Sorry for this but it appears unavoidable.
	time.Sleep(10 * time.Millisecond)
	return handle, out
}

func defaultTqId() *PhysicalTaskQueueKey {
	return newTestUnversionedPhysicalQueueKey(defaultNamespaceId, defaultRootTqID, enumspb.TASK_QUEUE_TYPE_WORKFLOW, 0)
}

func mustCreateTestPhysicalTaskQueueManager(
	t *testing.T,
	controller *gomock.Controller,
	opts ...taskQueueManagerOpt,
) *physicalTaskQueueManagerImpl {
	t.Helper()
	return mustCreateTestTaskQueueManagerWithConfig(t, controller, defaultTqmTestOpts(controller), opts...)
}

func mustCreateTestTaskQueueManagerWithConfig(
	t *testing.T,
	controller *gomock.Controller,
	testOpts *tqmTestOpts,
	opts ...taskQueueManagerOpt,
) *physicalTaskQueueManagerImpl {
	t.Helper()
	tqm, err := createTestTaskQueueManagerWithConfig(t, controller, testOpts, opts...)
	require.NoError(t, err)
	return tqm
}

func createTestTaskQueueManagerWithConfig(
	t *testing.T,
	controller *gomock.Controller,
	testOpts *tqmTestOpts,
	opts ...taskQueueManagerOpt,
) (*physicalTaskQueueManagerImpl, error) {
	nsName := namespace.Name("ns-name")
	ns, registry := createMockNamespaceCache(controller, nsName)
	me := createTestMatchingEngine(controller, testOpts.config, testOpts.matchingClientMock, registry)
	me.metricsHandler = metricstest.NewCaptureHandler()
	partition := testOpts.dbq.Partition()
	tqConfig := newTaskQueueConfig(partition.TaskQueue(), me.config, nsName)
	onFatalErr := func(unloadCause) { t.Fatal("user data manager called onFatalErr") }
	userDataManager := newUserDataManager(me.taskManager, me.matchingRawClient, onFatalErr, partition, tqConfig, me.logger, me.namespaceRegistry)
	pm := createTestTaskQueuePartitionManager(ns, partition, tqConfig, me, userDataManager)
	tlMgr, err := newPhysicalTaskQueueManager(pm, testOpts.dbq, opts...)
	pm.defaultQueue = tlMgr
	if err != nil {
		return nil, err
	}
	return tlMgr, nil
}

func createTestTaskQueuePartitionManager(ns *namespace.Namespace, partition tqid.Partition, tqConfig *taskQueueConfig, me *matchingEngineImpl, userDataManager userDataManager) *taskQueuePartitionManagerImpl {
	pm := &taskQueuePartitionManagerImpl{
		engine:          me,
		partition:       partition,
		config:          tqConfig,
		ns:              ns,
		logger:          me.logger,
		matchingClient:  me.matchingRawClient,
		metricsHandler:  me.metricsHandler,
		userDataManager: userDataManager,
	}

	me.partitions[partition.Key()] = pm
	return pm
}

func TestReaderBacklogAge(t *testing.T) {
	controller := gomock.NewController(t)

	// Create queue Manager and set queue state
	tlm := mustCreateTestPhysicalTaskQueueManager(t, controller)
	tlm.backlogMgr.db.rangeID = int64(1)
	tlm.backlogMgr.db.ackLevel = int64(0)
	tlm.backlogMgr.taskAckManager.setAckLevel(tlm.backlogMgr.db.ackLevel)

	tlm.backlogMgr.taskReader.taskBuffer <- randomTaskInfoWithAgeTaskID(time.Minute, 1)
	tlm.backlogMgr.taskReader.taskBuffer <- randomTaskInfoWithAgeTaskID(10*time.Second, 2)
	go tlm.backlogMgr.taskReader.dispatchBufferedTasks()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.InDelta(t, time.Minute, tlm.backlogMgr.taskReader.getBacklogHeadAge(), float64(time.Second))
	}, time.Second, 10*time.Millisecond)

	_, err := tlm.backlogMgr.pqMgr.PollTask(context.Background(), makePollMetadata(rpsInf))
	require.NoError(t, err)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.InDelta(t, 10*time.Second, tlm.backlogMgr.taskReader.getBacklogHeadAge(), float64(500*time.Millisecond))
	}, time.Second, 10*time.Millisecond)

	_, err = tlm.backlogMgr.pqMgr.PollTask(context.Background(), makePollMetadata(rpsInf))
	require.NoError(t, err)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.Equalf(t, time.Duration(0), tlm.backlogMgr.taskReader.getBacklogHeadAge(), "backlog age being reset because of no tasks in the buffer")
	}, time.Second, 10*time.Millisecond)
}

func randomTaskInfoWithAgeTaskID(age time.Duration, TaskID int64) *persistencespb.AllocatedTaskInfo {
	rt1 := time.Now().Add(-age)
	rt2 := rt1.Add(time.Hour)

	return &persistencespb.AllocatedTaskInfo{
		Data: &persistencespb.TaskInfo{
			NamespaceId:      uuid.New(),
			WorkflowId:       uuid.New(),
			RunId:            uuid.New(),
			ScheduledEventId: rand.Int63(),
			CreateTime:       timestamppb.New(rt1),
			ExpiryTime:       timestamppb.New(rt2),
		},
		TaskId: TaskID,
	}
}

func TestLegacyDescribeTaskQueue(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	startTaskID := int64(1)
	taskCount := int64(3)
	PollerIdentity := "test-poll"

	// Create queue Manager and set queue state
	tlm := mustCreateTestPhysicalTaskQueueManager(t, controller)
	tlm.backlogMgr.db.rangeID = int64(1)
	tlm.backlogMgr.db.ackLevel = int64(0)
	tlm.backlogMgr.taskAckManager.setAckLevel(tlm.backlogMgr.db.ackLevel)

	for i := int64(0); i < taskCount; i++ {
		tlm.backlogMgr.taskAckManager.addTask(startTaskID + i)
	}

	// Manually increase the backlog counter since it does not get incremented by taskAckManager.addTask
	// Only doing this for the purpose of this test
	tlm.backlogMgr.db.updateApproximateBacklogCount(taskCount)

	includeTaskStatus := false
	descResp := tlm.LegacyDescribeTaskQueue(includeTaskStatus)
	require.Equal(t, 0, len(descResp.DescResponse.GetPollers()))
	require.Nil(t, descResp.DescResponse.GetTaskQueueStatus())

	includeTaskStatus = true
	taskQueueStatus := tlm.LegacyDescribeTaskQueue(includeTaskStatus).DescResponse.GetTaskQueueStatus()
	require.NotNil(t, taskQueueStatus)
	require.Zero(t, taskQueueStatus.GetAckLevel())
	require.Equal(t, taskCount, taskQueueStatus.GetReadLevel())
	require.Equal(t, taskCount, taskQueueStatus.GetBacklogCountHint())
	taskIDBlock := taskQueueStatus.GetTaskIdBlock()
	require.Equal(t, int64(1), taskIDBlock.GetStartId())
	require.Equal(t, tlm.config.RangeSize, taskIDBlock.GetEndId())

	// Add a poller and complete all tasks
	tlm.pollerHistory.updatePollerInfo(pollerIdentity(PollerIdentity), &pollMetadata{})
	for i := int64(0); i < taskCount; i++ {
		tlm.backlogMgr.taskAckManager.completeTask(startTaskID + i)
	}

	descResp = tlm.LegacyDescribeTaskQueue(includeTaskStatus)
	require.Equal(t, 1, len(descResp.DescResponse.GetPollers()))
	require.Equal(t, PollerIdentity, descResp.DescResponse.Pollers[0].GetIdentity())
	require.NotEmpty(t, descResp.DescResponse.Pollers[0].GetLastAccessTime())

	rps := 5.0
	tlm.pollerHistory.updatePollerInfo(pollerIdentity(PollerIdentity), makePollMetadata(rps))
	descResp = tlm.LegacyDescribeTaskQueue(includeTaskStatus)
	require.Equal(t, 1, len(descResp.DescResponse.GetPollers()))
	require.Equal(t, PollerIdentity, descResp.DescResponse.Pollers[0].GetIdentity())
	require.True(t, descResp.DescResponse.Pollers[0].GetRatePerSecond() > 4.0 && descResp.DescResponse.Pollers[0].GetRatePerSecond() < 6.0)

	taskQueueStatus = descResp.DescResponse.GetTaskQueueStatus()
	require.NotNil(t, taskQueueStatus)
	require.Equal(t, taskCount, taskQueueStatus.GetAckLevel())
	require.Zero(t, taskQueueStatus.GetBacklogCountHint()) // should be 0 since AckManager.CompleteTask decrements the updated backlog counter
}

func TestCheckIdleTaskQueue(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	cfg := NewConfig(dynamicconfig.NewNoopCollection())
	cfg.MaxTaskQueueIdleTime = dynamicconfig.GetDurationPropertyFnFilteredByTaskQueue(2 * time.Second)
	tqCfg := defaultTqmTestOpts(controller)
	tqCfg.config = cfg

	// Idle
	tlm := mustCreateTestTaskQueueManagerWithConfig(t, controller, tqCfg)
	tlm.Start()
	time.Sleep(1 * time.Second)
	require.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&tlm.status))

	// Active poll-er
	tlm = mustCreateTestTaskQueueManagerWithConfig(t, controller, tqCfg)
	tlm.Start()
	tlm.pollerHistory.updatePollerInfo("test-poll", &pollMetadata{})
	require.Equal(t, 1, len(tlm.GetAllPollerInfo()))
	time.Sleep(1 * time.Second)
	require.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&tlm.status))
	tlm.Stop(unloadCauseUnspecified)
	require.Equal(t, common.DaemonStatusStopped, atomic.LoadInt32(&tlm.status))

	// Active adding task
	tlm = mustCreateTestTaskQueueManagerWithConfig(t, controller, tqCfg)
	tlm.Start()
	require.Equal(t, 0, len(tlm.GetAllPollerInfo()))
	tlm.backlogMgr.taskReader.Signal()
	time.Sleep(1 * time.Second)
	require.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&tlm.status))
	tlm.Stop(unloadCauseUnspecified)
	require.Equal(t, common.DaemonStatusStopped, atomic.LoadInt32(&tlm.status))
}

func TestAddTaskStandby(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tlm := mustCreateTestTaskQueueManagerWithConfig(
		t,
		controller,
		defaultTqmTestOpts(controller),
		func(tqm *physicalTaskQueueManagerImpl) {
			ns := namespace.NewGlobalNamespaceForTest(
				&persistencespb.NamespaceInfo{},
				&persistencespb.NamespaceConfig{},
				&persistencespb.NamespaceReplicationConfig{
					ActiveClusterName: cluster.TestAlternativeClusterName,
				},
				cluster.TestAlternativeClusterInitialFailoverVersion,
			)

			// we need to override the mockNamespaceCache to return a passive namespace
			mockNamespaceCache := namespace.NewMockRegistry(controller)
			mockNamespaceCache.EXPECT().GetNamespaceByID(gomock.Any()).Return(ns, nil).AnyTimes()
			mockNamespaceCache.EXPECT().GetNamespaceName(gomock.Any()).Return(ns.Name(), nil).AnyTimes()
			tqm.namespaceRegistry = mockNamespaceCache
		},
	)
	tlm.Start()
	// stop taskWriter so that we can check if there's any call to it
	// otherwise the task persist process is async and hard to test
	tlm.tqCtxCancel()

	err := tlm.SpoolTask(&persistencespb.TaskInfo{
		CreateTime: timestamp.TimePtr(time.Now().UTC()),
	})
	require.Equal(t, errShutdown, err) // task writer was stopped above
}

func TestTQMDoesFinalUpdateOnIdleUnload(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)

	cfg := NewConfig(dynamicconfig.NewNoopCollection())
	cfg.MaxTaskQueueIdleTime = dynamicconfig.GetDurationPropertyFnFilteredByTaskQueue(1 * time.Second)
	tqCfg := defaultTqmTestOpts(controller)
	tqCfg.config = cfg

	tqm := mustCreateTestTaskQueueManagerWithConfig(t, controller, tqCfg)
	tm, ok := tqm.partitionMgr.engine.taskManager.(*testTaskManager)
	require.True(t, ok)

	tqm.Start()
	time.Sleep(2 * time.Second) // will unload due to idleness
	require.Equal(t, 1, tm.getUpdateCount(tqCfg.dbq))
}

func TestTQMDoesNotDoFinalUpdateOnOwnershipLost(t *testing.T) {
	// TODO: use mocks instead of testTaskManager so we can do synchronization better instead of sleeps
	t.Parallel()

	controller := gomock.NewController(t)

	cfg := NewConfig(dynamicconfig.NewNoopCollection())
	cfg.UpdateAckInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskQueue(2 * time.Second)
	tqCfg := defaultTqmTestOpts(controller)
	tqCfg.config = cfg

	tqm := mustCreateTestTaskQueueManagerWithConfig(t, controller, tqCfg)
	tm, ok := tqm.partitionMgr.engine.taskManager.(*testTaskManager)
	require.True(t, ok)

	tqm.Start()
	time.Sleep(1 * time.Second)

	// simulate ownership lost
	ttm := tm.getQueueManagerByKey(tqCfg.dbq)
	ttm.Lock()
	ttm.rangeID++
	ttm.Unlock()

	time.Sleep(2 * time.Second) // will attempt to update and fail and not try again

	require.Equal(t, 1, tm.getUpdateCount(tqCfg.dbq))
}

func TestTQMInterruptsPollOnClose(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)
	tqm.Start()

	pollStart := time.Now()
	pollCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, pollCh := runOneShotPoller(pollCtx, tqm)

	tqm.Stop(unloadCauseUnspecified) // should interrupt poller

	<-pollCh
	require.Less(t, time.Since(pollStart), 4*time.Second)
}

func TestPollScalingUpOnBacklog(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	rl := quotas.NewMockRateLimiter(controller)
	rl.EXPECT().AllowN(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	tqm.pollerScalingRateLimiter = rl

	fakeStats := &taskqueuepb.TaskQueueStats{
		ApproximateBacklogCount: 100,
		ApproximateBacklogAge:   durationpb.New(1 * time.Minute),
	}

	decision := tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.NotNil(t, decision)
	require.GreaterOrEqual(t, decision.PollRequestDeltaSuggestion, int32(1))
}

func TestPollScalingUpAddRateExceedsDispatchRate(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	rl := quotas.NewMockRateLimiter(controller)
	rl.EXPECT().AllowN(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	tqm.pollerScalingRateLimiter = rl

	fakeStats := &taskqueuepb.TaskQueueStats{
		TasksAddRate:      100,
		TasksDispatchRate: 10,
	}

	decision := tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.NotNil(t, decision)
	require.GreaterOrEqual(t, decision.PollRequestDeltaSuggestion, int32(1))
}

func TestPollScalingNoChangeOnNoBacklogFastMatch(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	fakeStats := &taskqueuepb.TaskQueueStats{
		ApproximateBacklogCount: 0,
		ApproximateBacklogAge:   durationpb.New(0),
	}
	decision := tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.Nil(t, decision)
}

func TestPollScalingNonRootPartition(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	// Non-root partitions only get to emit decisions on high backlog
	f, err := tqid.NewTaskQueueFamily(namespaceId, taskQueueName)
	require.NoError(t, err)
	partition := f.TaskQueue(enumspb.TASK_QUEUE_TYPE_WORKFLOW).NormalPartition(1)
	tqm.partitionMgr.partition = partition
	// Also disable rate limit to ensure that's not why nil is returned here
	rl := quotas.NewMockRateLimiter(controller)
	rl.EXPECT().AllowN(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	tqm.pollerScalingRateLimiter = rl

	fakeStats := &taskqueuepb.TaskQueueStats{
		ApproximateBacklogCount: 100,
		ApproximateBacklogAge:   durationpb.New(1 * time.Minute),
	}
	decision := tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.NotNil(t, decision)
	require.GreaterOrEqual(t, decision.PollRequestDeltaSuggestion, int32(1))

	fakeStats.ApproximateBacklogCount = 0
	decision = tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.Nil(t, decision)
}

func TestPollScalingDownOnLongSyncMatch(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	fakeStats := &taskqueuepb.TaskQueueStats{
		ApproximateBacklogCount: 0,
	}
	decision := tqm.makePollerScalingDecisionImpl(time.Now().Add(-2*time.Second), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.LessOrEqual(t, decision.PollRequestDeltaSuggestion, int32(-1))
}

func TestPollScalingDecisionsAreRateLimited(t *testing.T) {
	controller := gomock.NewController(t)
	tqm := mustCreateTestPhysicalTaskQueueManager(t, controller)

	rl := quotas.NewMockRateLimiter(controller)
	rl.EXPECT().AllowN(gomock.Any(), gomock.Any()).Return(true).Times(1)
	rl.EXPECT().AllowN(gomock.Any(), gomock.Any()).Return(false).Times(1)
	tqm.pollerScalingRateLimiter = rl

	fakeStats := &taskqueuepb.TaskQueueStats{
		ApproximateBacklogCount: 100,
		ApproximateBacklogAge:   durationpb.New(1 * time.Minute),
	}
	decision := tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.GreaterOrEqual(t, decision.PollRequestDeltaSuggestion, int32(1))

	decision = tqm.makePollerScalingDecisionImpl(time.Now(), func() *taskqueuepb.TaskQueueStats { return fakeStats })
	require.Nil(t, decision)
}
