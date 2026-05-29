package main

import (
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestAddTrailer(t *testing.T) {
	cases := []struct {
		name        string
		desc        string
		key         string
		value       string
		want        string
		wantChanged bool
	}{
		{
			name:        "empty description",
			desc:        "",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Assisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
		{
			name:        "subject only, no trailer block yet",
			desc:        "Add jj-trailer command",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Add jj-trailer command\n\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
		{
			name:        "subject + body, no trailer block",
			desc:        "Add jj-trailer command\n\nThis adds a CLI to append trailers.",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Add jj-trailer command\n\nThis adds a CLI to append trailers.\n\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
		{
			name:        "existing trailer block, append to it",
			desc:        "Add jj-trailer command\n\nSigned-off-by: Someone <a@example.com>",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Add jj-trailer command\n\nSigned-off-by: Someone <a@example.com>\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
		{
			name:        "trailer already present, no change",
			desc:        "Add jj-trailer command\n\nAssisted-by: Claude:claude-opus-4-7",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Add jj-trailer command\n\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: false,
		},
		{
			name:        "trailer present with different value, still skip",
			desc:        "Subject\n\nAssisted-by: Claude:other-model",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Subject\n\nAssisted-by: Claude:other-model",
			wantChanged: false,
		},
		{
			name:        "case-insensitive duplicate detection",
			desc:        "Subject\n\nassisted-by: Claude:foo",
			key:         "Assisted-by",
			value:       "Claude:bar",
			want:        "Subject\n\nassisted-by: Claude:foo",
			wantChanged: false,
		},
		{
			name:        "trailing newlines in input are preserved out of result",
			desc:        "Subject\n",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Subject\n\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
		{
			name:        "last paragraph mixes prose and Key: value is not a trailer block",
			desc:        "Subject\n\nSome body text.\nAssisted-by-ish: note",
			key:         "Assisted-by",
			value:       "Claude:claude-opus-4-7",
			want:        "Subject\n\nSome body text.\nAssisted-by-ish: note\n\nAssisted-by: Claude:claude-opus-4-7",
			wantChanged: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := addTrailer(tc.desc, tc.key, tc.value)
			assertion.Equal(t, got, tc.want)
			assertion.Equal(t, changed, tc.wantChanged)
		})
	}
}

func TestAppendTrailer_AllowsDuplicates(t *testing.T) {
	desc := "Subject\n\nAssisted-by: Claude:foo"
	got := appendTrailer(desc, "Assisted-by", "Claude:bar")
	assertion.Equal(t, got, "Subject\n\nAssisted-by: Claude:foo\nAssisted-by: Claude:bar")
}

func TestAppendTrailer_EmptyDescription(t *testing.T) {
	got := appendTrailer("", "Co-authored-by", "John <j@example.com>")
	assertion.Equal(t, got, "Co-authored-by: John <j@example.com>")
}

func TestAppendTrailer_StartsNewBlock(t *testing.T) {
	got := appendTrailer("Just a subject", "Co-authored-by", "John")
	assertion.Equal(t, got, "Just a subject\n\nCo-authored-by: John")
}

func TestParseRawTrailers(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []keyValue
		wantErr bool
	}{
		{
			name: "single",
			in:   []string{"Co-authored-by=John <j@example.com>"},
			want: []keyValue{{Key: "Co-authored-by", Value: "John <j@example.com>"}},
		},
		{
			name: "multiple including duplicate keys",
			in:   []string{"Reviewed-by=a", "Reviewed-by=b"},
			want: []keyValue{{Key: "Reviewed-by", Value: "a"}, {Key: "Reviewed-by", Value: "b"}},
		},
		{
			name: "value contains equals",
			in:   []string{"X-Url=https://example.com/?a=1"},
			want: []keyValue{{Key: "X-Url", Value: "https://example.com/?a=1"}},
		},
		{
			name: "empty value allowed",
			in:   []string{"Flag="},
			want: []keyValue{{Key: "Flag", Value: ""}},
		},
		{
			name:    "missing equals",
			in:      []string{"Bare"},
			wantErr: true,
		},
		{
			name:    "empty key",
			in:      []string{"=value"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRawTrailers(tc.in)
			if tc.wantErr {
				assertion.Error(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, tc.want)
		})
	}
}

func TestModelsCache_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "models.json")
	want := []anthropicModel{
		{ID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7"},
		{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6"},
	}

	assertion.MustNoError(t, writeModelsCache(path, want))

	got, err := loadModelsCache(path)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, want)
}

func TestLoadModelsCache_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	got, err := loadModelsCache(path)
	assertion.MustNoError(t, err)
	assertion.Nil(t, got)
}

func TestWriteModelsCache_Overwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	assertion.MustNoError(t, writeModelsCache(path, []anthropicModel{{ID: "old"}}))
	assertion.MustNoError(t, writeModelsCache(path, []anthropicModel{{ID: "new"}}))

	got, err := loadModelsCache(path)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, []anthropicModel{{ID: "new"}})
}
