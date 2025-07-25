package dynamicconfig

import (
	"math"
	"os"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	sdkworker "go.temporal.io/sdk/worker"
	"go.temporal.io/server/common/debug"
	"go.temporal.io/server/common/primitives"
	"go.temporal.io/server/common/retrypolicy"
)

var (
	// keys for dynamic config itself
	DynamicConfigSubscriptionCallback = NewGlobalTypedSetting(
		"dynamicconfig.subscriptionCallback",
		subscriptionCallbackSettings{
			MinWorkers:   10,
			MaxWorkers:   1e9, // effectively unlimited
			TargetDelay:  10 * time.Millisecond,
			ShrinkFactor: 1000, // 10 seconds
		},
		`Settings for dynamic config subscription dispatch. Requires server restart.`,
	)
	DynamicConfigSubscriptionPollInterval = NewGlobalDurationSetting(
		"dynamicconfig.subscriptionPollInterval",
		time.Minute,
		`Poll interval for emulating subscriptions on non-subscribable Client.`,
	)

	// keys for admin

	AdminEnableListHistoryTasks = NewGlobalBoolSetting(
		"admin.enableListHistoryTasks",
		true,
		`AdminEnableListHistoryTasks is the key for enabling listing history tasks`,
	)
	AdminMatchingNamespaceToPartitionDispatchRate = NewNamespaceFloatSetting(
		"admin.matchingNamespaceToPartitionDispatchRate",
		10000,
		`AdminMatchingNamespaceToPartitionDispatchRate is the max qps of any task queue partition for a given namespace`,
	)
	AdminMatchingNamespaceTaskqueueToPartitionDispatchRate = NewTaskQueueFloatSetting(
		"admin.matchingNamespaceTaskqueueToPartitionDispatchRate",
		1000,
		`AdminMatchingNamespaceTaskqueueToPartitionDispatchRate is the max qps of a task queue partition for a given namespace & task queue`,
	)

	// keys for system

	VisibilityPersistenceMaxReadQPS = NewGlobalIntSetting(
		"system.visibilityPersistenceMaxReadQPS",
		9000,
		`VisibilityPersistenceMaxReadQPS is the max QPC system host can query visibility DB for read.`,
	)
	VisibilityPersistenceMaxWriteQPS = NewGlobalIntSetting(
		"system.visibilityPersistenceMaxWriteQPS",
		9000,
		`VisibilityPersistenceMaxWriteQPS is the max QPC system host can query visibility DB for write.`,
	)
	VisibilityPersistenceSlowQueryThreshold = NewGlobalDurationSetting(
		"system.visibilityPersistenceSlowQueryThreshold",
		time.Second,
		`VisibilityPersistenceSlowQueryThreshold is the threshold above which a query is considered slow and logged.`,
	)
	EnableReadFromSecondaryVisibility = NewNamespaceBoolSetting(
		"system.enableReadFromSecondaryVisibility",
		false,
		`EnableReadFromSecondaryVisibility is the config to enable read from secondary visibility`,
	)
	VisibilityEnableShadowReadMode = NewGlobalBoolSetting(
		"system.visibilityEnableShadowReadMode",
		false,
		`VisibilityEnableShadowReadMode is the config to enable shadow read from secondary visibility`,
	)
	SecondaryVisibilityWritingMode = NewGlobalStringSetting(
		"system.secondaryVisibilityWritingMode",
		"off",
		`SecondaryVisibilityWritingMode is key for how to write to secondary visibility`,
	)
	VisibilityDisableOrderByClause = NewNamespaceBoolSetting(
		"system.visibilityDisableOrderByClause",
		true,
		`VisibilityDisableOrderByClause is the config to disable ORDERY BY clause for Elasticsearch`,
	)
	VisibilityEnableManualPagination = NewNamespaceBoolSetting(
		"system.visibilityEnableManualPagination",
		true,
		`VisibilityEnableManualPagination is the config to enable manual pagination for Elasticsearch`,
	)
	VisibilityAllowList = NewNamespaceBoolSetting(
		"system.visibilityAllowList",
		true,
		`VisibilityAllowList is the config to allow list of values for regular types`,
	)
	SuppressErrorSetSystemSearchAttribute = NewNamespaceBoolSetting(
		"system.suppressErrorSetSystemSearchAttribute",
		false,
		`SuppressErrorSetSystemSearchAttribute suppresses errors when trying to set
values in system search attributes.`,
	)

	HistoryArchivalState = NewGlobalStringSetting(
		"system.historyArchivalState",
		"", // actual default is from static config
		`HistoryArchivalState is key for the state of history archival`,
	)
	EnableReadFromHistoryArchival = NewGlobalBoolSetting(
		"system.enableReadFromHistoryArchival",
		false, // actual default is from static config
		`EnableReadFromHistoryArchival is key for enabling reading history from archival store`,
	)
	VisibilityArchivalState = NewGlobalStringSetting(
		"system.visibilityArchivalState",
		"", // actual default is from static config
		`VisibilityArchivalState is key for the state of visibility archival`,
	)
	EnableReadFromVisibilityArchival = NewGlobalBoolSetting(
		"system.enableReadFromVisibilityArchival",
		false, // actual default is from static config
		`EnableReadFromVisibilityArchival is key for enabling reading visibility from archival store`,
	)
	EnableNamespaceNotActiveAutoForwarding = NewNamespaceBoolSetting(
		"system.enableNamespaceNotActiveAutoForwarding",
		true,
		`EnableNamespaceNotActiveAutoForwarding whether enabling DC auto forwarding to active cluster
for signal / start / signal with start API if namespace is not active`,
	)
	EnableNamespaceHandoverWait = NewNamespaceBoolSetting(
		"system.enableNamespaceHandoverWait",
		false,
		`EnableNamespaceHandoverWait whether waiting for namespace replication state update before serve the request`,
	)
	TransactionSizeLimit = NewGlobalIntSetting(
		"system.transactionSizeLimit",
		primitives.DefaultTransactionSizeLimit,
		`TransactionSizeLimit is the largest allowed transaction size to persistence`,
	)
	DisallowQuery = NewNamespaceBoolSetting(
		"system.disallowQuery",
		false,
		`DisallowQuery is the key to disallow query for a namespace`,
	)
	EnableCrossNamespaceCommands = NewGlobalBoolSetting(
		"system.enableCrossNamespaceCommands",
		true,
		`EnableCrossNamespaceCommands is the key to enable commands for external namespaces`,
	)
	ClusterMetadataRefreshInterval = NewGlobalDurationSetting(
		"system.clusterMetadataRefreshInterval",
		time.Minute,
		`ClusterMetadataRefreshInterval is config to manage cluster metadata table refresh interval`,
	)
	ForceSearchAttributesCacheRefreshOnRead = NewGlobalBoolSetting(
		"system.forceSearchAttributesCacheRefreshOnRead",
		false,
		`ForceSearchAttributesCacheRefreshOnRead forces refreshing search attributes cache on a read operation, so we always
get the latest data from DB. This effectively bypasses cache value and is used to facilitate testing of changes in
search attributes. This should not be turned on in production.`,
	)
	EnableRingpopTLS = NewGlobalBoolSetting(
		"system.enableRingpopTLS",
		false,
		`EnableRingpopTLS controls whether to use TLS for ringpop, using the same "internode" TLS
config as the other services.`,
	)
	RingpopApproximateMaxPropagationTime = NewGlobalDurationSetting(
		"system.ringpopApproximateMaxPropagationTime",
		3*time.Second,
		`RingpopApproximateMaxPropagationTime is used for timing certain startup and shutdown processes.
(It is not and doesn't have to be a guarantee.)`,
	)
	EnableParentClosePolicyWorker = NewGlobalBoolSetting(
		"system.enableParentClosePolicyWorker",
		true,
		`EnableParentClosePolicyWorker decides whether or not enable system workers for processing parent close policy task`,
	)
	EnableStickyQuery = NewNamespaceBoolSetting(
		"system.enableStickyQuery",
		true,
		`EnableStickyQuery indicates if sticky query should be enabled per namespace`,
	)
	EnableActivityEagerExecution = NewNamespaceBoolSetting(
		"system.enableActivityEagerExecution",
		false,
		`EnableActivityEagerExecution indicates if activity eager execution is enabled per namespace`,
	)
	EnableEagerWorkflowStart = NewNamespaceBoolSetting(
		"system.enableEagerWorkflowStart",
		true,
		`Toggles "eager workflow start" - returning the first workflow task inline in the
response to a StartWorkflowExecution request and skipping the trip through matching.`,
	)
	NamespaceCacheRefreshInterval = NewGlobalDurationSetting(
		"system.namespaceCacheRefreshInterval",
		2*time.Second,
		`NamespaceCacheRefreshInterval is the key for namespace cache refresh interval dynamic config`,
	)
	PersistenceHealthSignalMetricsEnabled = NewGlobalBoolSetting(
		"system.persistenceHealthSignalMetricsEnabled",
		true,
		`PersistenceHealthSignalMetricsEnabled determines whether persistence shard RPS metrics are emitted`,
	)
	HistoryHealthSignalMetricsEnabled = NewGlobalBoolSetting(
		"system.historyHealthSignalMetricsEnabled",
		true,
		`HistoryHealthSignalMetricsEnabled determines whether history service RPC metrics are emitted`,
	)
	PersistenceHealthSignalAggregationEnabled = NewGlobalBoolSetting(
		"system.persistenceHealthSignalAggregationEnabled",
		true,
		`PersistenceHealthSignalAggregationEnabled determines whether persistence latency and error averages are tracked`,
	)
	PersistenceHealthSignalWindowSize = NewGlobalDurationSetting(
		"system.persistenceHealthSignalWindowSize",
		10*time.Second,
		`PersistenceHealthSignalWindowSize is the time window size in seconds for aggregating persistence signals`,
	)
	PersistenceHealthSignalBufferSize = NewGlobalIntSetting(
		"system.persistenceHealthSignalBufferSize",
		5000,
		`PersistenceHealthSignalBufferSize is the maximum number of persistence signals to buffer in memory per signal key`,
	)
	OperatorRPSRatio = NewGlobalFloatSetting(
		"system.operatorRPSRatio",
		0.2,
		`OperatorRPSRatio is the percentage of the rate limit provided to priority rate limiters that should be used for
operator API calls (highest priority). Should be >0.0 and <= 1.0 (defaults to 20% if not specified)`,
	)
	// TODO: The following 2 configs should be removed once server keepalive and client keepalive are enabled by default
	EnableInternodeServerKeepAlive = NewGlobalBoolSetting(
		"system.enableInternodeServerKeepAlive",
		false,
		`enableInternodeServerKeepAlive is the config to enable keep alive for inter-node connections on server side.`,
	)
	EnableInternodeClientKeepAlive = NewGlobalBoolSetting(
		"system.enableInternodeClientKeepAlive",
		false,
		`enableInternodeClientKeepAlive is the config to enable keep alive for inter-node connections on client side.`,
	)

	PersistenceQPSBurstRatio = NewGlobalFloatSetting(
		"system.persistenceQPSBurstRatio",
		1.0,
		`PersistenceQPSBurstRatio is the burst ratio for persistence QPS. This flag controls the burst ratio for all services.`,
	)

	// deadlock detector

	DeadlockDumpGoroutines = NewGlobalBoolSetting(
		"system.deadlock.DumpGoroutines",
		true,
		`Whether the deadlock detector should dump goroutines`,
	)
	DeadlockFailHealthCheck = NewGlobalBoolSetting(
		"system.deadlock.FailHealthCheck",
		false,
		`Whether the deadlock detector should cause the grpc server to fail health checks`,
	)
	DeadlockAbortProcess = NewGlobalBoolSetting(
		"system.deadlock.AbortProcess",
		false,
		`Whether the deadlock detector should abort the process`,
	)
	DeadlockInterval = NewGlobalDurationSetting(
		"system.deadlock.Interval",
		60*time.Second,
		`How often the detector checks each root.`,
	)
	DeadlockMaxWorkersPerRoot = NewGlobalIntSetting(
		"system.deadlock.MaxWorkersPerRoot",
		10,
		`How many extra goroutines can be created per root.`,
	)

	// keys for size limit

	BlobSizeLimitError = NewNamespaceIntSetting(
		"limit.blobSize.error",
		2*1024*1024,
		`BlobSizeLimitError is the per event blob size limit`,
	)
	BlobSizeLimitWarn = NewNamespaceIntSetting(
		"limit.blobSize.warn",
		512*1024,
		`BlobSizeLimitWarn is the per event blob size limit for warning`,
	)
	MemoSizeLimitError = NewNamespaceIntSetting(
		"limit.memoSize.error",
		2*1024*1024,
		`MemoSizeLimitError is the per event memo size limit`,
	)
	MemoSizeLimitWarn = NewNamespaceIntSetting(
		"limit.memoSize.warn",
		2*1024,
		`MemoSizeLimitWarn is the per event memo size limit for warning`,
	)
	NumPendingChildExecutionsLimitError = NewNamespaceIntSetting(
		"limit.numPendingChildExecutions.error",
		2000,
		`NumPendingChildExecutionsLimitError is the maximum number of pending child workflows a workflow can have before
StartChildWorkflowExecution commands will fail.`,
	)
	NumPendingActivitiesLimitError = NewNamespaceIntSetting(
		"limit.numPendingActivities.error",
		2000,
		`NumPendingActivitiesLimitError is the maximum number of pending activities a workflow can have before
ScheduleActivityTask will fail.`,
	)
	NumPendingSignalsLimitError = NewNamespaceIntSetting(
		"limit.numPendingSignals.error",
		2000,
		`NumPendingSignalsLimitError is the maximum number of pending signals a workflow can have before
SignalExternalWorkflowExecution commands from this workflow will fail.`,
	)
	NumPendingCancelRequestsLimitError = NewNamespaceIntSetting(
		"limit.numPendingCancelRequests.error",
		2000,
		`NumPendingCancelRequestsLimitError is the maximum number of pending requests to cancel other workflows a workflow can have before
RequestCancelExternalWorkflowExecution commands will fail.`,
	)
	HistorySizeLimitError = NewNamespaceIntSetting(
		"limit.historySize.error",
		50*1024*1024,
		`HistorySizeLimitError is the per workflow execution history size limit`,
	)
	HistorySizeLimitWarn = NewNamespaceIntSetting(
		"limit.historySize.warn",
		10*1024*1024,
		`HistorySizeLimitWarn is the per workflow execution history size limit for warning`,
	)
	HistorySizeSuggestContinueAsNew = NewNamespaceIntSetting(
		"limit.historySize.suggestContinueAsNew",
		4*1024*1024,
		`HistorySizeSuggestContinueAsNew is the workflow execution history size limit to suggest
continue-as-new (in workflow task started event)`,
	)
	HistoryCountLimitError = NewNamespaceIntSetting(
		"limit.historyCount.error",
		50*1024,
		`HistoryCountLimitError is the per workflow execution history event count limit`,
	)
	HistoryCountLimitWarn = NewNamespaceIntSetting(
		"limit.historyCount.warn",
		10*1024,
		`HistoryCountLimitWarn is the per workflow execution history event count limit for warning`,
	)
	MutableStateActivityFailureSizeLimitError = NewNamespaceIntSetting(
		"limit.mutableStateActivityFailureSize.error",
		4*1024,
		`MutableStateActivityFailureSizeLimitError is the per activity failure size limit for workflow mutable state.
If exceeded, failure will be truncated before being stored in mutable state.`,
	)
	MutableStateActivityFailureSizeLimitWarn = NewNamespaceIntSetting(
		"limit.mutableStateActivityFailureSize.warn",
		2*1024,
		`MutableStateActivityFailureSizeLimitWarn is the per activity failure size warning limit for workflow mutable state`,
	)
	MutableStateSizeLimitError = NewGlobalIntSetting(
		"limit.mutableStateSize.error",
		8*1024*1024,
		`MutableStateSizeLimitError is the per workflow execution mutable state size limit in bytes`,
	)
	MutableStateSizeLimitWarn = NewGlobalIntSetting(
		"limit.mutableStateSize.warn",
		1*1024*1024,
		`MutableStateSizeLimitWarn is the per workflow execution mutable state size limit in bytes for warning`,
	)
	MutableStateTombstoneCountLimit = NewGlobalIntSetting(
		"limit.mutableStateTombstoneCountLimit",
		16,
		`MutableStateTombstoneCountLimit is the maximum number of deleted sub state machines tracked in mutable state.`,
	)
	HistoryCountSuggestContinueAsNew = NewNamespaceIntSetting(
		"limit.historyCount.suggestContinueAsNew",
		4*1024,
		`HistoryCountSuggestContinueAsNew is the workflow execution history event count limit to
suggest continue-as-new (in workflow task started event)`,
	)
	HistoryMaxPageSize = NewNamespaceIntSetting(
		"limit.historyMaxPageSize",
		primitives.GetHistoryMaxPageSize,
		`HistoryMaxPageSize is default max size for GetWorkflowExecutionHistory in one page`,
	)
	MaxIDLengthLimit = NewGlobalIntSetting(
		"limit.maxIDLength",
		1000,
		`MaxIDLengthLimit is the length limit for various IDs, including: Namespace, TaskQueue, WorkflowID, ActivityID, TimerID,
WorkflowType, ActivityType, SignalName, MarkerName, ErrorReason/FailureReason/CancelCause, Identity, RequestID`,
	)
	WorkerBuildIdSizeLimit = NewGlobalIntSetting(
		"limit.workerBuildIdSize",
		255,
		`WorkerBuildIdSizeLimit is the byte length limit for a worker build id as used in the rpc methods for updating
the version sets for a task queue.
Do not set this to a value higher than 255 for clusters using SQL based persistence due to predefined VARCHAR
column width.`,
	)
	VersionCompatibleSetLimitPerQueue = NewNamespaceIntSetting(
		"limit.versionCompatibleSetLimitPerQueue",
		10,
		`VersionCompatibleSetLimitPerQueue is the max number of compatible sets allowed in the versioning data for a task
queue. Update requests which would cause the versioning data to exceed this number will fail with a
FailedPrecondition error.`,
	)
	VersionBuildIdLimitPerQueue = NewNamespaceIntSetting(
		"limit.versionBuildIdLimitPerQueue",
		100,
		`VersionBuildIdLimitPerQueue is the max number of build IDs allowed to be defined in the versioning data for a
task queue. Update requests which would cause the versioning data to exceed this number will fail with a
FailedPrecondition error.`,
	)
	AssignmentRuleLimitPerQueue = NewNamespaceIntSetting(
		"limit.wv.AssignmentRuleLimitPerQueue",
		100,
		`AssignmentRuleLimitPerQueue is the max number of Build ID assignment rules allowed to be defined in the
versioning data for a task queue. Update requests which would cause the versioning data to exceed this number
will fail with a FailedPrecondition error.`,
	)
	RedirectRuleLimitPerQueue = NewNamespaceIntSetting(
		"limit.wv.RedirectRuleLimitPerQueue",
		500,
		`RedirectRuleLimitPerQueue is the max number of compatible redirect rules allowed to be defined
in the versioning data for a task queue. Update requests which would cause the versioning data to exceed this
number will fail with a FailedPrecondition error.`,
	)
	RedirectRuleMaxUpstreamBuildIDsPerQueue = NewNamespaceIntSetting(
		"limit.wv.RedirectRuleMaxUpstreamBuildIDsPerQueue",
		50,
		`RedirectRuleMaxUpstreamBuildIDsPerQueue is the max number of compatible redirect rules allowed to be connected
in one chain in the versioning data for a task queue. Update requests which would cause the versioning data
to exceed this number will fail with a FailedPrecondition error.`,
	)
	MatchingDeletedRuleRetentionTime = NewNamespaceDurationSetting(
		"matching.wv.DeletedRuleRetentionTime",
		14*24*time.Hour,
		`MatchingDeletedRuleRetentionTime is the length of time that deleted Version Assignment Rules and
Deleted Redirect Rules will be kept in the DB (with DeleteTimestamp). After this time, the tombstones are deleted at the next time update of versioning data for the task queue.`,
	)
	PollerHistoryTTL = NewNamespaceDurationSetting(
		"matching.PollerHistoryTTL",
		5*time.Minute,
		`PollerHistoryTTL is the time to live for poller histories in the pollerHistory cache of a physical task queue. Poller histories are fetched when
		requiring a list of pollers that polled a given task queue.`,
	)
	ReachabilityBuildIdVisibilityGracePeriod = NewNamespaceDurationSetting(
		"matching.wv.ReachabilityBuildIdVisibilityGracePeriod",
		3*time.Minute,
		`ReachabilityBuildIdVisibilityGracePeriod is the time period for which deleted versioning rules are still considered active
to account for the delay in updating the build id field in visibility. Not yet supported for GetDeploymentReachability. We recommend waiting
at least 2 minutes between changing the current deployment and calling GetDeployment, so that newly started workflow executions using the
recently-current deployment can arrive in visibility.`,
	)
	VersionDrainageStatusVisibilityGracePeriod = NewNamespaceDurationSetting(
		"matching.wv.VersionDrainageStatusVisibilityGracePeriod",
		3*time.Minute,
		`VersionDrainageStatusVisibilityGracePeriod is the time period for which non-current / non-ramping worker deployment versions
are still considered active to account for the delay in updating the build id field in visibility.`,
	)
	VersionDrainageStatusRefreshInterval = NewNamespaceDurationSetting(
		"matching.wv.VersionDrainageStatusRefreshInterval",
		3*time.Minute,
		`VersionDrainageStatusRefreshInterval is the interval at which each draining deployment version refreshes its
Drainage Status by querying visibility for open pinned workflows using that version.`,
	)
	ReachabilityTaskQueueScanLimit = NewGlobalIntSetting(
		"limit.reachabilityTaskQueueScan",
		20,
		`ReachabilityTaskQueueScanLimit limits the number of task queues to scan when responding to a
GetWorkerTaskReachability query.`,
	)
	ReachabilityQueryBuildIdLimit = NewGlobalIntSetting(
		"limit.reachabilityQueryBuildIds",
		5,
		`ReachabilityQueryBuildIdLimit limits the number of build ids that can be requested in a single call to the
DescribeTaskQueue API with ReportTaskQueueReachability==true, or to the GetWorkerTaskReachability API.`,
	)
	ReachabilityCacheOpenWFsTTL = NewGlobalDurationSetting(
		"matching.wv.reachabilityCacheOpenWFsTTL",
		time.Minute,
		`ReachabilityCacheOpenWFsTTL is the TTL for the reachability open workflows cache.`,
	)
	ReachabilityCacheClosedWFsTTL = NewGlobalDurationSetting(
		"matching.wv.reachabilityCacheClosedWFsTTL",
		10*time.Minute,
		`ReachabilityCacheClosedWFsTTL is the TTL for the reachability closed workflows cache.`,
	)
	ReachabilityQuerySetDurationSinceDefault = NewGlobalDurationSetting(
		"frontend.reachabilityQuerySetDurationSinceDefault",
		5*time.Minute,
		`ReachabilityQuerySetDurationSinceDefault is the minimum period since a version set was demoted from being the
queue default before it is considered unreachable by new workflows.
This setting allows some propagation delay of versioning data for the reachability queries, which may happen for
the following reasons:
1. There are no workflows currently marked as open in the visibility store but a worker for the demoted version
is currently processing a task.
2. There are delays in the visibility task processor (which is asynchronous).
3. There's propagation delay of the versioning data between matching nodes.`,
	)
	TaskQueuesPerBuildIdLimit = NewNamespaceIntSetting(
		"limit.taskQueuesPerBuildId",
		20,
		`TaskQueuesPerBuildIdLimit limits the number of task queue names that can be mapped to a single build id.`,
	)

	NexusEndpointNameMaxLength = NewGlobalIntSetting(
		"limit.endpointNameMaxLength",
		200,
		`NexusEndpointNameMaxLength is the maximum length of a Nexus endpoint name.`,
	)
	NexusEndpointExternalURLMaxLength = NewGlobalIntSetting(
		"limit.endpointExternalURLMaxLength",
		4*1024,
		`NexusEndpointExternalURLMaxLength is the maximum length of a Nexus endpoint external target URL.`,
	)
	NexusEndpointDescriptionMaxSize = NewNamespaceIntSetting(
		"limit.endpointDescriptionMaxSize",
		20000,
		`Maximum size of Nexus Endpoint description payload in bytes including data and metadata.`,
	)
	NexusEndpointListDefaultPageSize = NewGlobalIntSetting(
		"limit.endpointListDefaultPageSize",
		100,
		`NexusEndpointListDefaultPageSize is the default page size for listing Nexus endpoints.`,
	)
	NexusEndpointListMaxPageSize = NewGlobalIntSetting(
		"limit.endpointListMaxPageSize",
		1000,
		`NexusEndpointListMaxPageSize is the maximum page size for listing Nexus endpoints.`,
	)

	RemovableBuildIdDurationSinceDefault = NewGlobalDurationSetting(
		"worker.removableBuildIdDurationSinceDefault",
		time.Hour,
		`RemovableBuildIdDurationSinceDefault is the minimum duration since a build id was last default in its containing
set for it to be considered for removal, used by the build id scavenger.
This setting allows some propagation delay of versioning data, which may happen for the following reasons:
1. There are no workflows currently marked as open in the visibility store but a worker for the demoted version
is currently processing a task.
2. There are delays in the visibility task processor (which is asynchronous).
3. There's propagation delay of the versioning data between matching nodes.`,
	)
	BuildIdScavengerVisibilityRPS = NewGlobalFloatSetting(
		"worker.buildIdScavengerVisibilityRPS",
		1.0,
		`BuildIdScavengerVisibilityRPS is the rate limit for visibility calls from the build id scavenger`,
	)

	// keys for frontend
	FrontendHTTPAllowedHosts = NewGlobalTypedSetting(
		"frontend.httpAllowedHosts",
		[]string(nil),
		`HTTP API Requests with a "Host" header matching the allowed hosts will be processed, otherwise rejected.
Wildcards (*) are expanded to allow any substring. By default any Host header is allowed.`,
	)
	FrontendPersistenceMaxQPS = NewGlobalIntSetting(
		"frontend.persistenceMaxQPS",
		2000,
		`FrontendPersistenceMaxQPS is the max qps frontend host can query DB`,
	)
	FrontendPersistenceGlobalMaxQPS = NewGlobalIntSetting(
		"frontend.persistenceGlobalMaxQPS",
		0,
		`FrontendPersistenceGlobalMaxQPS is the max qps frontend cluster can query DB`,
	)
	FrontendPersistenceNamespaceMaxQPS = NewNamespaceIntSetting(
		"frontend.persistenceNamespaceMaxQPS",
		0,
		`FrontendPersistenceNamespaceMaxQPS is the max qps each namespace on frontend host can query DB`,
	)
	FrontendPersistenceGlobalNamespaceMaxQPS = NewNamespaceIntSetting(
		"frontend.persistenceGlobalNamespaceMaxQPS",
		0,
		`FrontendPersistenceGlobalNamespaceMaxQPS is the max qps each namespace in frontend cluster can query DB`,
	)
	FrontendPersistenceDynamicRateLimitingParams = NewGlobalTypedSetting(
		"frontend.persistenceDynamicRateLimitingParams",
		DefaultDynamicRateLimitingParams,
		`FrontendPersistenceDynamicRateLimitingParams is a struct that contains all adjustable dynamic rate limiting params.
Fields: Enabled, RefreshInterval, LatencyThreshold, ErrorThreshold, RateBackoffStepSize, RateIncreaseStepSize, RateMultiMin, RateMultiMax.
See DynamicRateLimitingParams comments for more details.`,
	)
	FrontendVisibilityMaxPageSize = NewNamespaceIntSetting(
		"frontend.visibilityMaxPageSize",
		1000,
		`FrontendVisibilityMaxPageSize is default max size for ListWorkflowExecutions in one page`,
	)
	FrontendHistoryMaxPageSize = NewNamespaceIntSetting(
		"frontend.historyMaxPageSize",
		primitives.GetHistoryMaxPageSize,
		`FrontendHistoryMaxPageSize is default max size for GetWorkflowExecutionHistory in one page`,
	)
	FrontendRPS = NewGlobalIntSetting(
		"frontend.rps",
		2400,
		`FrontendRPS is workflow rate limit per second per-instance`,
	)
	FrontendGlobalRPS = NewGlobalIntSetting(
		"frontend.globalRPS",
		0,
		`FrontendGlobalRPS is workflow rate limit per second for the whole cluster`,
	)
	FrontendNamespaceReplicationInducingAPIsRPS = NewGlobalIntSetting(
		"frontend.rps.namespaceReplicationInducingAPIs",
		20,
		`FrontendNamespaceReplicationInducingAPIsRPS limits the per second request rate for namespace replication inducing
APIs (e.g. RegisterNamespace, UpdateNamespace, UpdateWorkerBuildIdCompatibility).
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	FrontendMaxNamespaceRPSPerInstance = NewNamespaceIntSetting(
		"frontend.namespaceRPS",
		2400,
		`FrontendMaxNamespaceRPSPerInstance is workflow namespace rate limit per second`,
	)
	FrontendMaxNamespaceBurstRatioPerInstance = NewNamespaceFloatSetting(
		"frontend.namespaceBurstRatio",
		2,
		`FrontendMaxNamespaceBurstRatioPerInstance is workflow namespace burst limit as a ratio of namespace RPS. The RPS
used here will be the effective RPS from global and per-instance limits. The value must be 1 or higher.`,
	)
	FrontendMaxConcurrentLongRunningRequestsPerInstance = NewNamespaceIntSetting(
		"frontend.namespaceCount",
		1200,
		`FrontendMaxConcurrentLongRunningRequestsPerInstance limits concurrent long-running requests per-instance,
per-API. Example requests include long-poll requests, and 'Query' requests (which need to wait for WFTs). The
limit is applied individually to each API method. This value is ignored if
FrontendGlobalMaxConcurrentLongRunningRequests is greater than zero. Warning: setting this to zero will cause all
long-running requests to fail. The name 'frontend.namespaceCount' is kept for backwards compatibility with
existing deployments even though it is a bit of a misnomer. This does not limit the number of namespaces; it is a
per-_namespace_ limit on the _count_ of long-running requests. Requests are only throttled when the limit is
exceeded, not when it is only reached.`,
	)
	ReducePollWorkflowHistoryRequestPriority = NewGlobalBoolSetting(
		"frontend.reducePollWorkflowRequestPriority",
		true,
		`ReducePollWorkflowRequestPriority decides whether to reduce the priority of GetWorkflowExecutionHistory
requests if WaitNewEvent is true.`,
	)
	FrontendGlobalMaxConcurrentLongRunningRequests = NewNamespaceIntSetting(
		"frontend.globalNamespaceCount",
		0,
		`FrontendGlobalMaxConcurrentLongRunningRequests limits concurrent long-running requests across all frontend
instances in the cluster, for a given namespace, per-API method. If this is set to 0 (the default), then it is
ignored. The name 'frontend.globalNamespaceCount' is kept for consistency with the per-instance limit name,
'frontend.namespaceCount'.`,
	)
	FrontendMaxNamespaceVisibilityRPSPerInstance = NewNamespaceIntSetting(
		"frontend.namespaceRPS.visibility",
		10,
		`FrontendMaxNamespaceVisibilityRPSPerInstance is namespace rate limit per second for visibility APIs.
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	FrontendMaxNamespaceNamespaceReplicationInducingAPIsRPSPerInstance = NewNamespaceIntSetting(
		"frontend.namespaceRPS.namespaceReplicationInducingAPIs",
		1,
		`FrontendMaxNamespaceNamespaceReplicationInducingAPIsRPSPerInstance is a per host/per namespace RPS limit for
namespace replication inducing APIs (e.g. RegisterNamespace, UpdateNamespace, UpdateWorkerBuildIdCompatibility).
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	FrontendMaxNamespaceVisibilityBurstRatioPerInstance = NewNamespaceFloatSetting(
		"frontend.namespaceBurstRatio.visibility",
		1,
		`FrontendMaxNamespaceVisibilityBurstRatioPerInstance is namespace burst limit for visibility APIs as a ratio of
namespace visibility RPS. The RPS used here will be the effective RPS from global and per-instance limits. This
config is EXPERIMENTAL and may be changed or removed in a later release. The value must be 1 or higher.`,
	)
	FrontendMaxNamespaceNamespaceReplicationInducingAPIsBurstRatioPerInstance = NewNamespaceFloatSetting(
		"frontend.namespaceBurstRatio.namespaceReplicationInducingAPIs",
		10,
		`FrontendMaxNamespaceNamespaceReplicationInducingAPIsBurstRatioPerInstance is a per host/per namespace burst limit for
namespace replication inducing APIs (e.g. RegisterNamespace, UpdateNamespace, UpdateWorkerBuildIdCompatibility)
as a ratio of namespace ReplicationInducingAPIs RPS. The RPS used here will be the effective RPS from global and
per-instance limits. This config is EXPERIMENTAL and may be changed or removed in a later release. The value must
be 1 or higher.`,
	)
	FrontendGlobalNamespaceRPS = NewNamespaceIntSetting(
		"frontend.globalNamespaceRPS",
		0,
		`FrontendGlobalNamespaceRPS is workflow namespace rate limit per second for the whole cluster.
The limit is evenly distributed among available frontend service instances.
If this is set, it overwrites per instance limit "frontend.namespaceRPS".`,
	)
	InternalFrontendGlobalNamespaceRPS = NewNamespaceIntSetting(
		"internal-frontend.globalNamespaceRPS",
		0,
		`InternalFrontendGlobalNamespaceRPS is workflow namespace rate limit per second across
all internal-frontends.`,
	)
	FrontendGlobalNamespaceVisibilityRPS = NewNamespaceIntSetting(
		"frontend.globalNamespaceRPS.visibility",
		0,
		`FrontendGlobalNamespaceVisibilityRPS is workflow namespace rate limit per second for the whole cluster for visibility API.
The limit is evenly distributed among available frontend service instances.
If this is set, it overwrites per instance limit "frontend.namespaceRPS.visibility".
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	FrontendGlobalNamespaceNamespaceReplicationInducingAPIsRPS = NewNamespaceIntSetting(
		"frontend.globalNamespaceRPS.namespaceReplicationInducingAPIs",
		10,
		`FrontendGlobalNamespaceNamespaceReplicationInducingAPIsRPS is a cluster global, per namespace RPS limit for
namespace replication inducing APIs (e.g. RegisterNamespace, UpdateNamespace, UpdateWorkerBuildIdCompatibility).
The limit is evenly distributed among available frontend service instances.
If this is set, it overwrites the per instance limit configured with
"frontend.namespaceRPS.namespaceReplicationInducingAPIs".
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	InternalFrontendGlobalNamespaceVisibilityRPS = NewNamespaceIntSetting(
		"internal-frontend.globalNamespaceRPS.visibility",
		0,
		`InternalFrontendGlobalNamespaceVisibilityRPS is workflow namespace rate limit per second
across all internal-frontends.
This config is EXPERIMENTAL and may be changed or removed in a later release.`,
	)
	FrontendThrottledLogRPS = NewGlobalIntSetting(
		"frontend.throttledLogRPS",
		20,
		`FrontendThrottledLogRPS is the rate limit on number of log messages emitted per second for throttled logger`,
	)
	FrontendShutdownDrainDuration = NewGlobalDurationSetting(
		"frontend.shutdownDrainDuration",
		0*time.Second,
		`FrontendShutdownDrainDuration is the duration of traffic drain during shutdown`,
	)
	FrontendShutdownFailHealthCheckDuration = NewGlobalDurationSetting(
		"frontend.shutdownFailHealthCheckDuration",
		0*time.Second,
		`FrontendShutdownFailHealthCheckDuration is the duration of shutdown failure detection`,
	)
	FrontendMaxBadBinaries = NewNamespaceIntSetting(
		"frontend.maxBadBinaries",
		10,
		`FrontendMaxBadBinaries is the max number of bad binaries in namespace config`,
	)
	FrontendMaskInternalErrorDetails = NewNamespaceBoolSetting(
		"frontend.maskInternalErrorDetails",
		true,
		`MaskInternalOrUnknownErrors is whether to replace internal/unknown errors with default error`,
	)
	HistoryHostErrorPercentage = NewGlobalFloatSetting(
		"frontend.historyHostErrorPercentage",
		0.5,
		`HistoryHostErrorPercentage is the proportion of hosts that are unhealthy through observation external to the host and internal host health checks`,
	)
	HistoryHostSelfErrorProportion = NewGlobalFloatSetting(
		"frontend.historyHostSelfErrorProportion",
		0.05,
		`HistoryHostStartingProportion is the proportion of hosts that have marked themselves as not ready -- this could due to waiting to acquire all shards on startup, or an internal health check failure`,
	)
	SendRawWorkflowHistory = NewNamespaceBoolSetting(
		"frontend.sendRawWorkflowHistory",
		false,
		`SendRawWorkflowHistory is whether to enable raw history retrieving`,
	)
	SearchAttributesNumberOfKeysLimit = NewNamespaceIntSetting(
		"frontend.searchAttributesNumberOfKeysLimit",
		100,
		`SearchAttributesNumberOfKeysLimit is the limit of number of keys`,
	)
	SearchAttributesSizeOfValueLimit = NewNamespaceIntSetting(
		"frontend.searchAttributesSizeOfValueLimit",
		2*1024,
		`SearchAttributesSizeOfValueLimit is the size limit of each value`,
	)
	SearchAttributesTotalSizeLimit = NewNamespaceIntSetting(
		"frontend.searchAttributesTotalSizeLimit",
		40*1024,
		`SearchAttributesTotalSizeLimit is the size limit of the whole map`,
	)
	VisibilityArchivalQueryMaxPageSize = NewGlobalIntSetting(
		"frontend.visibilityArchivalQueryMaxPageSize",
		10000,
		`VisibilityArchivalQueryMaxPageSize is the maximum page size for a visibility archival query`,
	)
	EnableServerVersionCheck = NewGlobalBoolSetting(
		"frontend.enableServerVersionCheck",
		os.Getenv("TEMPORAL_VERSION_CHECK_DISABLED") == "",
		`EnableServerVersionCheck is a flag that controls whether or not periodic version checking is enabled`,
	)
	EnableTokenNamespaceEnforcement = NewGlobalBoolSetting(
		"frontend.enableTokenNamespaceEnforcement",
		true,
		`EnableTokenNamespaceEnforcement enables enforcement that namespace in completion token matches namespace of the request`,
	)
	DisableListVisibilityByFilter = NewNamespaceBoolSetting(
		"frontend.disableListVisibilityByFilter",
		false,
		`DisableListVisibilityByFilter is config to disable list open/close workflow using filter`,
	)
	KeepAliveMinTime = NewGlobalDurationSetting(
		"frontend.keepAliveMinTime",
		10*time.Second,
		`KeepAliveMinTime is the minimum amount of time a client should wait before sending a keepalive ping.`,
	)
	KeepAlivePermitWithoutStream = NewGlobalBoolSetting(
		"frontend.keepAlivePermitWithoutStream",
		true,
		`KeepAlivePermitWithoutStream If true, server allows keepalive pings even when there are no active
streams(RPCs). If false, and client sends ping when there are no active
streams, server will send GOAWAY and close the connection.`,
	)
	KeepAliveMaxConnectionIdle = NewGlobalDurationSetting(
		"frontend.keepAliveMaxConnectionIdle",
		2*time.Minute,
		`KeepAliveMaxConnectionIdle is a duration for the amount of time after which an
idle connection would be closed by sending a GoAway. Idleness duration is
defined since the most recent time the number of outstanding RPCs became
zero or the connection establishment.`,
	)
	KeepAliveMaxConnectionAge = NewGlobalDurationSetting(
		"frontend.keepAliveMaxConnectionAge",
		5*time.Minute,
		`KeepAliveMaxConnectionAge is a duration for the maximum amount of time a
connection may exist before it will be closed by sending a GoAway. A
random jitter of +/-10% will be added to MaxConnectionAge to spread out
connection storms.`,
	)
	KeepAliveMaxConnectionAgeGrace = NewGlobalDurationSetting(
		"frontend.keepAliveMaxConnectionAgeGrace",
		70*time.Second,
		`KeepAliveMaxConnectionAgeGrace is an additive period after MaxConnectionAge after
which the connection will be forcibly closed.`,
	)
	KeepAliveTime = NewGlobalDurationSetting(
		"frontend.keepAliveTime",
		1*time.Minute,
		`KeepAliveTime After a duration of this time if the server doesn't see any activity it
pings the client to see if the transport is still alive.
If set below 1s, a minimum value of 1s will be used instead.`,
	)
	KeepAliveTimeout = NewGlobalDurationSetting(
		"frontend.keepAliveTimeout",
		10*time.Second,
		`KeepAliveTimeout After having pinged for keepalive check, the server waits for a duration
of Timeout and if no activity is seen even after that the connection is closed.`,
	)
	FrontendEnableSchedules = NewNamespaceBoolSetting(
		"frontend.enableSchedules",
		true,
		`FrontendEnableSchedules enables schedule-related RPCs in the frontend`,
	)
	// [cleanup-wv-pre-release]
	EnableDeployments = NewNamespaceBoolSetting(
		"system.enableDeployments",
		false,
		`EnableDeployments enables deployments (deprecated versioning v3 pre-release) in all services,
including deployment-related RPCs in the frontend, deployment entity workflows in the worker,
and deployment interaction in matching and history.`,
	)
	EnableDeploymentVersions = NewNamespaceBoolSetting(
		"system.enableDeploymentVersions",
		true,
		`EnableDeploymentVersions enables deployment versions (versioning v3) in all services,
including deployment-related RPCs in the frontend, deployment version entity workflows in the worker,
and deployment interaction in matching and history.`,
	)
	EnableNexus = NewGlobalBoolSetting(
		"system.enableNexus",
		true,
		`Toggles all Nexus functionality on the server. Note that toggling this requires restarting server hosts for it
		to take effect.`,
	)

	AllowDeleteNamespaceIfNexusEndpointTarget = NewGlobalBoolSetting(
		"frontend.allowDeleteNamespaceIfNexusEndpointTarget",
		false,
		`If set to true (default is false), namespaces that are Nexus endpoint targets will be prevented from being deleted.`,
	)

	RefreshNexusEndpointsLongPollTimeout = NewGlobalDurationSetting(
		"system.refreshNexusEndpointsLongPollTimeout",
		5*time.Minute,
		`RefreshNexusEndpointsLongPollTimeout is the maximum duration of background long poll requests to update Nexus endpoints.`,
	)
	RefreshNexusEndpointsMinWait = NewGlobalDurationSetting(
		"system.refreshNexusEndpointsMinWait",
		1*time.Second,
		`RefreshNexusEndpointsMinWait is the minimum wait time between background long poll requests to update Nexus endpoints.`,
	)
	NexusReadThroughCacheSize = NewGlobalIntSetting(
		"system.nexusReadThroughCacheSize",
		100,
		`The size of the Nexus endpoint registry's readthrough LRU cache - the cache is a secondary cache and is only
used when the first cache layer has a miss. Requires server restart for change to be applied.`,
	)
	NexusReadThroughCacheTTL = NewGlobalDurationSetting(
		"system.nexusReadThroughCacheTTL",
		30*time.Second,
		`The TTL of the Nexus endpoint registry's readthrough LRU cache - the cache is a secondary cache and is only
used when the first cache layer has a miss. Requires server restart for change to be applied.`,
	)
	FrontendNexusRequestHeadersBlacklist = NewGlobalTypedSetting(
		"frontend.nexusRequestHeadersBlacklist",
		[]string(nil),
		`Nexus request headers to be removed before being sent to a user handler.
Wildcards (*) are expanded to allow any substring. By default blacklist is empty.`,
	)
	FrontendCallbackURLMaxLength = NewNamespaceIntSetting(
		"frontend.callbackURLMaxLength",
		1000,
		`FrontendCallbackURLMaxLength is the maximum length of callback URL`,
	)
	FrontendCallbackHeaderMaxSize = NewNamespaceIntSetting(
		"frontend.callbackHeaderMaxLength",
		8*1024,
		`FrontendCallbackHeaderMaxSize is the maximum accumulated size of callback header keys and values`,
	)
	MaxCallbacksPerWorkflow = NewNamespaceIntSetting(
		"system.maxCallbacksPerWorkflow",
		32,
		`MaxCallbacksPerWorkflow is the maximum number of callbacks that can be attached to a workflow.`,
	)
	FrontendLinkMaxSize = NewNamespaceIntSetting(
		"frontend.linkMaxSize",
		4000, // Links may include a workflow ID and namespace name, both of which are limited to a length of 1000.
		`Maximum size in bytes of temporal.api.common.v1.Link object in an API request.`,
	)
	FrontendMaxLinksPerRequest = NewNamespaceIntSetting(
		"frontend.maxlinksPerRequest",
		10,
		`Maximum number of links allowed to be attached via a single API request.`,
	)
	FrontendMaxConcurrentBatchOperationPerNamespace = NewNamespaceIntSetting(
		"frontend.MaxConcurrentBatchOperationPerNamespace",
		1,
		`FrontendMaxConcurrentBatchOperationPerNamespace is the max concurrent batch operation job count per namespace`,
	)
	FrontendMaxExecutionCountBatchOperationPerNamespace = NewNamespaceIntSetting(
		"frontend.MaxExecutionCountBatchOperationPerNamespace",
		1000,
		`FrontendMaxExecutionCountBatchOperationPerNamespace is the max execution count batch operation supports per namespace`,
	)
	FrontendEnableBatcher = NewNamespaceBoolSetting(
		"frontend.enableBatcher",
		true,
		`FrontendEnableBatcher enables batcher-related RPCs in the frontend`,
	)

	FrontendEnableUpdateWorkflowExecution = NewNamespaceBoolSetting(
		"frontend.enableUpdateWorkflowExecution",
		true,
		`FrontendEnableUpdateWorkflowExecution enables UpdateWorkflowExecution API in the frontend.`,
	)

	FrontendEnableExecuteMultiOperation = NewNamespaceBoolSetting(
		"frontend.enableExecuteMultiOperation",
		true,
		`FrontendEnableExecuteMultiOperation enables the ExecuteMultiOperation API in the frontend.
The API is under active development.`,
	)

	FrontendEnableUpdateWorkflowExecutionAsyncAccepted = NewNamespaceBoolSetting(
		"frontend.enableUpdateWorkflowExecutionAsyncAccepted",
		true,
		`FrontendEnableUpdateWorkflowExecutionAsyncAccepted enables the UpdateWorkflowExecution API
to allow waiting on the "Accepted" lifecycle stage.`,
	)

	FrontendEnableWorkerVersioningDataAPIs = NewNamespaceBoolSetting(
		"frontend.workerVersioningDataAPIs",
		false,
		`FrontendEnableWorkerVersioningDataAPIs enables worker versioning data read / write APIs.`,
	)
	FrontendEnableWorkerVersioningWorkflowAPIs = NewNamespaceBoolSetting(
		"frontend.workerVersioningWorkflowAPIs",
		true,
		`FrontendEnableWorkerVersioningWorkflowAPIs enables worker versioning in workflow progress APIs.`,
	)
	FrontendEnableWorkerVersioningRuleAPIs = NewNamespaceBoolSetting(
		"frontend.workerVersioningRuleAPIs",
		false,
		`FrontendEnableWorkerVersioningRuleAPIs enables worker versioning in workflow progress APIs.`,
	)

	DeleteNamespaceDeleteActivityRPS = NewGlobalIntSetting(
		"frontend.deleteNamespaceDeleteActivityRPS",
		100,
		`DeleteNamespaceDeleteActivityRPS is an RPS per every parallel delete executions activity.
Total RPS is equal to DeleteNamespaceDeleteActivityRPS * DeleteNamespaceConcurrentDeleteExecutionsActivities.
Default value is 100. Despite starting with 'frontend.' this setting is used by a worker and can be changed while namespace is deleted.`,
	)
	DeleteNamespacePageSize = NewGlobalIntSetting(
		"frontend.deleteNamespaceDeletePageSize",
		1000,
		`DeleteNamespacePageSize is a page size to read executions from visibility for delete executions activity.
Default value is 1000. Read once before delete of specified namespace is started.`,
	)
	DeleteNamespacePagesPerExecution = NewGlobalIntSetting(
		"frontend.deleteNamespacePagesPerExecution",
		256,
		`DeleteNamespacePagesPerExecution is a number of pages before returning ContinueAsNew from delete executions activity.
Default value is 256. Read once before delete of specified namespace is started.`,
	)
	DeleteNamespaceConcurrentDeleteExecutionsActivities = NewGlobalIntSetting(
		"frontend.deleteNamespaceConcurrentDeleteExecutionsActivities",
		4,
		`DeleteNamespaceConcurrentDeleteExecutionsActivities is a number of concurrent delete executions activities.
Must be not greater than 256 and number of worker cores in the cluster.
Default is 4. Read once before delete of specified namespace is started.`,
	)
	DeleteNamespaceNamespaceDeleteDelay = NewGlobalDurationSetting(
		"frontend.deleteNamespaceNamespaceDeleteDelay",
		0*time.Hour,
		`DeleteNamespaceNamespaceDeleteDelay is a duration for how long namespace stays in database
after all namespace resources (i.e. workflow executions) are deleted.
Default is 0, means, namespace will be deleted immediately.`,
	)
	ProtectedNamespaces = NewGlobalTypedSetting(
		"worker.protectedNamespaces",
		([]string)(nil),
		`List of namespace names that can't be deleted.`,
	)

	// keys for matching

	MatchingRPS = NewGlobalIntSetting(
		"matching.rps",
		1200,
		`MatchingRPS is request rate per second for each matching host`,
	)
	MatchingPersistenceMaxQPS = NewGlobalIntSetting(
		"matching.persistenceMaxQPS",
		3000,
		`MatchingPersistenceMaxQPS is the max qps matching host can query DB`,
	)
	MatchingPersistenceGlobalMaxQPS = NewGlobalIntSetting(
		"matching.persistenceGlobalMaxQPS",
		0,
		`MatchingPersistenceGlobalMaxQPS is the max qps matching cluster can query DB`,
	)
	MatchingPersistenceNamespaceMaxQPS = NewNamespaceIntSetting(
		"matching.persistenceNamespaceMaxQPS",
		0,
		`MatchingPersistenceNamespaceMaxQPS is the max qps each namespace on matching host can query DB`,
	)
	MatchingPersistenceGlobalNamespaceMaxQPS = NewNamespaceIntSetting(
		"matching.persistenceGlobalNamespaceMaxQPS",
		0,
		`MatchingPersistenceNamespaceMaxQPS is the max qps each namespace in matching cluster can query DB`,
	)
	MatchingPersistenceDynamicRateLimitingParams = NewGlobalTypedSetting(
		"matching.persistenceDynamicRateLimitingParams",
		DefaultDynamicRateLimitingParams,
		`MatchingPersistenceDynamicRateLimitingParams is a struct that contains all adjustable dynamic rate limiting params.
Fields: Enabled, RefreshInterval, LatencyThreshold, ErrorThreshold, RateBackoffStepSize, RateIncreaseStepSize, RateMultiMin, RateMultiMax.
See DynamicRateLimitingParams comments for more details.`,
	)
	MatchingMinTaskThrottlingBurstSize = NewTaskQueueIntSetting(
		"matching.minTaskThrottlingBurstSize",
		1,
		`MatchingMinTaskThrottlingBurstSize is the minimum burst size for task queue throttling`,
	)
	MatchingGetTasksBatchSize = NewTaskQueueIntSetting(
		"matching.getTasksBatchSize",
		1000,
		`How many backlog tasks to read from persistence at once`,
	)
	MatchingGetTasksReloadAt = NewTaskQueueIntSetting(
		"matching.getTasksReloadAt",
		100,
		`Reload a batch of tasks when there are this many remaining. Must be less than MatchingGetTasksBatchSize. (Requires new matcher.)`,
	)
	MatchingLongPollExpirationInterval = NewTaskQueueDurationSetting(
		"matching.longPollExpirationInterval",
		time.Minute,
		`MatchingLongPollExpirationInterval is the long poll expiration interval in the matching service`,
	)
	// TODO(pri): old matcher cleanup
	MatchingSyncMatchWaitDuration = NewTaskQueueDurationSetting(
		"matching.syncMatchWaitDuration",
		200*time.Millisecond,
		`MatchingSyncMatchWaitDuration is to wait time for sync match`,
	)
	MatchingHistoryMaxPageSize = NewNamespaceIntSetting(
		"matching.historyMaxPageSize",
		primitives.GetHistoryMaxPageSize,
		`MatchingHistoryMaxPageSize is the maximum page size of history events returned on PollWorkflowTaskQueue requests`,
	)
	MatchingUpdateAckInterval = NewTaskQueueDurationSettingWithConstrainedDefault(
		"matching.updateAckInterval",
		[]TypedConstrainedValue[time.Duration]{
			// Use a longer default interval for the per-namespace internal worker queues.
			{
				Constraints: Constraints{
					TaskQueueName: primitives.PerNSWorkerTaskQueue,
				},
				Value: 5 * time.Minute,
			},
			// Default for everything else.
			{
				Value: 1 * time.Minute,
			},
		},
		`MatchingUpdateAckInterval is the interval for update ack`,
	)
	MatchingMaxTaskQueueIdleTime = NewTaskQueueDurationSetting(
		"matching.maxTaskQueueIdleTime",
		5*time.Minute,
		`MatchingMaxTaskQueueIdleTime is the time after which an idle task queue will be unloaded.
Note: this should be greater than matching.longPollExpirationInterval and matching.getUserDataLongPollTimeout.`,
	)
	MatchingOutstandingTaskAppendsThreshold = NewTaskQueueIntSetting(
		"matching.outstandingTaskAppendsThreshold",
		250,
		`MatchingOutstandingTaskAppendsThreshold is the threshold for outstanding task appends`,
	)
	MatchingMaxTaskBatchSize = NewTaskQueueIntSetting(
		"matching.maxTaskBatchSize",
		100,
		`MatchingMaxTaskBatchSize is max batch size for task writer`,
	)
	MatchingMaxTaskDeleteBatchSize = NewTaskQueueIntSetting(
		"matching.maxTaskDeleteBatchSize",
		100,
		`MatchingMaxTaskDeleteBatchSize is the max batch size for range deletion of tasks`,
	)
	MatchingTaskDeleteInterval = NewTaskQueueDurationSetting(
		"matching.taskDeleteInterval",
		15*time.Second,
		`MatchingTaskDeleteInterval is the minimum interval between task range deletions`,
	)
	MatchingThrottledLogRPS = NewGlobalIntSetting(
		"matching.throttledLogRPS",
		20,
		`MatchingThrottledLogRPS is the rate limit on number of log messages emitted per second for throttled logger`,
	)
	MatchingNumTaskqueueWritePartitions = NewTaskQueueIntSettingWithConstrainedDefault(
		"matching.numTaskqueueWritePartitions",
		defaultNumTaskQueuePartitions,
		`MatchingNumTaskqueueWritePartitions is the number of write partitions for a task queue`,
	)
	MatchingNumTaskqueueReadPartitions = NewTaskQueueIntSettingWithConstrainedDefault(
		"matching.numTaskqueueReadPartitions",
		defaultNumTaskQueuePartitions,
		`MatchingNumTaskqueueReadPartitions is the number of read partitions for a task queue`,
	)
	MetricsBreakdownByTaskQueue = NewTaskQueueBoolSetting(
		"metrics.breakdownByTaskQueue",
		true,
		`MetricsBreakdownByTaskQueue determines if the 'taskqueue' tag in Matching and History metrics should
contain the actual TQ name or a generic __omitted__ value. Disable this option if the cardinality is too high for your
observability stack. Disabling this option will disable all the per-Task Queue gauges such as backlog lag, count, and age.`,
	)
	MetricsBreakdownByPartition = NewTaskQueueBoolSetting(
		"metrics.breakdownByPartition",
		true,
		`MetricsBreakdownByPartition determines if the 'partition' tag in Matching metrics should
contain the actual normal partition ID or a generic __normal__ value. Regardless of this config, the tag value for sticky
queues will be "__sticky__". Disable this option if the partition cardinality is too high for your
observability stack. Disabling this option will disable all the per-Task Queue gauges such as backlog lag, count, and age.`,
	)
	MetricsBreakdownByBuildID = NewTaskQueueBoolSetting(
		"metrics.breakdownByBuildID",
		true,
		`MetricsBreakdownByBuildID determines if the 'worker-build-id' tag in Matching metrics should
contain the actual Build ID or a generic "__versioned__" value. Regardless of this config, the tag value for unversioned
queues will be "__unversioned__". Disable this option if the Build ID cardinality is too high for your
observability stack. Disabling this option will disable all the per-Task Queue gauges such as backlog lag, count, and age
for VERSIONED queues.`,
	)
	MatchingForwarderMaxOutstandingPolls = NewTaskQueueIntSetting(
		"matching.forwarderMaxOutstandingPolls",
		1,
		`MatchingForwarderMaxOutstandingPolls is the max number of inflight polls from the forwarder`,
	)
	MatchingForwarderMaxOutstandingTasks = NewTaskQueueIntSetting(
		"matching.forwarderMaxOutstandingTasks",
		1,
		`MatchingForwarderMaxOutstandingTasks is the max number of inflight addTask/queryTask from the forwarder`,
	)
	MatchingForwarderMaxRatePerSecond = NewTaskQueueFloatSetting(
		"matching.forwarderMaxRatePerSecond",
		10,
		`MatchingForwarderMaxRatePerSecond is the max rate at which add/query can be forwarded`,
	)
	MatchingForwarderMaxChildrenPerNode = NewTaskQueueIntSetting(
		"matching.forwarderMaxChildrenPerNode",
		20,
		`MatchingForwarderMaxChildrenPerNode is the max number of children per node in the task queue partition tree`,
	)
	MatchingAlignMembershipChange = NewGlobalDurationSetting(
		"matching.alignMembershipChange",
		0*time.Second,
		`MatchingAlignMembershipChange is a duration to align matching's membership changes to.
This can help reduce effects of task queue movement.`,
	)
	MatchingShutdownDrainDuration = NewGlobalDurationSetting(
		"matching.shutdownDrainDuration",
		0*time.Second,
		`MatchingShutdownDrainDuration is the duration of traffic drain during shutdown`,
	)
	MatchingGetUserDataLongPollTimeout = NewGlobalDurationSetting(
		"matching.getUserDataLongPollTimeout",
		5*time.Minute-10*time.Second,
		`MatchingGetUserDataLongPollTimeout is the max length of long polls for GetUserData calls between partitions.`,
	)
	MatchingGetUserDataRefresh = NewGlobalDurationSetting(
		"matching.getUserDataRefresh",
		5*time.Minute,
		`MatchingGetUserDataRefresh is how often the user data owner refreshes data from persistence.`,
	)
	MatchingBacklogNegligibleAge = NewTaskQueueDurationSetting(
		"matching.backlogNegligibleAge",
		5*time.Second,
		`MatchingBacklogNegligibleAge if the head of backlog gets older than this we stop sync match and
forwarding to ensure more equal dispatch order among partitions.`,
	)
	MatchingMaxWaitForPollerBeforeFwd = NewTaskQueueDurationSetting(
		"matching.maxWaitForPollerBeforeFwd",
		200*time.Millisecond,
		`MatchingMaxWaitForPollerBeforeFwd in presence of a non-negligible backlog, we resume forwarding tasks if the
duration since last poll exceeds this threshold.`,
	)
	QueryPollerUnavailableWindow = NewGlobalDurationSetting(
		"matching.queryPollerUnavailableWindow",
		20*time.Second,
		`QueryPollerUnavailableWindow WF Queries are rejected after a while if no poller has been seen within the window`,
	)
	MatchingListNexusEndpointsLongPollTimeout = NewGlobalDurationSetting(
		"matching.listNexusEndpointsLongPollTimeout",
		5*time.Minute-10*time.Second,
		`MatchingListNexusEndpointsLongPollTimeout is the max length of long polls for ListNexusEndpoints calls.`,
	)
	MatchingNexusEndpointsRefreshInterval = NewGlobalDurationSetting(
		"matching.nexusEndpointsRefreshInterval",
		10*time.Second,
		`Time to wait between calls to check that the in-memory view of Nexus endpoints matches the persisted state.`,
	)
	MatchingMembershipUnloadDelay = NewGlobalDurationSetting(
		"matching.membershipUnloadDelay",
		500*time.Millisecond,
		`MatchingMembershipUnloadDelay is how long to wait to re-confirm loss of ownership before unloading a task queue.
Set to zero to disable proactive unload.`,
	)
	MatchingQueryWorkflowTaskTimeoutLogRate = NewTaskQueueFloatSetting(
		"matching.queryWorkflowTaskTimeoutLogRate",
		0.0,
		`MatchingQueryWorkflowTaskTimeoutLogRate defines the sampling rate for logs when a query workflow task times out. Since
these log lines can be noisy, we want to be able to turn on and sample selectively for each affected namespace.`,
	)
	TaskQueueInfoByBuildIdTTL = NewTaskQueueDurationSetting(
		"matching.TaskQueueInfoByBuildIdTTL",
		5*time.Second,
		`TaskQueueInfoByBuildIdTTL serves as a TTL for the cache holding DescribeTaskQueue partition results`,
	)
	MatchingMaxTaskQueuesInDeployment = NewNamespaceIntSetting(
		"matching.maxTaskQueuesInDeployment",
		1000,
		`MatchingMaxTaskQueuesInDeployment represents the maximum number of task-queues that can be registed in a single deployment`,
	)
	MatchingMaxDeployments = NewNamespaceIntSetting(
		"matching.maxDeployments",
		100,
		`MatchingMaxDeployments represents the maximum number of worker deployments that can be registered in a single namespace`,
	)
	MatchingMaxVersionsInDeployment = NewNamespaceIntSetting(
		"matching.maxVersionsInDeployment",
		100,
		`MatchingMaxVersionsInDeployment represents the maximum number of versions that can be registered in a single worker deployment`,
	)
	MatchingMaxTaskQueuesInDeploymentVersion = NewNamespaceIntSetting(
		"matching.maxTaskQueuesInDeploymentVersion",
		100,
		`MatchingMaxTaskQueuesInDeployment represents the maximum number of task-queues that can be registered in a single worker deployment version`,
	)
	MatchingPollerScalingBacklogAgeScaleUp = NewTaskQueueDurationSetting(
		"matching.pollerScalingMinimumBacklog",
		200*time.Millisecond,
		`MatchingPollerScalingBacklogAgeScaleUp is the minimum backlog age that must be accumulated before 
a decision to scale up the number of pollers will be issued`,
	)
	MatchingPollerScalingWaitTime = NewTaskQueueDurationSetting(
		"matching.pollerScalingWaitTime",
		1*time.Second,
		`MatchingPollerScalingWaitTime is the duration a sync-matched poller must exceed before
a decision to scale down the number of pollers will be issued`,
	)
	MatchingPollerScalingDecisionsPerSecond = NewTaskQueueFloatSetting(
		"matching.pollerScalingDecisionsPerSecond",
		10,
		`MatchingPollerScalingDecisionsPerSecond is the maximum number of scaling decisions that will be issued per
second per poller by one physical queue manager`,
	)
	MatchingUseNewMatcher = NewTaskQueueBoolSetting(
		"matching.useNewMatcher",
		false,
		`Use priority-enabled TaskMatcher`,
	)
	MatchingPriorityLevels = NewTaskQueueIntSetting(
		"matching.priorityLevels",
		5,
		`Number of simple priority levels (requires new matcher)`,
	)
	MatchingBacklogTaskForwardTimeout = NewTaskQueueDurationSetting(
		"matching.backlogTaskForwardTimeout",
		60*time.Second,
		`Timeout for forwarded backlog task (requires new matcher)`,
	)

	// keys for history

	EnableReplicationStream = NewGlobalBoolSetting(
		"history.enableReplicationStream",
		true,
		`EnableReplicationStream turn on replication stream`,
	)
	EnableHistoryReplicationDLQV2 = NewGlobalBoolSetting(
		"history.enableHistoryReplicationDLQV2",
		true,
		`EnableHistoryReplicationDLQV2 switches to the DLQ v2 implementation for history replication. See details in
[go.temporal.io/server/common/persistence.QueueV2]`,
	)

	HistoryRPS = NewGlobalIntSetting(
		"history.rps",
		3000,
		`HistoryRPS is request rate per second for each history host`,
	)
	HistoryPersistenceMaxQPS = NewGlobalIntSetting(
		"history.persistenceMaxQPS",
		9000,
		`HistoryPersistenceMaxQPS is the max qps history host can query DB`,
	)
	HistoryPersistenceGlobalMaxQPS = NewGlobalIntSetting(
		"history.persistenceGlobalMaxQPS",
		0,
		`HistoryPersistenceGlobalMaxQPS is the max qps history cluster can query DB`,
	)
	HistoryPersistenceNamespaceMaxQPS = NewNamespaceIntSetting(
		"history.persistenceNamespaceMaxQPS",
		0,
		`HistoryPersistenceNamespaceMaxQPS is the max qps each namespace on history host can query DB
If value less or equal to 0, will fall back to HistoryPersistenceMaxQPS`,
	)
	HistoryPersistenceGlobalNamespaceMaxQPS = NewNamespaceIntSetting(
		"history.persistenceGlobalNamespaceMaxQPS",
		0,
		`HistoryPersistenceNamespaceMaxQPS is the max qps each namespace in history cluster can query DB`,
	)
	HistoryPersistencePerShardNamespaceMaxQPS = NewNamespaceIntSetting(
		"history.persistencePerShardNamespaceMaxQPS",
		0,
		`HistoryPersistencePerShardNamespaceMaxQPS is the max qps each namespace on a shard can query DB`,
	)
	HistoryPersistenceDynamicRateLimitingParams = NewGlobalTypedSetting(
		"history.persistenceDynamicRateLimitingParams",
		DefaultDynamicRateLimitingParams,
		`HistoryPersistenceDynamicRateLimitingParams is a struct that contains all adjustable dynamic rate limiting params.
Fields: Enabled, RefreshInterval, LatencyThreshold, ErrorThreshold, RateBackoffStepSize, RateIncreaseStepSize, RateMultiMin, RateMultiMax.
See DynamicRateLimitingParams comments for more details.`,
	)
	HistoryLongPollExpirationInterval = NewNamespaceDurationSetting(
		"history.longPollExpirationInterval",
		time.Second*20,
		`HistoryLongPollExpirationInterval is the long poll expiration interval in the history service`,
	)
	HistoryCacheSizeBasedLimit = NewGlobalBoolSetting(
		"history.cacheSizeBasedLimit",
		false,
		`HistoryCacheSizeBasedLimit if true, size of the history cache will be limited by HistoryCacheMaxSizeBytes
and HistoryCacheHostLevelMaxSizeBytes. Otherwise, entry count in the history cache will be limited by
HistoryCacheMaxSize and HistoryCacheHostLevelMaxSize.`,
	)
	HistoryCacheTTL = NewGlobalDurationSetting(
		"history.cacheTTL",
		time.Hour,
		`HistoryCacheTTL is TTL of history cache`,
	)
	HistoryCacheNonUserContextLockTimeout = NewGlobalDurationSetting(
		"history.cacheNonUserContextLockTimeout",
		500*time.Millisecond,
		`HistoryCacheNonUserContextLockTimeout controls how long non-user call (callerType != API or Operator)
will wait on workflow lock acquisition. Requires service restart to take effect.`,
	)
	HistoryCacheHostLevelMaxSize = NewGlobalIntSetting(
		"history.hostLevelCacheMaxSize",
		128000,
		`HistoryCacheHostLevelMaxSize is the maximum number of entries in the host level history cache`,
	)
	HistoryCacheHostLevelMaxSizeBytes = NewGlobalIntSetting(
		"history.hostLevelCacheMaxSizeBytes",
		256000*4*1024,
		`HistoryCacheHostLevelMaxSizeBytes is the maximum size of the host level history cache. This is only used if
HistoryCacheSizeBasedLimit is set to true.`,
	)
	EnableWorkflowExecutionTimeoutTimer = NewGlobalBoolSetting(
		"history.enableWorkflowExecutionTimeoutTimer",
		true,
		`EnableWorkflowExecutionTimeoutTimer controls whether to enable the new logic for generating a workflow execution
timeout timer when execution timeout is specified when starting a workflow.`,
	)
	EnableUpdateWorkflowModeIgnoreCurrent = NewGlobalBoolSetting(
		"history.enableUpdateWorkflowModeIgnoreCurrent",
		true,
		`EnableUpdateWorkflowModeIgnoreCurrent controls whether to enable the new logic for updating closed workflow execution
by mutation using UpdateWorkflowModeIgnoreCurrent`,
	)
	EnableTransitionHistory = NewGlobalBoolSetting(
		"history.enableTransitionHistory",
		false,
		`EnableTransitionHistory controls whether to enable the new logic for recording the history for each state transition.
This feature is still under development and should NOT be enabled.`,
	)
	HistoryStartupMembershipJoinDelay = NewGlobalDurationSetting(
		"history.startupMembershipJoinDelay",
		0*time.Second,
		`HistoryStartupMembershipJoinDelay is the duration a history instance waits
before joining membership after starting.`,
	)
	HistoryAlignMembershipChange = NewGlobalDurationSetting(
		"history.alignMembershipChange",
		0*time.Second,
		`HistoryAlignMembershipChange is a duration to align history's membership changes to.
This can help reduce effects of shard movement.`,
	)
	HistoryShutdownDrainDuration = NewGlobalDurationSetting(
		"history.shutdownDrainDuration",
		0*time.Second,
		`HistoryShutdownDrainDuration is the duration of traffic drain during shutdown`,
	)
	XDCCacheMaxSizeBytes = NewGlobalIntSetting(
		"history.xdcCacheMaxSizeBytes",
		8*1024*1024,
		`XDCCacheMaxSizeBytes is max size of events cache in bytes`,
	)
	EventsCacheMaxSizeBytes = NewGlobalIntSetting(
		"history.eventsCacheMaxSizeBytes",
		512*1024,
		`EventsCacheMaxSizeBytes is max size of the shard level events cache in bytes`,
	)
	EventsHostLevelCacheMaxSizeBytes = NewGlobalIntSetting(
		"history.eventsHostLevelCacheMaxSizeBytes",
		512*512*1024,
		`EventsHostLevelCacheMaxSizeBytes is max size of the host level events cache in bytes`,
	)
	EventsCacheTTL = NewGlobalDurationSetting(
		"history.eventsCacheTTL",
		time.Hour,
		`EventsCacheTTL is TTL of events cache`,
	)
	EnableHostLevelEventsCache = NewGlobalBoolSetting(
		"history.enableHostLevelEventsCache",
		false,
		`EnableHostLevelEventsCache controls if the events cache is host level`,
	)
	AcquireShardInterval = NewGlobalDurationSetting(
		"history.acquireShardInterval",
		time.Minute,
		`AcquireShardInterval is interval that timer used to acquire shard`,
	)
	AcquireShardConcurrency = NewGlobalIntSetting(
		"history.acquireShardConcurrency",
		10,
		`AcquireShardConcurrency is number of goroutines that can be used to acquire shards in the shard controller.`,
	)
	ShardLingerOwnershipCheckQPS = NewGlobalIntSetting(
		"history.shardLingerOwnershipCheckQPS",
		4,
		`ShardLingerOwnershipCheckQPS is the frequency to perform shard ownership
checks while a shard is lingering.`,
	)
	ShardLingerTimeLimit = NewGlobalDurationSetting(
		"history.shardLingerTimeLimit",
		0,
		`ShardLingerTimeLimit configures if and for how long the shard controller
will temporarily delay closing shards after a membership update, awaiting a
shard ownership lost error from persistence. If set to zero, shards will not delay closing.
Do NOT use non-zero value with persistence layers that are missing AssertShardOwnership support.`,
	)
	ShardFinalizerTimeout = NewGlobalDurationSetting(
		"history.shardFinalizerTimeout",
		2*time.Second,
		`ShardFinalizerTimeout configures if and for how long the shard will attempt
to cleanup any of its associated data, such as workflow contexts. If set to zero, the finalizer is disabled.`,
	)
	HistoryClientOwnershipCachingEnabled = NewGlobalBoolSetting(
		"history.clientOwnershipCachingEnabled",
		false,
		`HistoryClientOwnershipCachingEnabled configures if history clients try to cache
shard ownership information, instead of checking membership for each request.
Only inspected when an instance first creates a history client, so changes
to this require a restart to take effect.`,
	)
	HistoryClientOwnershipCachingStaleTTL = NewGlobalDurationSetting(
		"history.clientOwnershipCachingUnusedTTL",
		30*time.Second,
		`HistoryClientOwnershipCachingStaleTTL, if non-zero, configures the TTL
for cached shard ownership entries after a membership update.`,
	)
	ShardIOConcurrency = NewGlobalIntSetting(
		"history.shardIOConcurrency",
		1,
		`ShardIOConcurrency controls the concurrency of persistence operations in shard context`,
	)
	ShardIOTimeout = NewGlobalDurationSetting(
		"history.shardIOTimeout",
		5*time.Second*debug.TimeoutMultiplier,
		`ShardIOTimeout sets the timeout for persistence operations in the shard context`,
	)
	StandbyClusterDelay = NewGlobalDurationSetting(
		"history.standbyClusterDelay",
		5*time.Minute,
		`StandbyClusterDelay is the artificial delay added to standby cluster's view of active cluster's time`,
	)
	StandbyTaskMissingEventsResendDelay = NewTaskTypeDurationSetting(
		"history.standbyTaskMissingEventsResendDelay",
		10*time.Minute,
		`StandbyTaskMissingEventsResendDelay is the amount of time standby cluster's will wait (if events are missing)
before calling remote for missing events`,
	)
	StandbyTaskMissingEventsDiscardDelay = NewTaskTypeDurationSetting(
		"history.standbyTaskMissingEventsDiscardDelay",
		15*time.Minute,
		`StandbyTaskMissingEventsDiscardDelay is the amount of time standby cluster's will wait (if events are missing)
before discarding the task`,
	)
	QueuePendingTaskCriticalCount = NewGlobalIntSetting(
		"history.queuePendingTaskCriticalCount",
		9000,
		`Max number of pending tasks in a history queue before triggering slice splitting and unloading.
NOTE: The outbound queue has a separate configuration: outboundQueuePendingTaskCriticalCount.`,
	)
	QueueReaderStuckCriticalAttempts = NewGlobalIntSetting(
		"history.queueReaderStuckCriticalAttempts",
		3,
		`QueueReaderStuckCriticalAttempts is the max number of task loading attempts for a certain task range
before that task range is split into a separate slice to unblock loading for later range.
currently only work for scheduled queues and the task range is 1s.`,
	)
	QueueCriticalSlicesCount = NewGlobalIntSetting(
		"history.queueCriticalSlicesCount",
		50,
		`QueueCriticalSlicesCount is the max number of slices in one queue
before force compacting slices`,
	)
	QueuePendingTaskMaxCount = NewGlobalIntSetting(
		"history.queuePendingTasksMaxCount",
		10000,
		`The max number of task pending tasks in a history queue before stopping loading new tasks into memory. This
limit is in addition to queuePendingTaskCriticalCount which controls when to unload already loaded tasks but doesn't
prevent loading new tasks. Ideally this max count limit should not be hit and task unloading should happen once critical
count is exceeded. But since queue action is async, we need this hard limit.
NOTE: The outbound queue has a separate configuration: outboundQueuePendingTaskMaxCount.
`,
	)
	QueueMaxPredicateSize = NewGlobalIntSetting(
		"history.queueMaxPredicateSize",
		0,
		`The max size of the multi-cursor predicate structure stored in the shard info record. 0 is considered
unlimited. When the predicate size is surpassed for a given scope, the predicate is converted to a universal predicate,
which causes all tasks in the scope's range to eventually be reprocessed without applying any filtering logic.
NOTE: The outbound queue has a separate configuration: outboundQueueMaxPredicateSize.
`,
	)

	TaskSchedulerEnableRateLimiter = NewGlobalBoolSetting(
		"history.taskSchedulerEnableRateLimiter",
		false,
		`TaskSchedulerEnableRateLimiter indicates if task scheduler rate limiter should be enabled`,
	)
	TaskSchedulerEnableRateLimiterShadowMode = NewGlobalBoolSetting(
		"history.taskSchedulerEnableRateLimiterShadowMode",
		true,
		`TaskSchedulerEnableRateLimiterShadowMode indicates if task scheduler rate limiter should run in shadow mode
i.e. through rate limiter and emit metrics but do not actually block/throttle task scheduling`,
	)
	TaskSchedulerRateLimiterStartupDelay = NewGlobalDurationSetting(
		"history.taskSchedulerRateLimiterStartupDelay",
		5*time.Second,
		`TaskSchedulerRateLimiterStartupDelay is the duration to wait after startup before enforcing task scheduler rate limiting`,
	)
	TaskSchedulerGlobalMaxQPS = NewGlobalIntSetting(
		"history.taskSchedulerGlobalMaxQPS",
		0,
		`TaskSchedulerGlobalMaxQPS is the max qps all task schedulers in the cluster can schedule tasks
If value less or equal to 0, will fall back to TaskSchedulerMaxQPS`,
	)
	TaskSchedulerMaxQPS = NewGlobalIntSetting(
		"history.taskSchedulerMaxQPS",
		0,
		`TaskSchedulerMaxQPS is the max qps task schedulers on a host can schedule tasks
If value less or equal to 0, will fall back to HistoryPersistenceMaxQPS`,
	)
	TaskSchedulerGlobalNamespaceMaxQPS = NewNamespaceIntSetting(
		"history.taskSchedulerGlobalNamespaceMaxQPS",
		0,
		`TaskSchedulerGlobalNamespaceMaxQPS is the max qps all task schedulers in the cluster can schedule tasks for a certain namespace
If value less or equal to 0, will fall back to TaskSchedulerNamespaceMaxQPS`,
	)
	TaskSchedulerNamespaceMaxQPS = NewNamespaceIntSetting(
		"history.taskSchedulerNamespaceMaxQPS",
		0,
		`TaskSchedulerNamespaceMaxQPS is the max qps task schedulers on a host can schedule tasks for a certain namespace
If value less or equal to 0, will fall back to HistoryPersistenceNamespaceMaxQPS`,
	)
	TaskSchedulerInactiveChannelDeletionDelay = NewGlobalDurationSetting(
		"history.taskSchedulerInactiveChannelDeletionDelay",
		time.Hour,
		`TaskSchedulerInactiveChannelDeletionDelay the time delay before a namespace's' channel is removed from the scheduler`,
	)

	TimerTaskBatchSize = NewGlobalIntSetting(
		"history.timerTaskBatchSize",
		100,
		`TimerTaskBatchSize is batch size for timer processor to process tasks`,
	)
	TimerProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.timerProcessorSchedulerWorkerCount",
		512,
		`TimerProcessorSchedulerWorkerCount is the number of workers in the host level task scheduler for timer processor`,
	)
	TimerProcessorSchedulerActiveRoundRobinWeights = NewNamespaceMapSetting(
		"history.timerProcessorSchedulerActiveRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`TimerProcessorSchedulerActiveRoundRobinWeights is the priority round robin weights used by timer task scheduler for active namespaces`,
	)
	TimerProcessorSchedulerStandbyRoundRobinWeights = NewNamespaceMapSetting(
		"history.timerProcessorSchedulerStandbyRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`TimerProcessorSchedulerStandbyRoundRobinWeights is the priority round robin weights used by timer task scheduler for standby namespaces`,
	)
	TimerProcessorUpdateAckInterval = NewGlobalDurationSetting(
		"history.timerProcessorUpdateAckInterval",
		30*time.Second,
		`TimerProcessorUpdateAckInterval is update interval for timer processor`,
	)
	TimerProcessorUpdateAckIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.timerProcessorUpdateAckIntervalJitterCoefficient",
		0.15,
		`TimerProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient`,
	)
	TimerProcessorMaxPollRPS = NewGlobalIntSetting(
		"history.timerProcessorMaxPollRPS",
		20,
		`TimerProcessorMaxPollRPS is max poll rate per second for timer processor`,
	)
	TimerProcessorMaxPollHostRPS = NewGlobalIntSetting(
		"history.timerProcessorMaxPollHostRPS",
		0,
		`TimerProcessorMaxPollHostRPS is max poll rate per second for all timer processor on a host`,
	)
	TimerProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.timerProcessorMaxPollInterval",
		5*time.Minute,
		`TimerProcessorMaxPollInterval is max poll interval for timer processor`,
	)
	TimerProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.timerProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`TimerProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	TimerProcessorPollBackoffInterval = NewGlobalDurationSetting(
		"history.timerProcessorPollBackoffInterval",
		5*time.Second,
		`TimerProcessorPollBackoffInterval is the poll backoff interval if task redispatcher's size exceeds limit for timer processor`,
	)
	TimerProcessorMaxTimeShift = NewGlobalDurationSetting(
		"history.timerProcessorMaxTimeShift",
		1*time.Second,
		`TimerProcessorMaxTimeShift is the max shift timer processor can have`,
	)
	TimerQueueMaxReaderCount = NewGlobalIntSetting(
		"history.timerQueueMaxReaderCount",
		2,
		`TimerQueueMaxReaderCount is the max number of readers in one multi-cursor timer queue`,
	)
	RetentionTimerJitterDuration = NewGlobalDurationSetting(
		"history.retentionTimerJitterDuration",
		30*time.Minute,
		`RetentionTimerJitterDuration is a time duration jitter to distribute timer from T0 to T0 + jitter duration`,
	)

	MemoryTimerProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.memoryTimerProcessorSchedulerWorkerCount",
		64,
		`MemoryTimerProcessorSchedulerWorkerCount is the number of workers in the task scheduler for in memory timer processor.`,
	)

	TransferTaskBatchSize = NewGlobalIntSetting(
		"history.transferTaskBatchSize",
		100,
		`TransferTaskBatchSize is batch size for transferQueueProcessor`,
	)
	TransferProcessorMaxPollRPS = NewGlobalIntSetting(
		"history.transferProcessorMaxPollRPS",
		20,
		`TransferProcessorMaxPollRPS is max poll rate per second for transferQueueProcessor`,
	)
	TransferProcessorMaxPollHostRPS = NewGlobalIntSetting(
		"history.transferProcessorMaxPollHostRPS",
		0,
		`TransferProcessorMaxPollHostRPS is max poll rate per second for all transferQueueProcessor on a host`,
	)
	TransferProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.transferProcessorSchedulerWorkerCount",
		512,
		`TransferProcessorSchedulerWorkerCount is the number of workers in the host level task scheduler for transferQueueProcessor`,
	)
	TransferProcessorSchedulerActiveRoundRobinWeights = NewNamespaceMapSetting(
		"history.transferProcessorSchedulerActiveRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`TransferProcessorSchedulerActiveRoundRobinWeights is the priority round robin weights used by transfer task scheduler for active namespaces`,
	)
	TransferProcessorSchedulerStandbyRoundRobinWeights = NewNamespaceMapSetting(
		"history.transferProcessorSchedulerStandbyRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`TransferProcessorSchedulerStandbyRoundRobinWeights is the priority round robin weights used by transfer task scheduler for standby namespaces`,
	)
	TransferProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.transferProcessorMaxPollInterval",
		1*time.Minute,
		`TransferProcessorMaxPollInterval max poll interval for transferQueueProcessor`,
	)
	TransferProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.transferProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`TransferProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	TransferProcessorUpdateAckInterval = NewGlobalDurationSetting(
		"history.transferProcessorUpdateAckInterval",
		30*time.Second,
		`TransferProcessorUpdateAckInterval is update interval for transferQueueProcessor`,
	)
	TransferProcessorUpdateAckIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.transferProcessorUpdateAckIntervalJitterCoefficient",
		0.15,
		`TransferProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient`,
	)
	TransferProcessorPollBackoffInterval = NewGlobalDurationSetting(
		"history.transferProcessorPollBackoffInterval",
		5*time.Second,
		`TransferProcessorPollBackoffInterval is the poll backoff interval if task redispatcher's size exceeds limit for transferQueueProcessor`,
	)
	TransferProcessorEnsureCloseBeforeDelete = NewGlobalBoolSetting(
		"history.transferProcessorEnsureCloseBeforeDelete",
		true,
		`TransferProcessorEnsureCloseBeforeDelete means we ensure the execution is closed before we delete it`,
	)
	TransferQueueMaxReaderCount = NewGlobalIntSetting(
		"history.transferQueueMaxReaderCount",
		2,
		`TransferQueueMaxReaderCount is the max number of readers in one multi-cursor transfer queue`,
	)

	OutboundTaskBatchSize = NewGlobalIntSetting(
		"history.outboundTaskBatchSize",
		100,
		`OutboundTaskBatchSize is batch size for outboundQueueFactory`,
	)
	OutboundQueuePendingTaskMaxCount = NewGlobalIntSetting(
		"history.outboundQueuePendingTasksMaxCount",
		10000,
		`The max number of task pending tasks in the outbound queue before stopping loading new tasks into memory. This
limit is in addition to outboundQueuePendingTaskCriticalCount which controls when to unload already loaded tasks but
doesn't prevent loading new tasks. Ideally this max count limit should not be hit and task unloading should happen once
critical count is exceeded. But since queue action is async, we need this hard limit.
`,
	)
	OutboundQueuePendingTaskCriticalCount = NewGlobalIntSetting(
		"history.outboundQueuePendingTaskCriticalCount",
		9000,
		`Max number of pending tasks in the outbound queue before triggering slice splitting and unloading.`,
	)
	OutboundQueueMaxPredicateSize = NewGlobalIntSetting(
		"history.outboundQueueMaxPredicateSize",
		10*1024,
		`The max size of the multi-cursor predicate structure stored in the shard info record for the outbound queue. 0
is considered unlimited. When the predicate size is surpassed for a given scope, the predicate is converted to a
universal predicate, which causes all tasks in the scope's range to eventually be reprocessed without applying any
filtering logic.
`,
	)

	OutboundProcessorMaxPollRPS = NewGlobalIntSetting(
		"history.outboundProcessorMaxPollRPS",
		20,
		`OutboundProcessorMaxPollRPS is max poll rate per second for outboundQueueFactory`,
	)
	OutboundProcessorMaxPollHostRPS = NewGlobalIntSetting(
		"history.outboundProcessorMaxPollHostRPS",
		0,
		`OutboundProcessorMaxPollHostRPS is max poll rate per second for all outboundQueueFactory on a host`,
	)
	OutboundProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.outboundProcessorMaxPollInterval",
		1*time.Minute,
		`OutboundProcessorMaxPollInterval max poll interval for outboundQueueFactory`,
	)
	OutboundProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.outboundProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`OutboundProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	OutboundProcessorUpdateAckInterval = NewGlobalDurationSetting(
		"history.outboundProcessorUpdateAckInterval",
		30*time.Second,
		`OutboundProcessorUpdateAckInterval is update interval for outboundQueueFactory`,
	)
	OutboundProcessorUpdateAckIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.outboundProcessorUpdateAckIntervalJitterCoefficient",
		0.15,
		`OutboundProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient`,
	)
	OutboundProcessorPollBackoffInterval = NewGlobalDurationSetting(
		"history.outboundProcessorPollBackoffInterval",
		5*time.Second,
		`OutboundProcessorPollBackoffInterval is the poll backoff interval if task redispatcher's size exceeds limit for outboundQueueFactory`,
	)
	OutboundQueueMaxReaderCount = NewGlobalIntSetting(
		"history.outboundQueueMaxReaderCount",
		4,
		`OutboundQueueMaxReaderCount is the max number of readers in one multi-cursor outbound queue`,
	)
	OutboundQueueGroupLimiterBufferSize = NewDestinationIntSetting(
		"history.outboundQueue.groupLimiter.bufferSize",
		100,
		`OutboundQueueGroupLimiterBufferSize is the max buffer size of the group limiter`,
	)
	OutboundQueueGroupLimiterConcurrency = NewDestinationIntSetting(
		"history.outboundQueue.groupLimiter.concurrency",
		100,
		`OutboundQueueGroupLimiterConcurrency is the concurrency of the group limiter`,
	)
	OutboundQueueHostSchedulerMaxTaskRPS = NewDestinationFloatSetting(
		"history.outboundQueue.hostScheduler.maxTaskRPS",
		100.0,
		`OutboundQueueHostSchedulerMaxTaskRPS is the host scheduler max task RPS`,
	)
	OutboundQueueCircuitBreakerSettings = NewDestinationTypedSetting(
		"history.outboundQueue.circuitBreakerSettings",
		CircuitBreakerSettings{},
		`OutboundQueueCircuitBreakerSettings are circuit breaker settings.
Fields (see gobreaker reference for more details):
- MaxRequests: Maximum number of requests allowed to pass through when it is half-open (default 1).
- Interval (duration): Cyclic period in closed state to clear the internal counts;
  if interval is 0, then it never clears the internal counts (default 0).
- Timeout (duration): Period of open state before changing to half-open state (default 60s).`,
	)
	OutboundStandbyTaskMissingEventsDiscardDelay = NewDestinationDurationSetting(
		"history.outboundQueue.standbyTaskMissingEventsDiscardDelay",
		// This is effectively equivalent to never discarding outbound tasks since it's 290+ years.
		time.Duration(math.MaxInt64),
		`OutboundStandbyTaskMissingEventsDiscardDelay is the equivalent of
StandbyTaskMissingEventsDiscardDelay for outbound standby task processor.`,
	)
	OutboundStandbyTaskMissingEventsDestinationDownErr = NewDestinationBoolSetting(
		"history.outboundQueue.standbyTaskMissingEventsDestinationDownErr",
		true,
		`OutboundStandbyTaskMissingEventsDestinationDownErr enables returning DestinationDownError when
the outbound standby task failed to be processed due to missing events.`,
	)

	VisibilityTaskBatchSize = NewGlobalIntSetting(
		"history.visibilityTaskBatchSize",
		100,
		`VisibilityTaskBatchSize is batch size for visibilityQueueProcessor`,
	)
	VisibilityProcessorMaxPollRPS = NewGlobalIntSetting(
		"history.visibilityProcessorMaxPollRPS",
		20,
		`VisibilityProcessorMaxPollRPS is max poll rate per second for visibilityQueueProcessor`,
	)
	VisibilityProcessorMaxPollHostRPS = NewGlobalIntSetting(
		"history.visibilityProcessorMaxPollHostRPS",
		0,
		`VisibilityProcessorMaxPollHostRPS is max poll rate per second for all visibilityQueueProcessor on a host`,
	)
	VisibilityProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.visibilityProcessorSchedulerWorkerCount",
		512,
		`VisibilityProcessorSchedulerWorkerCount is the number of workers in the host level task scheduler for visibilityQueueProcessor`,
	)
	VisibilityProcessorSchedulerActiveRoundRobinWeights = NewNamespaceMapSetting(
		"history.visibilityProcessorSchedulerActiveRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`VisibilityProcessorSchedulerActiveRoundRobinWeights is the priority round robin weights by visibility task scheduler for active namespaces`,
	)
	VisibilityProcessorSchedulerStandbyRoundRobinWeights = NewNamespaceMapSetting(
		"history.visibilityProcessorSchedulerStandbyRoundRobinWeights",
		nil, // actual default is in service/history/configs package
		`VisibilityProcessorSchedulerStandbyRoundRobinWeights is the priority round robin weights by visibility task scheduler for standby namespaces`,
	)
	VisibilityProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.visibilityProcessorMaxPollInterval",
		1*time.Minute,
		`VisibilityProcessorMaxPollInterval max poll interval for visibilityQueueProcessor`,
	)
	VisibilityProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.visibilityProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`VisibilityProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	VisibilityProcessorUpdateAckInterval = NewGlobalDurationSetting(
		"history.visibilityProcessorUpdateAckInterval",
		30*time.Second,
		`VisibilityProcessorUpdateAckInterval is update interval for visibilityQueueProcessor`,
	)
	VisibilityProcessorUpdateAckIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.visibilityProcessorUpdateAckIntervalJitterCoefficient",
		0.15,
		`VisibilityProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient`,
	)
	VisibilityProcessorPollBackoffInterval = NewGlobalDurationSetting(
		"history.visibilityProcessorPollBackoffInterval",
		5*time.Second,
		`VisibilityProcessorPollBackoffInterval is the poll backoff interval if task redispatcher's size exceeds limit for visibilityQueueProcessor`,
	)
	VisibilityProcessorEnsureCloseBeforeDelete = NewGlobalBoolSetting(
		"history.visibilityProcessorEnsureCloseBeforeDelete",
		false,
		`VisibilityProcessorEnsureCloseBeforeDelete means we ensure the visibility of an execution is closed before we delete its visibility records`,
	)
	VisibilityProcessorEnableCloseWorkflowCleanup = NewNamespaceBoolSetting(
		"history.visibilityProcessorEnableCloseWorkflowCleanup",
		false,
		`VisibilityProcessorEnableCloseWorkflowCleanup to clean up the mutable state after visibility
close task has been processed. Must use Elasticsearch as visibility store, otherwise workflow
data (eg: search attributes) will be lost after workflow is closed.`,
	)
	VisibilityProcessorRelocateAttributesMinBlobSize = NewNamespaceIntSetting(
		"history.visibilityProcessorRelocateAttributesMinBlobSize",
		0,
		`VisibilityProcessorRelocateAttributesMinBlobSize is the minimum size in bytes of memo or search
attributes.`,
	)
	VisibilityQueueMaxReaderCount = NewGlobalIntSetting(
		"history.visibilityQueueMaxReaderCount",
		2,
		`VisibilityQueueMaxReaderCount is the max number of readers in one multi-cursor visibility queue`,
	)

	DisableFetchRelocatableAttributesFromVisibility = NewNamespaceBoolSetting(
		"history.disableFetchRelocatableAttributesFromVisibility",
		false,
		`DisableFetchRelocatableAttributesFromVisibility disables fetching memo and search attributes from
visibility if they were removed from the mutable state`,
	)

	ArchivalTaskBatchSize = NewGlobalIntSetting(
		"history.archivalTaskBatchSize",
		100,
		`ArchivalTaskBatchSize is batch size for archivalQueueProcessor`,
	)
	ArchivalProcessorMaxPollRPS = NewGlobalIntSetting(
		"history.archivalProcessorMaxPollRPS",
		20,
		`ArchivalProcessorMaxPollRPS is max poll rate per second for archivalQueueProcessor`,
	)
	ArchivalProcessorMaxPollHostRPS = NewGlobalIntSetting(
		"history.archivalProcessorMaxPollHostRPS",
		0,
		`ArchivalProcessorMaxPollHostRPS is max poll rate per second for all archivalQueueProcessor on a host`,
	)
	ArchivalProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.archivalProcessorSchedulerWorkerCount",
		512,
		`ArchivalProcessorSchedulerWorkerCount is the number of workers in the host level task scheduler for
archivalQueueProcessor`,
	)
	ArchivalProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.archivalProcessorMaxPollInterval",
		5*time.Minute,
		`ArchivalProcessorMaxPollInterval max poll interval for archivalQueueProcessor`,
	)
	ArchivalProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.archivalProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`ArchivalProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	ArchivalProcessorUpdateAckInterval = NewGlobalDurationSetting(
		"history.archivalProcessorUpdateAckInterval",
		30*time.Second,
		`ArchivalProcessorUpdateAckInterval is update interval for archivalQueueProcessor`,
	)
	ArchivalProcessorUpdateAckIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.archivalProcessorUpdateAckIntervalJitterCoefficient",
		0.15,
		`ArchivalProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient`,
	)
	ArchivalProcessorPollBackoffInterval = NewGlobalDurationSetting(
		"history.archivalProcessorPollBackoffInterval",
		5*time.Second,
		`ArchivalProcessorPollBackoffInterval is the poll backoff interval if task redispatcher's size exceeds limit for
archivalQueueProcessor`,
	)
	ArchivalProcessorArchiveDelay = NewGlobalDurationSetting(
		"history.archivalProcessorArchiveDelay",
		5*time.Minute,
		`ArchivalProcessorArchiveDelay is the delay before archivalQueueProcessor starts to process archival tasks`,
	)
	ArchivalBackendMaxRPS = NewGlobalFloatSetting(
		"history.archivalBackendMaxRPS",
		10000.0,
		`ArchivalBackendMaxRPS is the maximum rate of requests per second to the archival backend`,
	)
	ArchivalQueueMaxReaderCount = NewGlobalIntSetting(
		"history.archivalQueueMaxReaderCount",
		2,
		`ArchivalQueueMaxReaderCount is the max number of readers in one multi-cursor archival queue`,
	)

	WorkflowExecutionMaxInFlightUpdates = NewNamespaceIntSetting(
		"history.maxInFlightUpdates",
		10,
		`WorkflowExecutionMaxInFlightUpdates is the max number of updates that can be in-flight (admitted but not yet completed) for any given workflow execution. Set to zero to disable limit.`,
	)
	WorkflowExecutionMaxInFlightUpdatePayloads = NewNamespaceIntSetting(
		"history.maxInFlightUpdatePayloads",
		20*1024*1024,
		`WorkflowExecutionMaxInFlightUpdatePayloads is the max total payload size (in bytes) of in-flight updates (admitted but not yet completed) for any given workflow execution. Set to zero to disable.`,
	)
	WorkflowExecutionMaxTotalUpdates = NewNamespaceIntSetting(
		"history.maxTotalUpdates",
		2000,
		`WorkflowExecutionMaxTotalUpdates is the max number of updates that any given workflow execution can receive. Set to zero to disable.`,
	)
	WorkflowExecutionMaxTotalUpdatesSuggestContinueAsNewThreshold = NewNamespaceFloatSetting(
		"history.maxTotalUpdates.suggestContinueAsNewThreshold",
		0.9,
		`WorkflowExecutionMaxTotalUpdatesSuggestContinueAsNewThreshold is the percentage threshold of total updates that any given workflow execution can receive before suggesting to continue-as-new.`,
	)
	EnableUpdateWithStartRetryOnClosedWorkflowAbort = NewNamespaceBoolSetting(
		"history.enableUpdateWithStartRetryOnClosedWorkflowAbort",
		true,
		`EnableUpdateWithStartRetryOnClosedWorkflowAbort enables retrying Update-with-Start's update if it was aborted by a closing workflow.`,
	)
	EnableUpdateWithStartRetryableErrorOnClosedWorkflowAbort = NewNamespaceBoolSetting(
		"history.enableUpdateWithStartRetryableErrorOnClosedWorkflowAbort",
		true,
		`EnableUpdateWithStartRetryableErrorOnClosedWorkflowAbort enables sending back a retryable status code when the Update-with-Start's update was aborted by a closing workflow.`,
	)

	ReplicatorTaskBatchSize = NewGlobalIntSetting(
		"history.replicatorTaskBatchSize",
		100,
		`ReplicatorTaskBatchSize is batch size for ReplicatorProcessor`,
	)
	ReplicatorMaxSkipTaskCount = NewGlobalIntSetting(
		"history.replicatorMaxSkipTaskCount",
		250,
		`ReplicatorMaxSkipTaskCount is maximum number of tasks that can be skipped during tasks pagination due to not meeting filtering conditions (e.g. missed namespace).`,
	)
	ReplicatorProcessorMaxPollInterval = NewGlobalDurationSetting(
		"history.replicatorProcessorMaxPollInterval",
		1*time.Minute,
		`ReplicatorProcessorMaxPollInterval is max poll interval for ReplicatorProcessor`,
	)
	ReplicatorProcessorMaxPollIntervalJitterCoefficient = NewGlobalFloatSetting(
		"history.replicatorProcessorMaxPollIntervalJitterCoefficient",
		0.15,
		`ReplicatorProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient`,
	)
	MaximumBufferedEventsBatch = NewGlobalIntSetting(
		"history.maximumBufferedEventsBatch",
		100,
		`MaximumBufferedEventsBatch is the maximum permissible number of buffered events for any given mutable state.`,
	)
	MaximumBufferedEventsSizeInBytes = NewGlobalIntSetting(
		"history.maximumBufferedEventsSizeInBytes",
		2*1024*1024,
		`MaximumBufferedEventsSizeInBytes is the maximum permissible size of all buffered events for any given mutable
state. The total size is determined by the sum of the size, in bytes, of each HistoryEvent proto.`,
	)
	MaximumSignalsPerExecution = NewNamespaceIntSetting(
		"history.maximumSignalsPerExecution",
		10000,
		`MaximumSignalsPerExecution is max number of signals supported by single execution`,
	)
	ShardUpdateMinInterval = NewGlobalDurationSetting(
		"history.shardUpdateMinInterval",
		5*time.Minute,
		`ShardUpdateMinInterval is the minimal time interval which the shard info can be updated`,
	)
	ShardFirstUpdateInterval = NewGlobalDurationSetting(
		"history.shardFirstUpdateInterval",
		10*time.Second,
		`ShardFirstUpdateInterval is the time interval after which the first shard info update will happen.
		It should be smaller than ShardUpdateMinInterval`,
	)
	ShardUpdateMinTasksCompleted = NewGlobalIntSetting(
		"history.shardUpdateMinTasksCompleted",
		1000,
		`ShardUpdateMinTasksCompleted is the minimum number of tasks which must be completed (across all queues) before the shard info can be updated.
Note that once history.shardUpdateMinInterval amount of time has passed we'll update the shard info regardless of the number of tasks completed.
When the this config is zero or lower we will only update shard info at most once every history.shardUpdateMinInterval.`,
	)
	ShardSyncMinInterval = NewGlobalDurationSetting(
		"history.shardSyncMinInterval",
		5*time.Minute,
		`ShardSyncMinInterval is the minimal time interval which the shard info should be sync to remote`,
	)
	EmitShardLagLog = NewGlobalBoolSetting(
		"history.emitShardLagLog",
		false,
		`EmitShardLagLog whether emit the shard lag log`,
	)
	DefaultEventEncoding = NewNamespaceStringSetting(
		"history.defaultEventEncoding",
		enumspb.ENCODING_TYPE_PROTO3.String(),
		`DefaultEventEncoding is the encoding type for history events`,
	)
	DefaultActivityRetryPolicy = NewNamespaceTypedSetting(
		"history.defaultActivityRetryPolicy",
		retrypolicy.DefaultDefaultRetrySettings,
		`DefaultActivityRetryPolicy represents the out-of-box retry policy for activities where
the user has not specified an explicit RetryPolicy`,
	)
	DefaultWorkflowRetryPolicy = NewNamespaceTypedSetting(
		"history.defaultWorkflowRetryPolicy",
		retrypolicy.DefaultDefaultRetrySettings,
		`DefaultWorkflowRetryPolicy represents the out-of-box retry policy for unset fields
where the user has set an explicit RetryPolicy, but not specified all the fields`,
	)
	AllowResetWithPendingChildren = NewNamespaceBoolSetting(
		"history.allowResetWithPendingChildren",
		true,
		`Allows resetting of workflows with pending children when set to true`,
	)
	HistoryMaxAutoResetPoints = NewNamespaceIntSetting(
		"history.historyMaxAutoResetPoints",
		primitives.DefaultHistoryMaxAutoResetPoints,
		`HistoryMaxAutoResetPoints is the key for max number of auto reset points stored in mutableState`,
	)
	EnableParentClosePolicy = NewNamespaceBoolSetting(
		"history.enableParentClosePolicy",
		true,
		`EnableParentClosePolicy whether to  ParentClosePolicy`,
	)
	ParentClosePolicyThreshold = NewNamespaceIntSetting(
		"history.parentClosePolicyThreshold",
		10,
		`ParentClosePolicyThreshold decides that parent close policy will be processed by sys workers(if enabled) if
the number of children greater than or equal to this threshold`,
	)
	NumParentClosePolicySystemWorkflows = NewGlobalIntSetting(
		"history.numParentClosePolicySystemWorkflows",
		1000,
		`NumParentClosePolicySystemWorkflows is key for number of parentClosePolicy system workflows running in total`,
	)
	HistoryThrottledLogRPS = NewGlobalIntSetting(
		"history.throttledLogRPS",
		4,
		`HistoryThrottledLogRPS is the rate limit on number of log messages emitted per second for throttled logger`,
	)
	WorkflowTaskHeartbeatTimeout = NewNamespaceDurationSetting(
		"history.workflowTaskHeartbeatTimeout",
		time.Minute*30,
		`WorkflowTaskHeartbeatTimeout for workflow task heartbeat`,
	)
	WorkflowTaskCriticalAttempts = NewGlobalIntSetting(
		"history.workflowTaskCriticalAttempt",
		10,
		`WorkflowTaskCriticalAttempts is the number of attempts for a workflow task that's regarded as critical`,
	)
	WorkflowTaskRetryMaxInterval = NewGlobalDurationSetting(
		"history.workflowTaskRetryMaxInterval",
		time.Minute*10,
		`WorkflowTaskRetryMaxInterval is the maximum interval added to a workflow task's startToClose timeout for slowing down retry`,
	)
	DiscardSpeculativeWorkflowTaskMaximumEventsCount = NewGlobalIntSetting(
		"history.discardSpeculativeWorkflowTaskMaximumEventsCount",
		10,
		`If speculative workflow task shipped more than DiscardSpeculativeWorkflowTaskMaximumEventsCount events, it can't be discarded`,
	)
	DefaultWorkflowTaskTimeout = NewNamespaceDurationSetting(
		"history.defaultWorkflowTaskTimeout",
		primitives.DefaultWorkflowTaskTimeout,
		`DefaultWorkflowTaskTimeout for a workflow task`,
	)
	SkipReapplicationByNamespaceID = NewNamespaceIDBoolSetting(
		"history.SkipReapplicationByNamespaceID",
		false,
		`SkipReapplicationByNamespaceID is whether skipping a event re-application for a namespace`,
	)
	StandbyTaskReReplicationContextTimeout = NewNamespaceIDDurationSetting(
		"history.standbyTaskReReplicationContextTimeout",
		30*time.Second,
		`StandbyTaskReReplicationContextTimeout is the context timeout for standby task re-replication`,
	)
	MaxBufferedQueryCount = NewGlobalIntSetting(
		"history.MaxBufferedQueryCount",
		1,
		`MaxBufferedQueryCount indicates max buffer query count`,
	)
	MutableStateChecksumGenProbability = NewNamespaceIntSetting(
		"history.mutableStateChecksumGenProbability",
		0,
		`MutableStateChecksumGenProbability is the probability [0-100] that checksum will be generated for mutable state`,
	)
	MutableStateChecksumVerifyProbability = NewNamespaceIntSetting(
		"history.mutableStateChecksumVerifyProbability",
		0,
		`MutableStateChecksumVerifyProbability is the probability [0-100] that checksum will be verified for mutable state`,
	)
	MutableStateChecksumInvalidateBefore = NewGlobalFloatSetting(
		"history.mutableStateChecksumInvalidateBefore",
		0,
		`MutableStateChecksumInvalidateBefore is the epoch timestamp before which all checksums are to be discarded`,
	)

	ReplicationTaskApplyTimeout = NewGlobalDurationSetting(
		"history.ReplicationTaskApplyTimeout",
		20*time.Second,
		`ReplicationTaskApplyTimeout is the context timeout for replication task apply`,
	)
	ReplicationTaskFetcherParallelism = NewGlobalIntSetting(
		"history.ReplicationTaskFetcherParallelism",
		4,
		`ReplicationTaskFetcherParallelism determines how many go routines we spin up for fetching tasks`,
	)
	ReplicationTaskFetcherAggregationInterval = NewGlobalDurationSetting(
		"history.ReplicationTaskFetcherAggregationInterval",
		2*time.Second,
		`ReplicationTaskFetcherAggregationInterval determines how frequently the fetch requests are sent`,
	)
	ReplicationTaskFetcherTimerJitterCoefficient = NewGlobalFloatSetting(
		"history.ReplicationTaskFetcherTimerJitterCoefficient",
		0.15,
		`ReplicationTaskFetcherTimerJitterCoefficient is the jitter for fetcher timer`,
	)
	ReplicationTaskFetcherErrorRetryWait = NewGlobalDurationSetting(
		"history.ReplicationTaskFetcherErrorRetryWait",
		time.Second,
		`ReplicationTaskFetcherErrorRetryWait is the wait time when fetcher encounters error`,
	)
	ReplicationTaskProcessorErrorRetryWait = NewShardIDDurationSetting(
		"history.ReplicationTaskProcessorErrorRetryWait",
		1*time.Second,
		`ReplicationTaskProcessorErrorRetryWait is the initial retry wait when we see errors in applying replication tasks`,
	)
	ReplicationTaskProcessorErrorRetryBackoffCoefficient = NewShardIDFloatSetting(
		"history.ReplicationTaskProcessorErrorRetryBackoffCoefficient",
		1.2,
		`ReplicationTaskProcessorErrorRetryBackoffCoefficient is the retry wait backoff time coefficient`,
	)
	ReplicationTaskProcessorErrorRetryMaxInterval = NewShardIDDurationSetting(
		"history.ReplicationTaskProcessorErrorRetryMaxInterval",
		5*time.Second,
		`ReplicationTaskProcessorErrorRetryMaxInterval is the retry wait backoff max duration`,
	)
	ReplicationTaskProcessorErrorRetryMaxAttempts = NewShardIDIntSetting(
		"history.ReplicationTaskProcessorErrorRetryMaxAttempts",
		80,
		`ReplicationTaskProcessorErrorRetryMaxAttempts is the max retry attempts for applying replication tasks`,
	)
	ReplicationTaskProcessorErrorRetryExpiration = NewShardIDDurationSetting(
		"history.ReplicationTaskProcessorErrorRetryExpiration",
		5*time.Minute,
		`ReplicationTaskProcessorErrorRetryExpiration is the max retry duration for applying replication tasks`,
	)
	ReplicationTaskProcessorNoTaskInitialWait = NewShardIDDurationSetting(
		"history.ReplicationTaskProcessorNoTaskInitialWait",
		2*time.Second,
		`ReplicationTaskProcessorNoTaskInitialWait is the wait time when not ask is returned`,
	)
	ReplicationTaskProcessorCleanupInterval = NewShardIDDurationSetting(
		"history.ReplicationTaskProcessorCleanupInterval",
		1*time.Minute,
		`ReplicationTaskProcessorCleanupInterval determines how frequently the cleanup replication queue`,
	)
	ReplicationTaskProcessorCleanupJitterCoefficient = NewShardIDFloatSetting(
		"history.ReplicationTaskProcessorCleanupJitterCoefficient",
		0.15,
		`ReplicationTaskProcessorCleanupJitterCoefficient is the jitter for cleanup timer`,
	)
	ReplicationTaskProcessorHostQPS = NewGlobalFloatSetting(
		"history.ReplicationTaskProcessorHostQPS",
		1500,
		`ReplicationTaskProcessorHostQPS is the qps of task processing rate limiter on host level`,
	)
	ReplicationTaskProcessorShardQPS = NewGlobalFloatSetting(
		"history.ReplicationTaskProcessorShardQPS",
		30,
		`ReplicationTaskProcessorShardQPS is the qps of task processing rate limiter on shard level`,
	)
	ReplicationEnableDLQMetrics = NewGlobalBoolSetting(
		"history.ReplicationEnableDLQMetrics",
		true,
		`ReplicationEnableDLQMetrics is the flag to emit DLQ metrics`,
	)
	ReplicationEnableUpdateWithNewTaskMerge = NewGlobalBoolSetting(
		"history.ReplicationEnableUpdateWithNewTaskMerge",
		false,
		`ReplicationEnableUpdateWithNewTaskMerge is the flag controlling whether replication task merging logic
should be enabled for non continuedAsNew workflow UpdateWithNew case.`,
	)
	ReplicationMultipleBatches = NewGlobalBoolSetting(
		"history.ReplicationMultipleBatches",
		false,
		`ReplicationMultipleBatches is the flag to enable replication of multiple history event batches`,
	)
	HistoryTaskDLQEnabled = NewGlobalBoolSetting(
		"history.TaskDLQEnabled",
		true,
		`HistoryTaskDLQEnabled enables the history task DLQ. This applies to internal tasks like transfer and timer tasks.
Do not turn this on if you aren't using Cassandra as the history task DLQ is not implemented for other databases.`,
	)
	HistoryTaskDLQUnexpectedErrorAttempts = NewGlobalIntSetting(
		"history.TaskDLQUnexpectedErrorAttempts",
		70, // 70 attempts takes about an hour
		`HistoryTaskDLQUnexpectedErrorAttempts is the number of task execution attempts before sending the task to DLQ.`,
	)
	HistoryTaskDLQInternalErrors = NewGlobalBoolSetting(
		"history.TaskDLQInternalErrors",
		false,
		`HistoryTaskDLQInternalErrors causes history task processing to send tasks failing with serviceerror.Internal to
the dlq (or will drop them if not enabled)`,
	)
	HistoryTaskDLQErrorPattern = NewGlobalStringSetting(
		"history.TaskDLQErrorPattern",
		"",
		`HistoryTaskDLQErrorPattern specifies a regular expression. If a task processing error matches with this regex,
that task will be sent to DLQ.`,
	)

	MaxLocalParentWorkflowVerificationDuration = NewGlobalDurationSetting(
		"history.maxLocalParentWorkflowVerificationDuration",
		5*time.Minute,
		`MaxLocalParentWorkflowVerificationDuration controls the maximum duration to verify on the local cluster before requesting to resend parent workflow.`,
	)

	ReplicationStreamSyncStatusDuration = NewGlobalDurationSetting(
		"history.ReplicationStreamSyncStatusDuration",
		1*time.Second,
		`ReplicationStreamSyncStatusDuration sync replication status duration`,
	)
	ReplicationProcessorSchedulerQueueSize = NewGlobalIntSetting(
		"history.ReplicationProcessorSchedulerQueueSize",
		128,
		`ReplicationProcessorSchedulerQueueSize is the replication task executor queue size`,
	)
	ReplicationProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.ReplicationProcessorSchedulerWorkerCount",
		512,
		`ReplicationProcessorSchedulerWorkerCount is the replication task executor worker count`,
	)
	ReplicationLowPriorityProcessorSchedulerWorkerCount = NewGlobalIntSetting(
		"history.ReplicationLowPriorityProcessorSchedulerWorkerCount",
		128,
		`ReplicationLowPriorityProcessorSchedulerWorkerCount is the low priority replication task executor worker count`,
	)
	ReplicationLowPriorityTaskParallelism = NewGlobalIntSetting(
		"history.ReplicationLowPriorityTaskParallelism",
		1,
		`ReplicationLowPriorityTaskParallelism is the number of executions' low priority replication tasks that can be processed in parallel`,
	)

	EnableEagerNamespaceRefresher = NewGlobalBoolSetting(
		"history.EnableEagerNamespaceRefresher",
		false,
		`EnableEagerNamespaceRefresher is a feature flag for eagerly refresh namespace during processing replication task`,
	)
	EnableReplicationTaskBatching = NewGlobalBoolSetting(
		"history.EnableReplicationTaskBatching",
		false,
		`EnableReplicationTaskBatching is a feature flag for batching replicate history event task`,
	)
	EnableReplicationTaskTieredProcessing = NewGlobalBoolSetting(
		"history.EnableReplicationTaskTieredProcessing",
		false,
		`EnableReplicationTaskTieredProcessing is a feature flag for enabling tiered replication task processing stack`,
	)
	ReplicationStreamSenderHighPriorityQPS = NewGlobalIntSetting(
		"history.ReplicationStreamSenderHighPriorityQPS",
		100,
		`Maximum number of high priority replication tasks that can be sent per second per shard`,
	)
	ReplicationStreamSenderLowPriorityQPS = NewGlobalIntSetting(
		"history.ReplicationStreamSenderLowPriorityQPS",
		100,
		`Maximum number of low priority replication tasks that can be sent per second per shard`,
	)
	ReplicationStreamEventLoopRetryMaxAttempts = NewGlobalIntSetting(
		"history.ReplicationStreamEventLoopRetryMaxAttempts",
		100, // 0 means retry forever
		`Max attempts for retrying replication stream event loop`,
	)
	ReplicationReceiverMaxOutstandingTaskCount = NewGlobalIntSetting(
		"history.ReplicationReceiverMaxOutstandingTaskCount",
		500,
		`Maximum number of outstanding tasks allowed for a single shard in the stream receiver`,
	)
	ReplicationResendMaxBatchCount = NewGlobalIntSetting(
		"history.ReplicationResendMaxBatchCount",
		10,
		`Maximum number of resend events batch for a single replication request`,
	)
	ReplicationProgressCacheMaxSize = NewGlobalIntSetting(
		"history.ReplicationProgressCacheMaxSize",
		128000,
		`ReplicationProgressCacheMaxSize is the maximum number of entries in the replication progress cache`,
	)
	ReplicationProgressCacheTTL = NewGlobalDurationSetting(
		"history.ReplicationProgressCacheTTL",
		time.Hour,
		`ReplicationProgressCacheTTL is TTL of replication progress cache`,
	)
	ReplicationStreamSendEmptyTaskDuration = NewGlobalDurationSetting(
		"history.ReplicationStreamSendEmptyTaskDuration",
		time.Minute,
		`ReplicationStreamSendEmptyTaskDuration is the interval to sync status when there is no replication task`,
	)
	ReplicationEnableRateLimit = NewGlobalBoolSetting(
		"history.ReplicationEnableRateLimit",
		true,
		`ReplicationEnableRateLimit is the feature flag to enable replication global rate limiter`,
	)
	WorkflowIdReuseMinimalInterval = NewNamespaceDurationSetting(
		"history.workflowIdReuseMinimalInterval",
		1*time.Second,
		`WorkflowIdReuseMinimalInterval is used for timing how soon users can create new workflow with the same workflow ID.`,
	)
	EnableWorkflowIdReuseStartTimeValidation = NewNamespaceBoolSetting(
		"history.enableWorkflowIdReuseStartTimeValidation",
		false,
		`If true, validate the start time of the old workflow is older than WorkflowIdReuseMinimalInterval when reusing workflow ID.`,
	)
	HealthPersistenceLatencyFailure = NewGlobalFloatSetting(
		"history.healthPersistenceLatencyFailure",
		500,
		"History service health check on persistence average latency (millisecond) threshold",
	)
	HealthPersistenceErrorRatio = NewGlobalFloatSetting(
		"history.healthPersistenceErrorRatio",
		0.90,
		"History service health check on persistence error ratio",
	)
	HealthRPCLatencyFailure = NewGlobalFloatSetting(
		"history.healthRPCLatencyFailure",
		500,
		"History service health check on RPC average latency (millisecond) threshold",
	)
	HealthRPCErrorRatio = NewGlobalFloatSetting(
		"history.healthRPCErrorRatio",
		0.90,
		"History service health check on RPC error ratio",
	)
	SendRawHistoryBetweenInternalServices = NewGlobalBoolSetting(
		"history.sendRawHistoryBetweenInternalServices",
		false,
		`SendRawHistoryBetweenInternalServices is whether to send raw history events between internal temporal services`,
	)

	// TODO(rodrigozhou): This is temporary dynamic config to be removed before the next release.
	EnableRequestIdRefLinks = NewGlobalBoolSetting(
		"history.enableRequestIdRefLinks",
		false,
		"Enable generating request ID reference links",
	)

	EnableChasm = NewGlobalBoolSetting(
		"history.enableChasm",
		false,
		"Use real chasm tree implementation instead of the noop one",
	)

	// keys for worker

	WorkerPersistenceMaxQPS = NewGlobalIntSetting(
		"worker.persistenceMaxQPS",
		500,
		`WorkerPersistenceMaxQPS is the max qps worker host can query DB`,
	)
	WorkerPersistenceGlobalMaxQPS = NewGlobalIntSetting(
		"worker.persistenceGlobalMaxQPS",
		0,
		`WorkerPersistenceGlobalMaxQPS is the max qps worker cluster can query DB`,
	)
	WorkerPersistenceNamespaceMaxQPS = NewNamespaceIntSetting(
		"worker.persistenceNamespaceMaxQPS",
		0,
		`WorkerPersistenceNamespaceMaxQPS is the max qps each namespace on worker host can query DB`,
	)
	WorkerPersistenceGlobalNamespaceMaxQPS = NewNamespaceIntSetting(
		"worker.persistenceGlobalNamespaceMaxQPS",
		0,
		`WorkerPersistenceNamespaceMaxQPS is the max qps each namespace in worker cluster can query DB`,
	)
	WorkerPersistenceDynamicRateLimitingParams = NewGlobalTypedSetting(
		"worker.persistenceDynamicRateLimitingParams",
		DefaultDynamicRateLimitingParams,
		`WorkerPersistenceDynamicRateLimitingParams is a struct that contains all adjustable dynamic rate limiting params.
Fields: Enabled, RefreshInterval, LatencyThreshold, ErrorThreshold, RateBackoffStepSize, RateIncreaseStepSize, RateMultiMin, RateMultiMax.
See DynamicRateLimitingParams comments for more details.`,
	)
	WorkerIndexerConcurrency = NewGlobalIntSetting(
		"worker.indexerConcurrency",
		100,
		`WorkerIndexerConcurrency is the max concurrent messages to be processed at any given time`,
	)
	WorkerESProcessorNumOfWorkers = NewGlobalIntSetting(
		"worker.ESProcessorNumOfWorkers",
		2,
		`WorkerESProcessorNumOfWorkers is num of workers for esProcessor`,
	)
	WorkerESProcessorBulkActions = NewGlobalIntSetting(
		"worker.ESProcessorBulkActions",
		500,
		`WorkerESProcessorBulkActions is max number of requests in bulk for esProcessor`,
	)
	WorkerESProcessorBulkSize = NewGlobalIntSetting(
		"worker.ESProcessorBulkSize",
		16*1024*1024,
		`WorkerESProcessorBulkSize is max total size of bulk in bytes for esProcessor`,
	)
	WorkerESProcessorFlushInterval = NewGlobalDurationSetting(
		"worker.ESProcessorFlushInterval",
		1*time.Second,
		`WorkerESProcessorFlushInterval is flush interval for esProcessor`,
	)
	WorkerESProcessorAckTimeout = NewGlobalDurationSetting(
		"worker.ESProcessorAckTimeout",
		30*time.Second,
		`WorkerESProcessorAckTimeout is the timeout that store will wait to get ack signal from ES processor.
Should be at least WorkerESProcessorFlushInterval+<time to process request>.`,
	)
	WorkerThrottledLogRPS = NewGlobalIntSetting(
		"worker.throttledLogRPS",
		20,
		`WorkerThrottledLogRPS is the rate limit on number of log messages emitted per second for throttled logger`,
	)
	WorkerScannerMaxConcurrentActivityExecutionSize = NewGlobalIntSetting(
		"worker.ScannerMaxConcurrentActivityExecutionSize",
		10,
		`WorkerScannerMaxConcurrentActivityExecutionSize indicates worker scanner max concurrent activity execution size`,
	)
	WorkerScannerMaxConcurrentWorkflowTaskExecutionSize = NewGlobalIntSetting(
		"worker.ScannerMaxConcurrentWorkflowTaskExecutionSize",
		10,
		`WorkerScannerMaxConcurrentWorkflowTaskExecutionSize indicates worker scanner max concurrent workflow execution size`,
	)
	WorkerScannerMaxConcurrentActivityTaskPollers = NewGlobalIntSetting(
		"worker.ScannerMaxConcurrentActivityTaskPollers",
		8,
		`WorkerScannerMaxConcurrentActivityTaskPollers indicates worker scanner max concurrent activity pollers`,
	)
	WorkerScannerMaxConcurrentWorkflowTaskPollers = NewGlobalIntSetting(
		"worker.ScannerMaxConcurrentWorkflowTaskPollers",
		8,
		`WorkerScannerMaxConcurrentWorkflowTaskPollers indicates worker scanner max concurrent workflow pollers`,
	)
	ScannerPersistenceMaxQPS = NewGlobalIntSetting(
		"worker.scannerPersistenceMaxQPS",
		100,
		`ScannerPersistenceMaxQPS is the maximum rate of persistence calls from worker.Scanner`,
	)
	ExecutionScannerPerHostQPS = NewGlobalIntSetting(
		"worker.executionScannerPerHostQPS",
		10,
		`ExecutionScannerPerHostQPS is the maximum rate of calls per host from executions.Scanner`,
	)
	ExecutionScannerPerShardQPS = NewGlobalIntSetting(
		"worker.executionScannerPerShardQPS",
		1,
		`ExecutionScannerPerShardQPS is the maximum rate of calls per shard from executions.Scanner`,
	)
	ExecutionDataDurationBuffer = NewGlobalDurationSetting(
		"worker.executionDataDurationBuffer",
		time.Hour*24*90,
		`ExecutionDataDurationBuffer is the data TTL duration buffer of execution data`,
	)
	ExecutionScannerWorkerCount = NewGlobalIntSetting(
		"worker.executionScannerWorkerCount",
		8,
		`ExecutionScannerWorkerCount is the execution scavenger worker count`,
	)
	ExecutionScannerHistoryEventIdValidator = NewGlobalBoolSetting(
		"worker.executionEnableHistoryEventIdValidator",
		true,
		`ExecutionScannerHistoryEventIdValidator is the flag to enable history event id validator`,
	)
	TaskQueueScannerEnabled = NewGlobalBoolSetting(
		"worker.taskQueueScannerEnabled",
		true,
		`TaskQueueScannerEnabled indicates if task queue scanner should be started as part of worker.Scanner`,
	)
	BuildIdScavengerEnabled = NewGlobalBoolSetting(
		"worker.buildIdScavengerEnabled",
		false,
		`BuildIdScavengerEnabled indicates if the build id scavenger should be started as part of worker.Scanner`,
	)
	HistoryScannerEnabled = NewGlobalBoolSetting(
		"worker.historyScannerEnabled",
		true,
		`HistoryScannerEnabled indicates if history scanner should be started as part of worker.Scanner`,
	)
	ExecutionsScannerEnabled = NewGlobalBoolSetting(
		"worker.executionsScannerEnabled",
		false,
		`ExecutionsScannerEnabled indicates if executions scanner should be started as part of worker.Scanner`,
	)
	HistoryScannerDataMinAge = NewGlobalDurationSetting(
		"worker.historyScannerDataMinAge",
		60*24*time.Hour,
		`HistoryScannerDataMinAge indicates the history scanner cleanup minimum age.`,
	)
	HistoryScannerVerifyRetention = NewGlobalBoolSetting(
		"worker.historyScannerVerifyRetention",
		true,
		`HistoryScannerVerifyRetention indicates the history scanner verify data retention.
If the service configures with archival feature enabled, update worker.historyScannerVerifyRetention to be double of the data retention.`,
	)
	EnableBatcherNamespace = NewNamespaceBoolSetting(
		"worker.enableNamespaceBatcher",
		true,
		`EnableBatcher decides whether to start new (per-namespace) batcher in our worker`,
	)
	BatcherRPS = NewNamespaceIntSetting(
		"worker.batcherRPS",
		50,
		`BatcherRPS controls number the rps of batch operations`,
	)
	BatcherConcurrency = NewNamespaceIntSetting(
		"worker.batcherConcurrency",
		5,
		`BatcherConcurrency controls the concurrency of one batch operation`,
	)
	WorkerParentCloseMaxConcurrentActivityExecutionSize = NewGlobalIntSetting(
		"worker.ParentCloseMaxConcurrentActivityExecutionSize",
		1000,
		`WorkerParentCloseMaxConcurrentActivityExecutionSize indicates worker parent close worker max concurrent activity execution size`,
	)
	WorkerParentCloseMaxConcurrentWorkflowTaskExecutionSize = NewGlobalIntSetting(
		"worker.ParentCloseMaxConcurrentWorkflowTaskExecutionSize",
		1000,
		`WorkerParentCloseMaxConcurrentWorkflowTaskExecutionSize indicates worker parent close worker max concurrent workflow execution size`,
	)
	WorkerParentCloseMaxConcurrentActivityTaskPollers = NewGlobalIntSetting(
		"worker.ParentCloseMaxConcurrentActivityTaskPollers",
		4,
		`WorkerParentCloseMaxConcurrentActivityTaskPollers indicates worker parent close worker max concurrent activity pollers`,
	)
	WorkerParentCloseMaxConcurrentWorkflowTaskPollers = NewGlobalIntSetting(
		"worker.ParentCloseMaxConcurrentWorkflowTaskPollers",
		4,
		`WorkerParentCloseMaxConcurrentWorkflowTaskPollers indicates worker parent close worker max concurrent workflow pollers`,
	)
	WorkerPerNamespaceWorkerCount = NewNamespaceIntSetting(
		"worker.perNamespaceWorkerCount",
		1,
		`WorkerPerNamespaceWorkerCount controls number of per-ns (scheduler, batcher, etc.) workers to run per namespace`,
	)
	WorkerPerNamespaceWorkerOptions = NewNamespaceTypedSetting(
		"worker.perNamespaceWorkerOptions",
		sdkworker.Options{},
		`WorkerPerNamespaceWorkerOptions are SDK worker options for per-namespace workers`,
	)
	WorkerPerNamespaceWorkerStartRate = NewGlobalFloatSetting(
		"worker.perNamespaceWorkerStartRate",
		10.0,
		`WorkerPerNamespaceWorkerStartRate controls how fast per-namespace workers can be started (workers/second)`,
	)
	WorkerEnableScheduler = NewNamespaceBoolSetting(
		"worker.enableScheduler",
		true,
		`WorkerEnableScheduler controls whether to start the worker for scheduled workflows`,
	)
	WorkerStickyCacheSize = NewGlobalIntSetting(
		"worker.stickyCacheSize",
		0,
		`WorkerStickyCacheSize controls the sticky cache size for SDK workers on worker nodes
(shared between all workers in the process, cannot be changed after startup)`,
	)
	SchedulerNamespaceStartWorkflowRPS = NewNamespaceFloatSetting(
		"worker.schedulerNamespaceStartWorkflowRPS",
		30.0,
		`SchedulerNamespaceStartWorkflowRPS is the per-namespace limit for starting workflows by schedules`,
	)
	SchedulerLocalActivitySleepLimit = NewNamespaceDurationSetting(
		"worker.schedulerLocalActivitySleepLimit",
		5*time.Second,
		`How long to sleep within a local activity before pushing to workflow level sleep (don't make this
close to or more than the workflow task timeout)`,
	)
	WorkerDeleteNamespaceActivityLimits = NewGlobalTypedSetting(
		"worker.deleteNamespaceActivityLimitsConfig",
		sdkworker.Options{},
		`WorkerDeleteNamespaceActivityLimitsConfig is a struct with relevant sdkworker.Options
settings for controlling remote activity concurrency for delete namespace workflows.
Valid fields: MaxConcurrentActivityExecutionSize, TaskQueueActivitiesPerSecond,
WorkerActivitiesPerSecond, MaxConcurrentActivityTaskPollers.
`,
	)
	MaxUserMetadataSummarySize = NewNamespaceIntSetting(
		"limit.userMetadataSummarySize",
		400,
		`MaxUserMetadataSummarySize is the maximum size of user metadata summary payloads in bytes.`,
	)
	MaxUserMetadataDetailsSize = NewNamespaceIntSetting(
		"limit.userMetadataDetailsSize",
		20000,
		`MaxUserMetadataDetailsSize is the maximum size of user metadata details payloads in bytes.`,
	)

	LogAllReqErrors = NewNamespaceBoolSetting(
		"system.logAllReqErrors",
		false,
		`When set to true, logs all RPC/request errors for the namespace, not just unexpected ones.`,
	)

	WorkflowRulesAPIsEnabled = NewNamespaceBoolSetting(
		"frontend.workflowRulesAPIsEnabled",
		false,
		`WorkflowRulesAPIsEnabled is a "feature enable" flag. `,
	)

	MaxWorkflowRulesPerNamespace = NewNamespaceIntSetting(
		"frontend.maxWorkflowRulesPerNamespace",
		10,
		`Maximum number of workflow rules in a given namespace`,
	)

	SlowRequestLoggingThreshold = NewGlobalDurationSetting(
		"rpc.slowRequestLoggingThreshold",
		5*time.Second,
		`SlowRequestLoggingThreshold is the threshold above which a gRPC request is considered slow and logged.`,
	)

	WorkerHeartbeatsEnabled = NewNamespaceBoolSetting(
		"frontend.WorkerHeartbeatsEnabled",
		false,
		`WorkerHeartbeatsEnabled is a "feature enable" flag. It allows workers to send periodic heartbeats to the server.`,
	)

	ListWorkersEnabled = NewNamespaceBoolSetting(
		"frontend.ListWorkersEnabled",
		false,
		`ListWorkersEnabled is a "feature enable" flag. It allows clients to get workers heartbeat information.`,
	)

	WorkerCommandsEnabled = NewNamespaceBoolSetting(
		"frontend.WorkerCommandsEnabled",
		false,
		`WorkerCommandsEnabled is a "feature enable" flag. It allows clients to send commands to the workers.`,
	)
)
