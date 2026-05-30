package api

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestExtractRepositoryFromPayload(t *testing.T) {
	cases := []struct {
		name      string
		payload   string
		wantName  string
		wantURL   string
	}{
		{
			name:     "pull_request style payload",
			payload:  `{"action":"opened","repository":{"full_name":"f110/ops","html_url":"https://github.com/f110/ops"}}`,
			wantName: "f110/ops",
			wantURL:  "https://github.com/f110/ops",
		},
		{
			name:     "empty payload",
			payload:  ``,
			wantName: "",
			wantURL:  "",
		},
		{
			name:     "no repository field",
			payload:  `{"action":"created"}`,
			wantName: "",
			wantURL:  "",
		},
		{
			name:     "malformed JSON falls back to empty",
			payload:  `{"action": broken`,
			wantName: "",
			wantURL:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotURL := extractRepositoryFromPayload([]byte(tc.payload))
			assertion.Equal(t, tc.wantName, gotName)
			assertion.Equal(t, tc.wantURL, gotURL)
		})
	}
}
