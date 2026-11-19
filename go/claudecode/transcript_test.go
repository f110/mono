package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFindTranscript(t *testing.T) {
	dir := t.TempDir()
	assertion.MustNoError(t, os.MkdirAll(filepath.Join(dir, "-opt-repo"), 0755))
	p := filepath.Join(dir, "-opt-repo", "3f1c-abcd.jsonl")
	assertion.MustNoError(t, os.WriteFile(p, nil, 0644))

	found, err := FindTranscript(dir, "3f1c-abcd")
	assertion.MustNoError(t, err)
	assertion.Equal(t, found, p)

	found, err = FindTranscript(dir, "unknown")
	assertion.MustNoError(t, err)
	assertion.Equal(t, found, "")

	_, err = FindTranscript(dir, "../x")
	assertion.MustError(t, err)
	_, err = FindTranscript(dir, "*")
	assertion.MustError(t, err)
}

const transcriptLines = `{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command-caveat>caveat</local-command-caveat>"},"timestamp":"2026-11-18T10:00:00Z"}
{"type":"user","message":{"role":"user","content":"<command-name>/clear</command-name>"},"timestamp":"2026-11-18T10:00:00Z"}
{"type":"user","origin":{"kind":"human"},"message":{"role":"user","content":"Fix the bug\nwith the details"},"timestamp":"2026-11-18T10:00:01Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"}]},"timestamp":"2026-11-18T10:00:02Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll look at it."}],"usage":{"input_tokens":3,"output_tokens":20,"cache_read_input_tokens":1000,"cache_creation_input_tokens":200}},"timestamp":"2026-11-18T10:00:03Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls","description":"List"}}],"usage":{"input_tokens":3,"output_tokens":20,"cache_read_input_tokens":1000,"cache_creation_input_tokens":200}},"timestamp":"2026-11-18T10:00:04Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"a.go","is_error":false}]},"toolUseResult":{"stdout":"a.go"},"timestamp":"2026-11-18T10:00:05Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Read","input":{"file_path":"/repo/a.go"}}]},"timestamp":"2026-11-18T10:00:06Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"no such file"}],"is_error":true}]},"timestamp":"2026-11-18T10:00:07Z"}
{"type":"assistant","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"subagent"}]},"timestamp":"2026-11-18T10:00:08Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t3","name":"mcp__issue__get_ticket","input":{"number":1}}]},"timestamp":"2026-11-18T10:00:09Z"}
{"type":"last-prompt","lastPrompt":"Fix the bug\nwith the details"}
broken line
`

func TestTranscript(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	assertion.MustNoError(t, os.WriteFile(p, []byte(transcriptLines), 0644))

	tr := NewTranscript(p)
	entries, err := tr.Update()
	assertion.MustNoError(t, err)
	at := func(sec int) time.Time { return time.Date(2026, 11, 18, 10, 0, sec, 0, time.UTC) }
	usage := &Usage{InputTokens: 3, OutputTokens: 20, CacheReadInputTokens: 1000, CacheCreationInputTokens: 200}
	assertion.Equal(t, entries, []*Entry{
		{Kind: EntryKindUserPrompt, Time: at(1), Text: "Fix the bug\nwith the details"},
		{Kind: EntryKindAssistantText, Time: at(3), Text: "I'll look at it.", Usage: usage},
		{Kind: EntryKindToolUse, Time: at(4), Text: "ls", ToolName: "Bash", ToolUseID: "t1", Usage: usage},
		{Kind: EntryKindToolResult, Time: at(5), Text: "a.go", ToolUseID: "t1"},
		{Kind: EntryKindToolUse, Time: at(6), Text: "/repo/a.go", ToolName: "Read", ToolUseID: "t2"},
		{Kind: EntryKindToolResult, Time: at(7), Text: "no such file", ToolUseID: "t2", IsError: true},
		{Kind: EntryKindToolUse, Time: at(9), Text: `{"number":1}`, ToolName: "mcp__issue__get_ticket", ToolUseID: "t3"},
	})
	assertion.Equal(t, tr.Title(), "Fix the bug")
	assertion.Equal(t, tr.LastPrompt(), "Fix the bug\nwith the details")

	entries, err = tr.Update()
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 0)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	assertion.MustNoError(t, err)
	defer f.Close()
	_, err = f.WriteString(`{"type":"ai-title","aiTitle":"Bug fix"}` + "\n" + `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done"}]},`)
	assertion.MustNoError(t, err)
	entries, err = tr.Update()
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 0)
	assertion.Equal(t, tr.Title(), "Bug fix")

	_, err = f.WriteString(`"timestamp":"2026-11-18T10:00:10Z"}` + "\n")
	assertion.MustNoError(t, err)
	entries, err = tr.Update()
	assertion.MustNoError(t, err)
	assertion.Equal(t, entries, []*Entry{{Kind: EntryKindAssistantText, Time: at(10), Text: "Done"}})
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("あ", maxTextLength)
	got := truncate(s)
	assertion.True(t, strings.HasSuffix(got, "…"))
	assertion.True(t, len(got) <= maxTextLength+len("…"))
	assertion.Equal(t, truncate("abc"), "abc")
}
