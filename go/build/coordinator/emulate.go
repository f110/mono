package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	k8sbatchv1 "k8s.io/api/batch/v1"
	k8scorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"go.f110.dev/mono/go/logger/slogger"
)

// The dev-only emulation path stands in for a real build in the local kwok
// environment: it creates the pod the job would have produced (with fixture
// logs served by kwok's Logs CRD) and marks the Job terminal, so the normal
// JobWatcher -> syncJob -> postProcess flow runs unchanged. See docs/build.md.
const (
	emulatedNodeName = "kwok-node"
	emulatedHostIP   = "127.0.0.1"
)

var kwokLogsGVR = schema.GroupVersionResource{Group: "kwok.x-k8s.io", Version: "v1alpha1", Resource: "logs"}

// EnableEmulateJobControl turns on the dev-only CompleteTask path. fixtureDir is
// the directory holding the pod-log fixtures (pre-process.log, main.log,
// report.json) served through kwok.
func (b *BazelBuilder) EnableEmulateJobControl(fixtureDir string) {
	b.emulateJobControl = true
	b.emulateFixtureDir = fixtureDir
}

// CompleteTask forces the task's Job into a terminal state in the kwok-emulated
// kube-apiserver. It only works when the builder runs with --emulate-job-control.
func (b *BazelBuilder) CompleteTask(ctx context.Context, taskID int32, success bool) error {
	if !b.emulateJobControl {
		return xerrors.Define("coordinator: emulate job control is disabled").WithStack()
	}

	task, err := b.dao.Task.Select(ctx, taskID)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if task.JobObjectName == "" {
		return xerrors.Define(fmt.Sprintf("coordinator: task %d has no job", taskID)).WithStack()
	}

	clientset, err := kubernetes.NewForConfig(b.config)
	if err != nil {
		return xerrors.WithStack(err)
	}
	dynClient, err := dynamic.NewForConfig(b.config)
	if err != nil {
		return xerrors.WithStack(err)
	}

	job, err := clientset.BatchV1().Jobs(b.Namespace).Get(ctx, task.JobObjectName, k8smetav1.GetOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}

	podName, err := b.injectEmulatedPod(ctx, clientset, dynClient, job, success)
	if err != nil {
		return err
	}

	now := k8smetav1.Now()
	condType := k8sbatchv1.JobComplete
	if !success {
		condType = k8sbatchv1.JobFailed
	}
	job.Status.StartTime = &now
	job.Status.CompletionTime = &now
	job.Status.Conditions = append(job.Status.Conditions, k8sbatchv1.JobCondition{
		Type:               condType,
		Status:             k8scorev1.ConditionTrue,
		LastProbeTime:      now,
		LastTransitionTime: now,
		Reason:             "EmulatedByMonodev",
	})
	if _, err := clientset.BatchV1().Jobs(b.Namespace).UpdateStatus(ctx, job, k8smetav1.UpdateOptions{}); err != nil {
		return xerrors.WithStack(err)
	}

	slogger.Log.Info("Completed task in emulation",
		slog.Int("task_id", int(taskID)), slog.Bool("success", success), slog.String("pod", podName))
	return nil
}

// injectEmulatedPod creates (or reuses) the single pod that postProcess expects
// for the job, fills in a terminal status, and registers the kwok Logs mapping.
func (b *BazelBuilder) injectEmulatedPod(ctx context.Context, clientset kubernetes.Interface, dynClient dynamic.Interface, job *k8sbatchv1.Job, success bool) (string, error) {
	podName := fmt.Sprintf("%s-emulated", job.Name)

	exitCode := int32(0)
	phase := k8scorev1.PodSucceeded
	if !success {
		exitCode = 1
		phase = k8scorev1.PodFailed
	}

	tmpl := job.Spec.Template
	pod := &k8scorev1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      podName,
			Namespace: b.Namespace,
			Labels:    tmpl.Labels,
		},
		Spec: k8scorev1.PodSpec{
			NodeName:       emulatedNodeName,
			RestartPolicy:  k8scorev1.RestartPolicyNever,
			InitContainers: minimalContainers(tmpl.Spec.InitContainers),
			Containers:     minimalContainers(tmpl.Spec.Containers),
			// Tolerate any node taint since the pod is placed directly by name.
			Tolerations: []k8scorev1.Toleration{{Operator: k8scorev1.TolerationOpExists}},
		},
	}
	created, err := clientset.CoreV1().Pods(b.Namespace).Create(ctx, pod, k8smetav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		created, err = clientset.CoreV1().Pods(b.Namespace).Get(ctx, podName, k8smetav1.GetOptions{})
	}
	if err != nil {
		return "", xerrors.WithStack(err)
	}

	created.Status.Phase = phase
	created.Status.HostIP = emulatedHostIP
	created.Status.InitContainerStatuses = terminatedStatuses(tmpl.Spec.InitContainers, 0)
	created.Status.ContainerStatuses = terminatedStatuses(tmpl.Spec.Containers, exitCode)
	if _, err := clientset.CoreV1().Pods(b.Namespace).UpdateStatus(ctx, created, k8smetav1.UpdateOptions{}); err != nil {
		return "", xerrors.WithStack(err)
	}

	if err := b.createKwokLogs(ctx, dynClient, podName); err != nil {
		return "", err
	}
	return podName, nil
}

// createKwokLogs registers a kwok Logs object so that GetPodLogs for the pod's
// containers returns the fixture files. The plain-text fixtures are rewritten
// into the CRI container-log format kwok requires.
func (b *BazelBuilder) createKwokLogs(ctx context.Context, dynClient dynamic.Interface, podName string) error {
	fixtures := map[string]string{
		b.jobBuilder.PreProcessContainerName: "pre-process.log",
		b.jobBuilder.BuildContainerName:      "main.log",
		b.jobBuilder.ReportContainerName:     "report.json",
	}
	tmpDir := filepath.Join(os.TempDir(), "build-emulate-logs")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return xerrors.WithStack(err)
	}
	var logs []any
	for container, name := range fixtures {
		content, err := os.ReadFile(filepath.Join(b.emulateFixtureDir, name))
		if err != nil {
			// Fixtures are optional (e.g. report.json only applies to trunk tests).
			continue
		}
		criPath := filepath.Join(tmpDir, fmt.Sprintf("%s-%s.log", podName, container))
		if err := writeCRILog(criPath, content); err != nil {
			return err
		}
		logs = append(logs, map[string]any{
			"containers": []any{container},
			"logsFile":   criPath,
		})
	}
	if len(logs) == 0 {
		return nil
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kwok.x-k8s.io/v1alpha1",
		"kind":       "Logs",
		"metadata": map[string]any{
			"name":      podName,
			"namespace": b.Namespace,
		},
		"spec": map[string]any{"logs": logs},
	}}
	_, err := dynClient.Resource(kwokLogsGVR).Namespace(b.Namespace).Create(ctx, obj, k8smetav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return xerrors.WithStack(err)
	}
	return nil
}

// writeCRILog rewrites plain-text fixture content into the CRI container-log
// format kwok requires (`<timestamp> stdout F <line>`). kwok rejects files that
// are not in a known container-log format.
func writeCRILog(path string, content []byte) error {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	var buf strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(string(content), "\n"), "\n") {
		buf.WriteString(ts)
		buf.WriteString(" stdout F ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func minimalContainers(src []k8scorev1.Container) []k8scorev1.Container {
	out := make([]k8scorev1.Container, 0, len(src))
	for _, c := range src {
		out = append(out, k8scorev1.Container{Name: c.Name, Image: c.Image})
	}
	return out
}

func terminatedStatuses(src []k8scorev1.Container, exitCode int32) []k8scorev1.ContainerStatus {
	now := k8smetav1.Now()
	out := make([]k8scorev1.ContainerStatus, 0, len(src))
	for _, c := range src {
		out = append(out, k8scorev1.ContainerStatus{
			Name:  c.Name,
			Image: c.Image,
			Ready: false,
			State: k8scorev1.ContainerState{
				Terminated: &k8scorev1.ContainerStateTerminated{ExitCode: exitCode, Reason: "Completed", FinishedAt: now},
			},
		})
	}
	return out
}
