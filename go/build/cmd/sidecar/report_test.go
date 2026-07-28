package sidecar

import (
	"bytes"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/bazel/buildeventstream"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseReport(t *testing.T) {
	start := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	// BuildMetrics is emitted AFTER BuildFinished and carries last_message=true. The
	// fixture reproduces that ordering so a parser that stops at BuildFinished fails.
	events := []*buildeventstream.BuildEvent{
		testResultEvent("//foo:bar", false),
		testSummaryEvent("//foo:bar", buildeventstream.TestStatus_PASSED, 500*time.Millisecond, start),
		testResultEvent("//foo:cached", true),
		testSummaryEvent("//foo:cached", buildeventstream.TestStatus_PASSED, time.Second, start),
		buildFinishedEvent(),
		buildMetricsEvent(),
	}
	var buf bytes.Buffer
	for _, ev := range events {
		if _, err := protodelim.MarshalTo(&buf, ev); err != nil {
			t.Fatal(err)
		}
	}

	report, err := parseReport(&buf)
	assertion.MustNoError(t, err)

	// Remotely cached tests are excluded, so only //foo:bar survives.
	assertion.Equal(t, len(report.Tests), 1)
	assertion.Equal(t, report.Tests[0].Label, "//foo:bar")
	assertion.Equal(t, report.Tests[0].Status, TestStatusPassed)
	assertion.Equal(t, report.Tests[0].Duration, int64(500))

	// Metrics being non-nil proves the BuildMetrics event after BuildFinished was reached.
	assertion.NotNil(t, report.Metrics)
	assertion.Equal(t, report.Metrics.RemoteCacheHits, int64(120))
	assertion.Equal(t, report.Metrics.WallTimeMs, int64(5270))
	assertion.Equal(t, report.Metrics.CpuTimeMs, int64(28589))
	assertion.Equal(t, report.Metrics.ActionsCreated, int64(890))
	assertion.Equal(t, report.Metrics.ActionsExecuted, int64(5))
	assertion.Equal(t, report.Metrics.OutputArtifactsFromCacheBytes, int64(1080163452))
	assertion.Equal(t, report.Metrics.OutputArtifactsFromCacheCount, int64(2144))
}

func testResultEvent(label string, cachedRemotely bool) *buildeventstream.BuildEvent {
	return &buildeventstream.BuildEvent{
		Id: &buildeventstream.BuildEventId{
			Id: &buildeventstream.BuildEventId_TestResult{
				TestResult: &buildeventstream.BuildEventId_TestResultId{Label: label},
			},
		},
		Payload: &buildeventstream.BuildEvent_TestResult{
			TestResult: &buildeventstream.TestResult{
				ExecutionInfo: &buildeventstream.TestResult_ExecutionInfo{CachedRemotely: cachedRemotely},
			},
		},
	}
}

func testSummaryEvent(label string, status buildeventstream.TestStatus, d time.Duration, start time.Time) *buildeventstream.BuildEvent {
	return &buildeventstream.BuildEvent{
		Id: &buildeventstream.BuildEventId{
			Id: &buildeventstream.BuildEventId_TestSummary{
				TestSummary: &buildeventstream.BuildEventId_TestSummaryId{Label: label},
			},
		},
		Payload: &buildeventstream.BuildEvent_TestSummary{
			TestSummary: &buildeventstream.TestSummary{
				OverallStatus:    status,
				TotalRunDuration: durationpb.New(d),
				FirstStartTime:   timestamppb.New(start),
			},
		},
	}
}

func buildFinishedEvent() *buildeventstream.BuildEvent {
	return &buildeventstream.BuildEvent{
		Id: &buildeventstream.BuildEventId{
			Id: &buildeventstream.BuildEventId_BuildFinished{
				BuildFinished: &buildeventstream.BuildEventId_BuildFinishedId{},
			},
		},
		Payload: &buildeventstream.BuildEvent_Finished{Finished: &buildeventstream.BuildFinished{}},
	}
}

func buildMetricsEvent() *buildeventstream.BuildEvent {
	return &buildeventstream.BuildEvent{
		LastMessage: true,
		Id: &buildeventstream.BuildEventId{
			Id: &buildeventstream.BuildEventId_BuildMetrics{
				BuildMetrics: &buildeventstream.BuildEventId_BuildMetricsId{},
			},
		},
		Payload: &buildeventstream.BuildEvent_BuildMetrics{
			BuildMetrics: &buildeventstream.BuildMetrics{
				ActionSummary: &buildeventstream.BuildMetrics_ActionSummary{
					ActionsCreated:  890,
					ActionsExecuted: 5,
					RunnerCount: []*buildeventstream.BuildMetrics_ActionSummary_RunnerCount{
						{Name: "remote cache hit", Count: 120, ExecKind: "Remote"},
						{Name: "internal", Count: 3},
					},
				},
				TimingMetrics: &buildeventstream.BuildMetrics_TimingMetrics{
					WallTimeInMs:           5270,
					CpuTimeInMs:            28589,
					AnalysisPhaseTimeInMs:  3361,
					ExecutionPhaseTimeInMs: 1449,
				},
				ArtifactMetrics: &buildeventstream.BuildMetrics_ArtifactMetrics{
					OutputArtifactsFromActionCache: &buildeventstream.BuildMetrics_ArtifactMetrics_FilesMetric{SizeInBytes: 1080163452, Count: 2144},
				},
			},
		},
	}
}
