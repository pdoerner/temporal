package persistence

import (
	"context"
	"errors"
	"strings"

	commonpb "go.temporal.io/api/common/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	historyspb "go.temporal.io/server/api/history/v1"
	persistencespb "go.temporal.io/server/api/persistence/v1"
	"go.temporal.io/server/common"
	"go.temporal.io/server/common/definition"
	"go.temporal.io/server/common/dynamicconfig"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"
	"go.temporal.io/server/common/persistence/serialization"
	"go.temporal.io/server/common/persistence/transitionhistory"
	"go.temporal.io/server/common/persistence/versionhistory"
	"go.temporal.io/server/service/history/tasks"
)

type (
	// executionManagerImpl implements ExecutionManager based on ExecutionStore, statsComputer and Serializer
	executionManagerImpl struct {
		serializer            serialization.Serializer
		eventBlobCache        XDCCache
		persistence           ExecutionStore
		logger                log.Logger
		pagingTokenSerializer *jsonHistoryTokenSerializer
		transactionSizeLimit  dynamicconfig.IntPropertyFn
	}
)

var _ ExecutionManager = (*executionManagerImpl)(nil)

// NewExecutionManager returns new ExecutionManager
func NewExecutionManager(
	persistence ExecutionStore,
	serializer serialization.Serializer,
	eventBlobCache XDCCache,
	logger log.Logger,
	transactionSizeLimit dynamicconfig.IntPropertyFn,
) ExecutionManager {
	return &executionManagerImpl{
		serializer:            serializer,
		eventBlobCache:        eventBlobCache,
		persistence:           persistence,
		logger:                logger,
		pagingTokenSerializer: newJSONHistoryTokenSerializer(),
		transactionSizeLimit:  transactionSizeLimit,
	}
}

func (m *executionManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *executionManagerImpl) GetHistoryBranchUtil() HistoryBranchUtil {
	return m.persistence.GetHistoryBranchUtil()
}

// The below three APIs are related to serialization/deserialization

func (m *executionManagerImpl) CreateWorkflowExecution(
	ctx context.Context,
	request *CreateWorkflowExecutionRequest,
) (*CreateWorkflowExecutionResponse, error) {

	newSnapshot := request.NewWorkflowSnapshot
	newWorkflowXDCKVs, newWorkflowNewEvents, newHistoryDiff, err := m.serializeWorkflowEventBatches(
		ctx,
		request.ShardID,
		request.NewWorkflowSnapshot.ExecutionInfo,
		request.NewWorkflowEvents,
	)
	if err != nil {
		return nil, err
	}

	newSnapshot.ExecutionInfo.ExecutionStats.HistorySize += int64(newHistoryDiff.SizeDiff)

	if err := ValidateCreateWorkflowModeState(
		request.Mode,
		newSnapshot,
	); err != nil {
		return nil, err
	}
	if err := ValidateCreateWorkflowStateStatus(
		newSnapshot.ExecutionState.State,
		newSnapshot.ExecutionState.Status,
	); err != nil {
		return nil, err
	}

	serializedNewWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&newSnapshot)
	if err != nil {
		return nil, err
	}

	newRequest := &InternalCreateWorkflowExecutionRequest{
		ShardID:                  request.ShardID,
		RangeID:                  request.RangeID,
		Mode:                     request.Mode,
		PreviousRunID:            request.PreviousRunID,
		PreviousLastWriteVersion: request.PreviousLastWriteVersion,
		NewWorkflowSnapshot:      *serializedNewWorkflowSnapshot,
		NewWorkflowNewEvents:     newWorkflowNewEvents,
	}

	if _, err := m.persistence.CreateWorkflowExecution(ctx, newRequest); err != nil {
		return nil, err
	}
	m.addXDCCacheKV(newWorkflowXDCKVs)
	return &CreateWorkflowExecutionResponse{
		NewMutableStateStats: *statusOfInternalWorkflowSnapshot(
			serializedNewWorkflowSnapshot,
			newHistoryDiff,
		),
	}, nil
}

func (m *executionManagerImpl) UpdateWorkflowExecution(
	ctx context.Context,
	request *UpdateWorkflowExecutionRequest,
) (*UpdateWorkflowExecutionResponse, error) {

	updateMutation := request.UpdateWorkflowMutation
	newSnapshot := request.NewWorkflowSnapshot

	updateWorkflowXDCKVs, updateWorkflowNewEvents, updateWorkflowHistoryDiff, err := m.serializeWorkflowEventBatches(
		ctx,
		request.ShardID,
		request.UpdateWorkflowMutation.ExecutionInfo,
		request.UpdateWorkflowEvents,
	)
	if err != nil {
		return nil, err
	}
	updateMutation.ExecutionInfo.ExecutionStats.HistorySize += int64(updateWorkflowHistoryDiff.SizeDiff)

	var newWorkflowXDCKVs map[XDCCacheKey]XDCCacheValue
	var newWorkflowNewEvents []*InternalAppendHistoryNodesRequest
	var newWorkflowHistoryDiff *HistoryStatistics
	if newSnapshot != nil {
		newWorkflowXDCKVs, newWorkflowNewEvents, newWorkflowHistoryDiff, err = m.serializeWorkflowEventBatches(
			ctx,
			request.ShardID,
			request.NewWorkflowSnapshot.ExecutionInfo,
			request.NewWorkflowEvents,
		)
		if err != nil {
			return nil, err
		}
		newSnapshot.ExecutionInfo.ExecutionStats.HistorySize += int64(newWorkflowHistoryDiff.SizeDiff)
	}

	if err := ValidateUpdateWorkflowModeState(
		request.Mode,
		updateMutation,
		newSnapshot,
	); err != nil {
		return nil, err
	}
	if err := ValidateUpdateWorkflowStateStatus(
		updateMutation.ExecutionState.State,
		updateMutation.ExecutionState.Status,
	); err != nil {
		return nil, err
	}

	serializedWorkflowMutation, err := m.SerializeWorkflowMutation(&updateMutation)
	if err != nil {
		return nil, err
	}
	var serializedNewWorkflowSnapshot *InternalWorkflowSnapshot
	if newSnapshot != nil {
		serializedNewWorkflowSnapshot, err = m.SerializeWorkflowSnapshot(newSnapshot)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalUpdateWorkflowExecutionRequest{
		ShardID: request.ShardID,
		RangeID: request.RangeID,

		Mode: request.Mode,

		UpdateWorkflowMutation:  *serializedWorkflowMutation,
		UpdateWorkflowNewEvents: updateWorkflowNewEvents,
		NewWorkflowSnapshot:     serializedNewWorkflowSnapshot,
		NewWorkflowNewEvents:    newWorkflowNewEvents,
	}

	err = m.persistence.UpdateWorkflowExecution(ctx, newRequest)
	switch err.(type) {
	case nil:
		m.addXDCCacheKV(updateWorkflowXDCKVs)
		m.addXDCCacheKV(newWorkflowXDCKVs)
		return &UpdateWorkflowExecutionResponse{
			UpdateMutableStateStats: *statusOfInternalWorkflowMutation(
				&newRequest.UpdateWorkflowMutation,
				updateWorkflowHistoryDiff,
			),
			NewMutableStateStats: statusOfInternalWorkflowSnapshot(
				newRequest.NewWorkflowSnapshot,
				newWorkflowHistoryDiff,
			),
		}, nil
	case *CurrentWorkflowConditionFailedError,
		*WorkflowConditionFailedError,
		*ConditionFailedError:
		m.trimHistoryNode(
			ctx,
			request.ShardID,
			updateMutation.ExecutionInfo.NamespaceId,
			updateMutation.ExecutionInfo.WorkflowId,
			updateMutation.ExecutionState.RunId,
		)
		return nil, err
	default:
		return nil, err
	}
}

func (m *executionManagerImpl) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *ConflictResolveWorkflowExecutionRequest,
) (*ConflictResolveWorkflowExecutionResponse, error) {

	resetSnapshot := request.ResetWorkflowSnapshot
	newSnapshot := request.NewWorkflowSnapshot
	currentMutation := request.CurrentWorkflowMutation

	resetWorkflowXDCKVs, resetWorkflowEvents, resetWorkflowHistoryDiff, err := m.serializeWorkflowEventBatches(
		ctx,
		request.ShardID,
		request.ResetWorkflowSnapshot.ExecutionInfo,
		request.ResetWorkflowEvents,
	)
	if err != nil {
		return nil, err
	}
	resetSnapshot.ExecutionInfo.ExecutionStats.HistorySize += int64(resetWorkflowHistoryDiff.SizeDiff)

	var newWorkflowXDCKVs map[XDCCacheKey]XDCCacheValue
	var newWorkflowEvents []*InternalAppendHistoryNodesRequest
	var newWorkflowHistoryDiff *HistoryStatistics
	if newSnapshot != nil {
		newWorkflowXDCKVs, newWorkflowEvents, newWorkflowHistoryDiff, err = m.serializeWorkflowEventBatches(
			ctx,
			request.ShardID,
			request.NewWorkflowSnapshot.ExecutionInfo,
			request.NewWorkflowEvents,
		)
		if err != nil {
			return nil, err
		}
		newSnapshot.ExecutionInfo.ExecutionStats.HistorySize += int64(newWorkflowHistoryDiff.SizeDiff)
	}

	var currentWorkflowXDCKVs map[XDCCacheKey]XDCCacheValue
	var currentWorkflowEvents []*InternalAppendHistoryNodesRequest
	var currentWorkflowHistoryDiff *HistoryStatistics
	if currentMutation != nil {
		currentWorkflowXDCKVs, currentWorkflowEvents, currentWorkflowHistoryDiff, err = m.serializeWorkflowEventBatches(
			ctx,
			request.ShardID,
			request.CurrentWorkflowMutation.ExecutionInfo,
			request.CurrentWorkflowEvents,
		)
		if err != nil {
			return nil, err
		}
		currentMutation.ExecutionInfo.ExecutionStats.HistorySize += int64(currentWorkflowHistoryDiff.SizeDiff)
	}

	if err := ValidateConflictResolveWorkflowModeState(
		request.Mode,
		resetSnapshot,
		newSnapshot,
		currentMutation,
	); err != nil {
		return nil, err
	}

	serializedResetWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&resetSnapshot)
	if err != nil {
		return nil, err
	}
	var serializedCurrentWorkflowMutation *InternalWorkflowMutation
	if currentMutation != nil {
		serializedCurrentWorkflowMutation, err = m.SerializeWorkflowMutation(currentMutation)
		if err != nil {
			return nil, err
		}
	}
	var serializedNewWorkflowMutation *InternalWorkflowSnapshot
	if newSnapshot != nil {
		serializedNewWorkflowMutation, err = m.SerializeWorkflowSnapshot(newSnapshot)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalConflictResolveWorkflowExecutionRequest{
		ShardID: request.ShardID,
		RangeID: request.RangeID,

		Mode: request.Mode,

		ResetWorkflowSnapshot:        *serializedResetWorkflowSnapshot,
		ResetWorkflowEventsNewEvents: resetWorkflowEvents,

		NewWorkflowSnapshot:        serializedNewWorkflowMutation,
		NewWorkflowEventsNewEvents: newWorkflowEvents,

		CurrentWorkflowMutation:        serializedCurrentWorkflowMutation,
		CurrentWorkflowEventsNewEvents: currentWorkflowEvents,
	}

	err = m.persistence.ConflictResolveWorkflowExecution(ctx, newRequest)
	switch err.(type) {
	case nil:
		m.addXDCCacheKV(resetWorkflowXDCKVs)
		m.addXDCCacheKV(newWorkflowXDCKVs)
		m.addXDCCacheKV(currentWorkflowXDCKVs)
		return &ConflictResolveWorkflowExecutionResponse{
			ResetMutableStateStats: *statusOfInternalWorkflowSnapshot(
				&newRequest.ResetWorkflowSnapshot,
				resetWorkflowHistoryDiff,
			),
			NewMutableStateStats: statusOfInternalWorkflowSnapshot(
				newRequest.NewWorkflowSnapshot,
				newWorkflowHistoryDiff,
			),
			CurrentMutableStateStats: statusOfInternalWorkflowMutation(
				newRequest.CurrentWorkflowMutation,
				currentWorkflowHistoryDiff,
			),
		}, nil
	case *CurrentWorkflowConditionFailedError,
		*WorkflowConditionFailedError,
		*ConditionFailedError:
		m.trimHistoryNode(
			ctx,
			request.ShardID,
			resetSnapshot.ExecutionInfo.NamespaceId,
			resetSnapshot.ExecutionInfo.WorkflowId,
			resetSnapshot.ExecutionState.RunId,
		)
		if currentMutation != nil {
			m.trimHistoryNode(
				ctx,
				request.ShardID,
				currentMutation.ExecutionInfo.NamespaceId,
				currentMutation.ExecutionInfo.WorkflowId,
				currentMutation.ExecutionState.RunId,
			)
		}
		return nil, err
	default:
		return nil, err
	}
}

func (m *executionManagerImpl) GetWorkflowExecution(
	ctx context.Context,
	request *GetWorkflowExecutionRequest,
) (*GetWorkflowExecutionResponse, error) {
	response, respErr := m.persistence.GetWorkflowExecution(ctx, request)

	var notFound *serviceerror.NotFound
	if errors.As(respErr, &notFound) {
		// strip persistence-specific error message
		respErr = serviceerror.NewNotFoundf(
			"workflow execution not found for workflow ID %q and run ID %q", request.WorkflowID, request.RunID)
	}
	if respErr != nil && response == nil {
		// try to utilize resp as much as possible, for RebuildMutableState API
		return nil, respErr
	}
	state, err := m.toWorkflowMutableState(response.State)
	if err != nil {
		return nil, err
	}
	if state.ExecutionInfo.ExecutionStats == nil {
		state.ExecutionInfo.ExecutionStats = &persistencespb.ExecutionStats{
			HistorySize: 0,
		}
	}

	newResponse := &GetWorkflowExecutionResponse{
		State:             state,
		DBRecordVersion:   response.DBRecordVersion,
		MutableStateStats: *statusOfInternalWorkflow(response.State, state, nil),
	}
	return newResponse, respErr
}

func (m *executionManagerImpl) SetWorkflowExecution(
	ctx context.Context,
	request *SetWorkflowExecutionRequest,
) (*SetWorkflowExecutionResponse, error) {
	serializedWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&request.SetWorkflowSnapshot)
	if err != nil {
		return nil, err
	}

	newRequest := &InternalSetWorkflowExecutionRequest{
		ShardID: request.ShardID,
		RangeID: request.RangeID,

		SetWorkflowSnapshot: *serializedWorkflowSnapshot,
	}

	err = m.persistence.SetWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}
	return &SetWorkflowExecutionResponse{}, nil
}

func (m *executionManagerImpl) serializeWorkflowEventBatches(
	ctx context.Context,
	shardID int32,
	executionInfo *persistencespb.WorkflowExecutionInfo,
	eventBatches []*WorkflowEvents,
) (map[XDCCacheKey]XDCCacheValue, []*InternalAppendHistoryNodesRequest, *HistoryStatistics, error) {
	var historyStatistics HistoryStatistics
	if len(eventBatches) == 0 {
		return nil, nil, &historyStatistics, nil
	}

	xdcKVs := make(map[XDCCacheKey]XDCCacheValue, len(eventBatches))
	workflowNewEvents := make([]*InternalAppendHistoryNodesRequest, 0, len(eventBatches))
	for _, workflowEvents := range eventBatches {
		newEvents, err := m.serializeWorkflowEvents(shardID, workflowEvents)
		if err != nil {
			return nil, nil, nil, err
		}
		versionHistoryItems, _, baseWorkflowInfo, err := GetXDCCacheValue(
			executionInfo,
			workflowEvents.Events[0].EventId,
			workflowEvents.Events[0].Version,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		xdcKVs[NewXDCCacheKey(
			definition.NewWorkflowKey(workflowEvents.NamespaceID, workflowEvents.WorkflowID, workflowEvents.RunID),
			workflowEvents.Events[0].EventId,
			workflowEvents.Events[0].Version,
		)] = NewXDCCacheValue(
			baseWorkflowInfo,
			versionHistoryItems,
			[]*commonpb.DataBlob{newEvents.Node.Events},
			workflowEvents.Events[len(workflowEvents.Events)-1].EventId+1,
		)
		newEvents.ShardID = shardID
		workflowNewEvents = append(workflowNewEvents, newEvents)
		historyStatistics.SizeDiff += len(newEvents.Node.Events.Data)
		historyStatistics.CountDiff += len(workflowEvents.Events)
	}
	return xdcKVs, workflowNewEvents, &historyStatistics, nil
}

func (m *executionManagerImpl) addXDCCacheKV(
	xdcKVs map[XDCCacheKey]XDCCacheValue,
) {
	if m.eventBlobCache == nil {
		return
	}
	for k, v := range xdcKVs {
		m.eventBlobCache.Put(k, v)
	}
}

func (m *executionManagerImpl) DeserializeBufferedEvents( // unexport
	blobs []*commonpb.DataBlob,
) ([]*historypb.HistoryEvent, error) {

	events := make([]*historypb.HistoryEvent, 0)
	for _, b := range blobs {
		if b == nil {
			// Should not happen, log and discard to prevent callers from consuming
			m.logger.Warn("discarding nil buffered event")
			continue
		}

		history, err := m.serializer.DeserializeEvents(b)
		if err != nil {
			return nil, err
		}
		events = append(events, history...)
	}
	return events, nil
}

func (m *executionManagerImpl) serializeWorkflowEvents(
	shardID int32,
	workflowEvents *WorkflowEvents,
) (*InternalAppendHistoryNodesRequest, error) {
	if len(workflowEvents.Events) == 0 {
		return nil, nil // allow update workflow without events
	}

	request := &AppendHistoryNodesRequest{
		ShardID:           shardID,
		BranchToken:       workflowEvents.BranchToken,
		Events:            workflowEvents.Events,
		PrevTransactionID: workflowEvents.PrevTxnID,
		TransactionID:     workflowEvents.TxnID,
	}

	if workflowEvents.Events[0].EventId == common.FirstEventID {
		request.IsNewBranch = true
		request.Info = BuildHistoryGarbageCleanupInfo(workflowEvents.NamespaceID, workflowEvents.WorkflowID, workflowEvents.RunID)
	}

	return m.serializeAppendHistoryNodesRequest(request)
}

func (m *executionManagerImpl) SerializeWorkflowMutation( // unexport
	input *WorkflowMutation,
) (*InternalWorkflowMutation, error) {

	tasks, err := serializeTasks(m.serializer, input.Tasks)
	if err != nil {
		return nil, err
	}

	result := &InternalWorkflowMutation{
		NamespaceID: input.ExecutionInfo.GetNamespaceId(),
		WorkflowID:  input.ExecutionInfo.GetWorkflowId(),
		RunID:       input.ExecutionState.GetRunId(),

		UpsertActivityInfos: make(map[int64]*commonpb.DataBlob, len(input.UpsertActivityInfos)),
		DeleteActivityInfos: input.DeleteActivityInfos,

		UpsertTimerInfos: make(map[string]*commonpb.DataBlob, len(input.UpsertTimerInfos)),
		DeleteTimerInfos: input.DeleteTimerInfos,

		UpsertChildExecutionInfos: make(map[int64]*commonpb.DataBlob, len(input.UpsertChildExecutionInfos)),
		DeleteChildExecutionInfos: input.DeleteChildExecutionInfos,

		UpsertRequestCancelInfos: make(map[int64]*commonpb.DataBlob, len(input.UpsertRequestCancelInfos)),
		DeleteRequestCancelInfos: input.DeleteRequestCancelInfos,

		UpsertSignalInfos: make(map[int64]*commonpb.DataBlob, len(input.UpsertSignalInfos)),
		DeleteSignalInfos: input.DeleteSignalInfos,

		UpsertChasmNodes: make(map[string]InternalChasmNode, len(input.UpsertChasmNodes)),
		DeleteChasmNodes: input.DeleteChasmNodes,

		UpsertSignalRequestedIDs: input.UpsertSignalRequestedIDs,
		DeleteSignalRequestedIDs: input.DeleteSignalRequestedIDs,

		NewBufferedEvents:   nil,
		ClearBufferedEvents: input.ClearBufferedEvents,

		ExecutionInfo:  input.ExecutionInfo,
		ExecutionState: input.ExecutionState,

		Tasks: tasks,

		Condition:       input.Condition,
		DBRecordVersion: input.DBRecordVersion,
		NextEventID:     input.NextEventID,
	}

	result.ExecutionInfoBlob, err = m.serializer.WorkflowExecutionInfoToBlob(input.ExecutionInfo)
	if err != nil {
		return nil, err
	}
	result.ExecutionStateBlob, err = m.serializer.WorkflowExecutionStateToBlob(input.ExecutionState)
	if err != nil {
		return nil, err
	}

	for key, info := range input.UpsertActivityInfos {
		blob, err := m.serializer.ActivityInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.UpsertActivityInfos[key] = blob
	}

	for key, info := range input.UpsertTimerInfos {
		blob, err := m.serializer.TimerInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.UpsertTimerInfos[key] = blob
	}

	for key, info := range input.UpsertChildExecutionInfos {
		blob, err := m.serializer.ChildExecutionInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.UpsertChildExecutionInfos[key] = blob
	}

	for key, info := range input.UpsertRequestCancelInfos {
		blob, err := m.serializer.RequestCancelInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.UpsertRequestCancelInfos[key] = blob
	}

	for key, info := range input.UpsertSignalInfos {
		blob, err := m.serializer.SignalInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.UpsertSignalInfos[key] = blob
	}

	nodeMap, err := m.makeInternalChasmNodeMap(input.UpsertChasmNodes)
	if err != nil {
		return nil, err
	}
	result.UpsertChasmNodes = nodeMap

	if len(input.NewBufferedEvents) > 0 {
		result.NewBufferedEvents, err = m.serializer.SerializeEvents(input.NewBufferedEvents)
		if err != nil {
			return nil, err
		}
	}

	result.LastWriteVersion, err = getCurrentBranchLastWriteVersion(input.ExecutionInfo.VersionHistories, input.ExecutionInfo.TransitionHistory)
	if err != nil {
		return nil, err
	}
	result.Checksum, err = m.serializer.ChecksumToBlob(input.Checksum)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *executionManagerImpl) SerializeWorkflowSnapshot( // unexport
	input *WorkflowSnapshot,
) (*InternalWorkflowSnapshot, error) {

	tasks, err := serializeTasks(m.serializer, input.Tasks)
	if err != nil {
		return nil, err
	}

	result := &InternalWorkflowSnapshot{
		NamespaceID: input.ExecutionInfo.GetNamespaceId(),
		WorkflowID:  input.ExecutionInfo.GetWorkflowId(),
		RunID:       input.ExecutionState.GetRunId(),

		ActivityInfos:       make(map[int64]*commonpb.DataBlob, len(input.ActivityInfos)),
		TimerInfos:          make(map[string]*commonpb.DataBlob, len(input.TimerInfos)),
		ChildExecutionInfos: make(map[int64]*commonpb.DataBlob, len(input.ChildExecutionInfos)),
		RequestCancelInfos:  make(map[int64]*commonpb.DataBlob, len(input.RequestCancelInfos)),
		SignalInfos:         make(map[int64]*commonpb.DataBlob, len(input.SignalInfos)),
		ChasmNodes:          make(map[string]InternalChasmNode, len(input.ChasmNodes)),

		ExecutionInfo:      input.ExecutionInfo,
		ExecutionState:     input.ExecutionState,
		SignalRequestedIDs: make(map[string]struct{}),

		Tasks: tasks,

		Condition:       input.Condition,
		DBRecordVersion: input.DBRecordVersion,
		NextEventID:     input.NextEventID,
	}

	result.ExecutionInfoBlob, err = m.serializer.WorkflowExecutionInfoToBlob(input.ExecutionInfo)
	if err != nil {
		return nil, err
	}
	result.ExecutionStateBlob, err = m.serializer.WorkflowExecutionStateToBlob(input.ExecutionState)
	if err != nil {
		return nil, err
	}
	result.LastWriteVersion, err = getCurrentBranchLastWriteVersion(input.ExecutionInfo.VersionHistories, input.ExecutionInfo.TransitionHistory)
	if err != nil {
		return nil, err
	}

	for key, info := range input.ActivityInfos {
		blob, err := m.serializer.ActivityInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.ActivityInfos[key] = blob
	}
	for key, info := range input.TimerInfos {
		blob, err := m.serializer.TimerInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.TimerInfos[key] = blob
	}
	for key, info := range input.ChildExecutionInfos {
		blob, err := m.serializer.ChildExecutionInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.ChildExecutionInfos[key] = blob
	}
	for key, info := range input.RequestCancelInfos {
		blob, err := m.serializer.RequestCancelInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.RequestCancelInfos[key] = blob
	}
	for key, info := range input.SignalInfos {
		blob, err := m.serializer.SignalInfoToBlob(info)
		if err != nil {
			return nil, err
		}
		result.SignalInfos[key] = blob
	}
	for key := range input.SignalRequestedIDs {
		result.SignalRequestedIDs[key] = struct{}{}
	}
	nodeMap, err := m.makeInternalChasmNodeMap(input.ChasmNodes)
	if err != nil {
		return nil, err
	}
	result.ChasmNodes = nodeMap

	result.Checksum, err = m.serializer.ChecksumToBlob(input.Checksum)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *executionManagerImpl) DeleteWorkflowExecution(
	ctx context.Context,
	request *DeleteWorkflowExecutionRequest,
) error {
	return m.persistence.DeleteWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *DeleteCurrentWorkflowExecutionRequest,
) error {
	return m.persistence.DeleteCurrentWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) GetCurrentExecution(
	ctx context.Context,
	request *GetCurrentExecutionRequest,
) (*GetCurrentExecutionResponse, error) {
	response, respErr := m.persistence.GetCurrentExecution(ctx, request)

	var notFound *serviceerror.NotFound
	if errors.As(respErr, &notFound) {
		// strip persistence-specific error message
		respErr = serviceerror.NewNotFoundf("workflow not found for ID: %v", request.WorkflowID)
	}
	if respErr != nil && response == nil {
		// try to utilize resp as much as possible, for RebuildMutableState API
		return nil, respErr
	}

	return &GetCurrentExecutionResponse{
		RunID:          response.RunID,
		StartRequestID: response.ExecutionState.CreateRequestId,
		State:          response.ExecutionState.State,
		Status:         response.ExecutionState.Status,
	}, respErr
}

func (m *executionManagerImpl) ListConcreteExecutions(
	ctx context.Context,
	request *ListConcreteExecutionsRequest,
) (*ListConcreteExecutionsResponse, error) {
	response, err := m.persistence.ListConcreteExecutions(ctx, request)
	if err != nil {
		return nil, err
	}
	newResponse := &ListConcreteExecutionsResponse{
		States:    make([]*persistencespb.WorkflowMutableState, len(response.States)),
		PageToken: response.NextPageToken,
	}
	for i, s := range response.States {
		state, err := m.toWorkflowMutableState(s)
		if err != nil {
			return nil, err
		}
		newResponse.States[i] = state
	}
	return newResponse, nil
}

func (m *executionManagerImpl) AddHistoryTasks(
	ctx context.Context,
	input *AddHistoryTasksRequest,
) error {
	tasks, err := serializeTasks(m.serializer, input.Tasks)
	if err != nil {
		return err
	}

	return m.persistence.AddHistoryTasks(ctx, &InternalAddHistoryTasksRequest{
		ShardID: input.ShardID,
		RangeID: input.RangeID,

		NamespaceID: input.NamespaceID,
		WorkflowID:  input.WorkflowID,

		Tasks: tasks,
	})
}

func (m *executionManagerImpl) GetHistoryTasks(
	ctx context.Context,
	request *GetHistoryTasksRequest,
) (*GetHistoryTasksResponse, error) {
	if err := validateTaskRange(
		request.TaskCategory.Type(),
		request.InclusiveMinTaskKey,
		request.ExclusiveMaxTaskKey,
	); err != nil {
		return nil, err
	}

	resp, err := m.persistence.GetHistoryTasks(ctx, request)
	if err != nil {
		return nil, err
	}

	historyTasks := make([]tasks.Task, 0, len(resp.Tasks))
	for _, internalTask := range resp.Tasks {
		task, err := m.serializer.DeserializeTask(request.TaskCategory, internalTask.Blob)
		if err != nil {
			return nil, err
		}

		if !internalTask.Key.FireTime.Equal(tasks.DefaultFireTime) {
			task.SetVisibilityTime(internalTask.Key.FireTime)
		}
		task.SetTaskID(internalTask.Key.TaskID)

		historyTasks = append(historyTasks, task)
	}

	return &GetHistoryTasksResponse{
		Tasks:         historyTasks,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (m *executionManagerImpl) CompleteHistoryTask(
	ctx context.Context,
	request *CompleteHistoryTaskRequest,
) error {
	return m.persistence.CompleteHistoryTask(ctx, request)
}

func (m *executionManagerImpl) RangeCompleteHistoryTasks(
	ctx context.Context,
	request *RangeCompleteHistoryTasksRequest,
) error {
	if err := validateTaskRange(
		request.TaskCategory.Type(),
		request.InclusiveMinTaskKey,
		request.ExclusiveMaxTaskKey,
	); err != nil {
		return err
	}

	return m.persistence.RangeCompleteHistoryTasks(ctx, request)
}

func (m *executionManagerImpl) PutReplicationTaskToDLQ(
	ctx context.Context,
	request *PutReplicationTaskToDLQRequest,
) error {
	return m.persistence.PutReplicationTaskToDLQ(ctx, request)
}

func (m *executionManagerImpl) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *GetReplicationTasksFromDLQRequest,
) (*GetHistoryTasksResponse, error) {
	resp, err := m.persistence.GetReplicationTasksFromDLQ(ctx, request)
	if err != nil {
		return nil, err
	}

	category := tasks.CategoryReplication
	dlqTasks := make([]tasks.Task, 0, len(resp.Tasks))
	for i := range resp.Tasks {
		internalTask := resp.Tasks[i]
		task, err := m.serializer.DeserializeTask(category, internalTask.Blob)
		if err != nil {
			return nil, err
		}

		if !internalTask.Key.FireTime.Equal(tasks.DefaultFireTime) {
			task.SetVisibilityTime(internalTask.Key.FireTime)
		}
		task.SetTaskID(internalTask.Key.TaskID)

		dlqTasks = append(dlqTasks, task)
	}

	return &GetHistoryTasksResponse{
		Tasks:         dlqTasks,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (m *executionManagerImpl) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *DeleteReplicationTaskFromDLQRequest,
) error {
	return m.persistence.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *RangeDeleteReplicationTaskFromDLQRequest,
) error {
	return m.persistence.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) IsReplicationDLQEmpty(
	ctx context.Context,
	request *GetReplicationTasksFromDLQRequest,
) (bool, error) {
	return m.persistence.IsReplicationDLQEmpty(ctx, request)
}

func (m *executionManagerImpl) Close() {
	m.persistence.Close()
}

func (m *executionManagerImpl) trimHistoryNode(
	ctx context.Context,
	shardID int32,
	namespaceID string,
	workflowID string,
	runID string,
) {
	response, err := m.GetWorkflowExecution(ctx, &GetWorkflowExecutionRequest{
		ShardID:     shardID,
		NamespaceID: namespaceID,
		WorkflowID:  workflowID,
		RunID:       runID,
	})
	if err != nil {
		m.logger.Error("ExecutionManager unable to get mutable state for trimming history branch",
			tag.WorkflowNamespaceID(namespaceID),
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.Error(err),
		)
		return // best effort trim
	}

	executionInfo := response.State.ExecutionInfo
	branchToken, err := getCurrentBranchToken(executionInfo.VersionHistories)
	if err != nil {
		return
	}
	mutableStateLastNodeID := executionInfo.LastFirstEventId
	mutableStateLastNodeTransactionID := executionInfo.LastFirstEventTxnId
	if _, err := m.TrimHistoryBranch(ctx, &TrimHistoryBranchRequest{
		ShardID:       shardID,
		BranchToken:   branchToken,
		NodeID:        mutableStateLastNodeID,
		TransactionID: mutableStateLastNodeTransactionID,
	}); err != nil {
		// best effort trim
		m.logger.Error("ExecutionManager unable to trim history branch",
			tag.WorkflowNamespaceID(namespaceID),
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.Error(err),
		)
		return
	}
}

func (m *executionManagerImpl) toWorkflowMutableState(internState *InternalWorkflowMutableState) (*persistencespb.WorkflowMutableState, error) {
	state := &persistencespb.WorkflowMutableState{
		ActivityInfos:       make(map[int64]*persistencespb.ActivityInfo),
		TimerInfos:          make(map[string]*persistencespb.TimerInfo),
		ChildExecutionInfos: make(map[int64]*persistencespb.ChildExecutionInfo),
		RequestCancelInfos:  make(map[int64]*persistencespb.RequestCancelInfo),
		SignalInfos:         make(map[int64]*persistencespb.SignalInfo),
		ChasmNodes:          make(map[string]*persistencespb.ChasmNode),
		SignalRequestedIds:  internState.SignalRequestedIDs,
		NextEventId:         internState.NextEventID,
		BufferedEvents:      make([]*historypb.HistoryEvent, len(internState.BufferedEvents)),
	}
	for key, blob := range internState.ActivityInfos {
		info, err := m.serializer.ActivityInfoFromBlob(blob)
		if err != nil {
			return nil, err
		}
		state.ActivityInfos[key] = info
	}
	for key, blob := range internState.TimerInfos {
		info, err := m.serializer.TimerInfoFromBlob(blob)
		if err != nil {
			return nil, err
		}
		state.TimerInfos[key] = info
	}
	for key, blob := range internState.ChildExecutionInfos {
		info, err := m.serializer.ChildExecutionInfoFromBlob(blob)
		if err != nil {
			return nil, err
		}
		state.ChildExecutionInfos[key] = info
	}
	for key, blob := range internState.RequestCancelInfos {
		info, err := m.serializer.RequestCancelInfoFromBlob(blob)
		if err != nil {
			return nil, err
		}
		state.RequestCancelInfos[key] = info
	}
	for key, blob := range internState.SignalInfos {
		info, err := m.serializer.SignalInfoFromBlob(blob)
		if err != nil {
			return nil, err
		}
		state.SignalInfos[key] = info
	}
	for key, internal := range internState.ChasmNodes {
		var node *persistencespb.ChasmNode
		var err error

		if internal.CassandraBlob != nil {
			node, err = m.serializer.ChasmNodeFromBlob(internal.CassandraBlob)
		} else {
			node, err = m.serializer.ChasmNodeFromBlobs(internal.Metadata, internal.Data)
		}
		if err != nil {
			return nil, err
		}

		state.ChasmNodes[key] = node
	}
	var err error
	state.ExecutionInfo, err = m.serializer.WorkflowExecutionInfoFromBlob(internState.ExecutionInfo)
	if err != nil {
		return nil, err
	}
	if state.ExecutionInfo.AutoResetPoints == nil {
		// TODO: check if we need this?
		state.ExecutionInfo.AutoResetPoints = &workflowpb.ResetPoints{}
	}
	state.ExecutionState, err = m.serializer.WorkflowExecutionStateFromBlob(internState.ExecutionState)
	if err != nil {
		return nil, err
	}
	state.BufferedEvents, err = m.DeserializeBufferedEvents(internState.BufferedEvents)
	if err != nil {
		return nil, err
	}
	if internState.Checksum != nil {
		state.Checksum, err = m.serializer.ChecksumFromBlob(internState.Checksum)
	}
	if err != nil {
		return nil, err
	}

	return state, nil
}

func getCurrentBranchToken(
	versionHistories *historyspb.VersionHistories,
) ([]byte, error) {
	// TODO remove this if check once legacy execution tests are removed
	if versionHistories == nil {
		return nil, serviceerror.NewInternal("version history is empty")
	}
	versionHistory, err := versionhistory.GetCurrentVersionHistory(versionHistories)
	if err != nil {
		return nil, err
	}
	return versionHistory.BranchToken, nil
}

func getCurrentBranchLastWriteVersion(
	versionHistories *historyspb.VersionHistories,
	transitions []*persistencespb.VersionedTransition,
) (int64, error) {
	// TODO remove this if check once legacy execution tests are removed
	if versionHistories == nil {
		return common.EmptyVersion, nil
	}
	versionHistory, err := versionhistory.GetCurrentVersionHistory(versionHistories)
	if err != nil {
		return 0, err
	}

	if !versionhistory.IsEmptyVersionHistory(versionHistory) {
		versionHistoryItem, err := versionhistory.GetLastVersionHistoryItem(versionHistory)
		if err != nil {
			return 0, err
		}
		return versionHistoryItem.GetVersion(), nil
	}

	// No version history for the run, this only happens for CHASM run and we need to check the transition history.
	//
	// TODO: The logic should ignore version history and always use transition history, even for Workflows.
	// We are still checking version history first here since it's the old logic and we want to minimize the risk for now
	// and to account for the fact that transition history is not fully enabled.
	//
	// Theoritically, using version history here is wrong because there can be transitions (even on Workflows) that have no
	// events (e.g. Activity Heartbeat).
	//
	// Although using version history has the benefit of ensuring the returned version don't change after the run is closed, using
	// transition history is also correct here because the returned version is only used for updating mutable state current record
	// and that record won't be updated after the run is closed.
	//
	// Using transition history also suits the function name LastWriteVersion better.
	if len(transitions) != 0 {
		return transitionhistory.LastVersionedTransition(transitions).NamespaceFailoverVersion, nil
	}
	return common.EmptyVersion, serviceerror.NewInternal("both version history and transition history are empty")
}

func serializeTasks(
	serializer serialization.Serializer,
	inputTasks map[tasks.Category][]tasks.Task,
) (map[tasks.Category][]InternalHistoryTask, error) {
	outputTasks := make(map[tasks.Category][]InternalHistoryTask)
	for category, tasks := range inputTasks {
		serializedTasks := make([]InternalHistoryTask, 0, len(tasks))
		for _, task := range tasks {
			blob, err := serializer.SerializeTask(task)
			if err != nil {
				return nil, err
			}
			serializedTasks = append(serializedTasks, InternalHistoryTask{
				Key:  task.GetKey(),
				Blob: blob,
			})
		}
		outputTasks[category] = serializedTasks
	}
	return outputTasks, nil
}

func validateTaskRange(
	taskCategoryType tasks.CategoryType,
	minTaskKey tasks.Key,
	maxTaskKey tasks.Key,
) error {
	minTaskIDSpecified := minTaskKey.TaskID != 0
	minFireTimeSpecified := !minTaskKey.FireTime.IsZero() && !minTaskKey.FireTime.Equal(tasks.DefaultFireTime)
	maxTaskIDSpecified := maxTaskKey.TaskID != 0
	maxFireTimeSpecified := !maxTaskKey.FireTime.IsZero() && !maxTaskKey.FireTime.Equal(tasks.DefaultFireTime)

	switch taskCategoryType {
	case tasks.CategoryTypeImmediate:
		if !maxTaskIDSpecified {
			return serviceerror.NewInvalidArgument("invalid task range, max taskID must be specified for immediate task category")
		}
		if minFireTimeSpecified || maxFireTimeSpecified {
			return serviceerror.NewInvalidArgument("invalid task range, fireTime must be empty for immediate task category")
		}
	case tasks.CategoryTypeScheduled:
		if !maxFireTimeSpecified {
			return serviceerror.NewInvalidArgument("invalid task range, max fire time must be specified for scheduled task category")
		}
		if minTaskIDSpecified || maxTaskIDSpecified {
			return serviceerror.NewInvalidArgument("invalid task range, taskID must be empty for scheduled task category")
		}
	default:
		return serviceerror.NewInvalidArgumentf("invalid task category type: %v", taskCategoryType)
	}

	return nil
}

func (m *executionManagerImpl) makeInternalChasmNodeMap(
	nodes map[string]*persistencespb.ChasmNode,
) (map[string]InternalChasmNode, error) {
	res := make(map[string]InternalChasmNode, len(nodes))
	isCassandra := strings.Contains(m.GetName(), "cassandra")

	for path, node := range nodes {
		var internal InternalChasmNode

		// If we're running on Cassandra, set a single blob since that's how we store it.
		if isCassandra {
			blob, err := m.serializer.ChasmNodeToBlob(node)
			if err != nil {
				return nil, err
			}
			internal = InternalChasmNode{
				CassandraBlob: blob,
			}
		} else {
			// Otherwise, split the node into separate blobs.
			metadata, data, err := m.serializer.ChasmNodeToBlobs(node)
			if err != nil {
				return nil, err
			}
			internal = InternalChasmNode{
				Metadata: metadata,
				Data:     data,
			}
		}

		res[path] = internal
	}

	return res, nil
}
