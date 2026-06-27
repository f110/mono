package k8smanifest

import (
	"testing"

	"go.f110.dev/kubeproto/go/apis/batchv1"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestMarshalSetsGVKForJob(t *testing.T) {
	// Marshal must fill in apiVersion and kind for a built-in Job so the emitted
	// manifest applies standalone.
	buf, err := Marshal(&batchv1.Job{})
	assertion.MustNoError(t, err)

	out := string(buf)
	assertion.Contains(t, out, "apiVersion: batch/v1")
	assertion.Contains(t, out, "kind: Job")
}
