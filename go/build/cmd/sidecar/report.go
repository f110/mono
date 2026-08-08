package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"time"

	"google.golang.org/protobuf/encoding/protodelim"

	"go.f110.dev/mono/go/bazel/buildeventstream"
	"go.f110.dev/mono/go/cli"
)

// Report is the JSON document the report sidecar writes to stdout. Tests carries the
// per-test summary (empty for non-test builds); Metrics carries the build-wide
// statistics extracted from the BuildMetrics event.
type Report struct {
	Tests   []TestSummary `json:"tests"`
	Metrics *BuildMetrics `json:"metrics,omitempty"`
}

type TestStatus string

const (
	TestStatusPassed TestStatus = "passed"
	TestStatusFlaky  TestStatus = "flaky"
	TestStatusFailed TestStatus = "failed"
)

type TestSummary struct {
	Label  string     `json:"label"`
	Status TestStatus `json:"status"`
	// Duration is a elapsed time of the test in milliseconds.
	Duration int64     `json:"duration"`
	StartAt  time.Time `json:"start_at"`
}

// BuildMetrics is a flattened, scalar-only view of Bazel's BuildMetrics event.
type BuildMetrics struct {
	RemoteCacheHits               int64 `json:"remote_cache_hits"`
	WallTimeMs                    int64 `json:"wall_time_ms"`
	CpuTimeMs                     int64 `json:"cpu_time_ms"`
	AnalysisPhaseTimeMs           int64 `json:"analysis_phase_time_ms"`
	ExecutionPhaseTimeMs          int64 `json:"execution_phase_time_ms"`
	ActionsCreated                int64 `json:"actions_created"`
	ActionsExecuted               int64 `json:"actions_executed"`
	GraphActionCount              int64 `json:"graph_action_count"`
	OutputArtifactCount           int64 `json:"output_artifact_count"`
	SkyframeNodeCount             int64 `json:"skyframe_node_count"`
	TargetsConfigured             int64 `json:"targets_configured"`
	PackagesLoaded                int64 `json:"packages_loaded"`
	SourceArtifactsReadBytes      int64 `json:"source_artifacts_read_bytes"`
	SourceArtifactsReadCount      int64 `json:"source_artifacts_read_count"`
	OutputArtifactsSeenBytes      int64 `json:"output_artifacts_seen_bytes"`
	OutputArtifactsSeenCount      int64 `json:"output_artifacts_seen_count"`
	OutputArtifactsFromCacheBytes int64 `json:"output_artifacts_from_cache_bytes"`
	OutputArtifactsFromCacheCount int64 `json:"output_artifacts_from_cache_count"`
}

type TestReportCommand struct {
	eventBinaryFile string
	startUpTimeout  time.Duration
}

func NewTestReportCommand() *TestReportCommand {
	return &TestReportCommand{}
}

func (b *TestReportCommand) Name() string {
	return "report"
}

func (b *TestReportCommand) SetFlags(fs *cli.FlagSet) {
	fs.String("event-binary-file", "The path of the event file").Var(&b.eventBinaryFile)
	fs.Duration("startup-timeout", "Time of of start-up").Var(&b.startUpTimeout)
}

type testDuration struct {
	Label    string
	Cached   bool
	Status   buildeventstream.TestStatus
	Duration time.Duration
	Start    time.Time
}

func (b *TestReportCommand) Run(_ context.Context) error {
	f, r, err := waitAndOpenBEP(b.eventBinaryFile, b.startUpTimeout)
	if err != nil {
		return err
	}
	defer f.Close()

	report, err := parseReport(r)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	return nil
}

// parseReport decodes a length-delimited BuildEvent stream and builds the Report.
// The stream is terminated by the event whose last_message flag is set. Bazel emits
// the BuildMetrics event last (after BuildFinished), so termination must key on
// last_message rather than BuildFinished, otherwise the metrics are never read.
func parseReport(r protodelim.Reader) (*Report, error) {
	summaries := make(map[string]*testDuration)
	var metrics *BuildMetrics
	err := iterateBuildEvents(r, func(msg *buildeventstream.BuildEvent) error {
		switch v := msg.Id.Id.(type) {
		case *buildeventstream.BuildEventId_TestResult:
			label := v.TestResult.Label
			if _, ok := summaries[label]; !ok {
				summaries[label] = &testDuration{Label: label}
			}
			summaries[label].Cached = msg.GetTestResult().GetExecutionInfo().GetCachedRemotely()
		case *buildeventstream.BuildEventId_TestSummary:
			label := v.TestSummary.Label
			if _, ok := summaries[label]; !ok {
				summaries[label] = &testDuration{Label: label}
			}
			payload := msg.GetTestSummary()
			summaries[label].Duration = payload.GetTotalRunDuration().AsDuration()
			summaries[label].Start = payload.GetFirstStartTime().AsTime().Local()
			summaries[label].Status = payload.GetOverallStatus()
		case *buildeventstream.BuildEventId_BuildMetrics:
			metrics = extractBuildMetrics(msg.GetBuildMetrics())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]*testDuration, 0, len(summaries))
	for _, v := range summaries {
		if v.Cached {
			continue
		}
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Label < result[j].Label })
	report := &Report{Metrics: metrics}
	for _, v := range result {
		status := TestStatusFailed
		switch v.Status {
		case buildeventstream.TestStatus_FLAKY:
			status = TestStatusFlaky
		case buildeventstream.TestStatus_PASSED:
			status = TestStatusPassed
		}
		report.Tests = append(report.Tests, TestSummary{Label: v.Label, Status: status, Duration: v.Duration.Milliseconds(), StartAt: v.Start})
	}
	return report, nil
}

// extractBuildMetrics flattens the scalars we persist out of Bazel's BuildMetrics.
func extractBuildMetrics(m *buildeventstream.BuildMetrics) *BuildMetrics {
	if m == nil {
		return nil
	}
	as := m.GetActionSummary()
	tm := m.GetTimingMetrics()
	gm := m.GetBuildGraphMetrics()
	am := m.GetArtifactMetrics()
	// ActionSummary.remote_cache_hits is deprecated; derive the count from runner_count,
	// whose "remote cache hit" entry is the runner name Bazel assigns to cache hits.
	var remoteCacheHits int64
	for _, rc := range as.GetRunnerCount() {
		if rc.GetName() == "remote cache hit" {
			remoteCacheHits += int64(rc.GetCount())
		}
	}
	return &BuildMetrics{
		RemoteCacheHits:               remoteCacheHits,
		WallTimeMs:                    tm.GetWallTimeInMs(),
		CpuTimeMs:                     tm.GetCpuTimeInMs(),
		AnalysisPhaseTimeMs:           tm.GetAnalysisPhaseTimeInMs(),
		ExecutionPhaseTimeMs:          tm.GetExecutionPhaseTimeInMs(),
		ActionsCreated:                as.GetActionsCreated(),
		ActionsExecuted:               as.GetActionsExecuted(),
		GraphActionCount:              int64(gm.GetActionCount()),
		OutputArtifactCount:           int64(gm.GetOutputArtifactCount()),
		SkyframeNodeCount:             int64(gm.GetPostInvocationSkyframeNodeCount()),
		TargetsConfigured:             m.GetTargetMetrics().GetTargetsConfigured(),
		PackagesLoaded:                m.GetPackageMetrics().GetPackagesLoaded(),
		SourceArtifactsReadBytes:      am.GetSourceArtifactsRead().GetSizeInBytes(),
		SourceArtifactsReadCount:      int64(am.GetSourceArtifactsRead().GetCount()),
		OutputArtifactsSeenBytes:      am.GetOutputArtifactsSeen().GetSizeInBytes(),
		OutputArtifactsSeenCount:      int64(am.GetOutputArtifactsSeen().GetCount()),
		OutputArtifactsFromCacheBytes: am.GetOutputArtifactsFromActionCache().GetSizeInBytes(),
		OutputArtifactsFromCacheCount: int64(am.GetOutputArtifactsFromActionCache().GetCount()),
	}
}
