package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/encoding/protodelim"

	"go.f110.dev/mono/go/bazel/buildeventstream"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/file"
)

type TestReport struct {
	Tests []TestSummary `json:"tests"`
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
	ch := make(chan struct{})
	go func() {
		for {
			if _, err := os.Lstat(b.eventBinaryFile); err == nil {
				close(ch)
				break
			}
		}
	}()
	select {
	case <-ch:
	case <-time.After(b.startUpTimeout):
		return xerrors.Definef("could not find event binary file in %v", b.startUpTimeout).WithStack()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := watcher.Add(b.eventBinaryFile); err != nil {
		return xerrors.WithStack(err)
	}

	f, err := os.Open(b.eventBinaryFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	r, err := file.NewTailReader(f)
	if err != nil {
		return err
	}

	summaries := make(map[string]*testDuration)
	var msg buildeventstream.BuildEvent
Read:
	for {
		err := protodelim.UnmarshalFrom(r, &msg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xerrors.WithStack(err)
		}

		switch v := msg.Id.Id.(type) {
		case *buildeventstream.BuildEventId_BuildFinished:
			break Read
		case *buildeventstream.BuildEventId_TestResult:
			payload := msg.Payload.(*buildeventstream.BuildEvent_TestResult)
			if _, ok := summaries[v.TestResult.Label]; !ok {
				summaries[v.TestResult.Label] = &testDuration{Label: v.TestResult.Label}
			}
			summaries[v.TestResult.Label].Cached = payload.TestResult.ExecutionInfo.CachedRemotely
		case *buildeventstream.BuildEventId_TestSummary:
			payload := msg.Payload.(*buildeventstream.BuildEvent_TestSummary)
			if _, ok := summaries[v.TestSummary.Label]; !ok {
				summaries[v.TestSummary.Label] = &testDuration{Label: v.TestSummary.Label}
			}
			summaries[v.TestSummary.Label].Duration = payload.TestSummary.TotalRunDuration.AsDuration()
			summaries[v.TestSummary.Label].Start = payload.TestSummary.FirstStartTime.AsTime().Local()
			summaries[v.TestSummary.Label].Status = payload.TestSummary.OverallStatus
		}
	}
	result := make([]*testDuration, 0, len(summaries))
	for _, v := range summaries {
		if v.Cached {
			continue
		}
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Label < result[j].Label })
	var report TestReport
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

	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	return nil
}
