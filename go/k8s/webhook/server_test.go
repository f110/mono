package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestServer_Validate(t *testing.T) {
	s := NewServer(":0", "", "")
	s.Register(schema.GroupKind{Group: "test.f110.dev", Kind: "Denied"}, func(_ *admissionv1.AdmissionRequest) error {
		return errors.New("denied")
	})

	cases := []struct {
		Kind    string
		Allowed bool
		Message string
	}{
		{Kind: "Denied", Allowed: false, Message: "denied"},
		{Kind: "Unknown", Allowed: true},
	}
	for _, tc := range cases {
		t.Run(tc.Kind, func(t *testing.T) {
			review := &admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
				Request: &admissionv1.AdmissionRequest{
					UID:  types.UID("uid"),
					Kind: metav1.GroupVersionKind{Group: "test.f110.dev", Version: "v1alpha1", Kind: tc.Kind},
				},
			}
			body, err := json.Marshal(review)
			assertion.MustNoError(t, err)
			rec := httptest.NewRecorder()

			s.validate(rec, httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body)))

			assertion.Equal(t, rec.Code, http.StatusOK)
			res := &admissionv1.AdmissionReview{}
			assertion.MustNoError(t, json.Unmarshal(rec.Body.Bytes(), res))
			assertion.Equal(t, res.TypeMeta, review.TypeMeta)
			assertion.Equal(t, res.Response.UID, types.UID("uid"))
			assertion.Equal(t, res.Response.Allowed, tc.Allowed)
			if tc.Message != "" {
				assertion.Equal(t, res.Response.Result.Message, tc.Message)
			}
		})
	}
}
