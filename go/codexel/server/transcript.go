package server

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/claudecode"
	"go.f110.dev/mono/go/codexel/api"
)

// transcriptPollInterval is the interval to read the transcript. Claude Code appends to the transcript during the
// session.
const transcriptPollInterval = 500 * time.Millisecond

type TranscriptHandler struct {
	projectsDir string
}

var _ api.TranscriptServiceHandler = (*TranscriptHandler)(nil)

// NewTranscriptHandler returns the handler which reads the transcripts of Claude Code in projectsDir. If projectsDir is
// empty, the handler returns CodeUnimplemented.
func NewTranscriptHandler(projectsDir string) *TranscriptHandler {
	return &TranscriptHandler{projectsDir: projectsDir}
}

func (h *TranscriptHandler) WatchTranscript(ctx context.Context, req *connect.Request[api.RequestWatchTranscript], stream *connect.ServerStream[api.ResponseWatchTranscript]) error {
	if h.projectsDir == "" {
		return connect.NewError(connect.CodeUnimplemented, errors.New("the transcript is disabled"))
	}
	sessionID := req.Msg.GetSessionId()
	if sessionID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is required"))
	}

	ticker := time.NewTicker(transcriptPollInterval)
	defer ticker.Stop()
	var transcript *claudecode.Transcript
	var title, lastPrompt string
	sent := false
	for {
		if transcript == nil {
			p, err := claudecode.FindTranscript(h.projectsDir, sessionID)
			if err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
			if p != "" {
				transcript = claudecode.NewTranscript(p)
			}
		}
		var entries []*claudecode.Entry
		newTitle, newLastPrompt := title, lastPrompt
		if transcript != nil {
			var err error
			entries, err = transcript.Update()
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			newTitle, newLastPrompt = transcript.Title(), transcript.LastPrompt()
		}

		if !sent || len(entries) > 0 || newTitle != title || newLastPrompt != lastPrompt {
			title, lastPrompt = newTitle, newLastPrompt
			res := api.ResponseWatchTranscript_builder{Title: new(title), LastPrompt: new(lastPrompt), Entries: toTranscriptEntries(entries)}.Build()
			if err := stream.Send(res); err != nil {
				return xerrors.WithStack(err)
			}
			sent = true
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func toTranscriptEntries(entries []*claudecode.Entry) []*api.TranscriptEntry {
	res := make([]*api.TranscriptEntry, 0, len(entries))
	for _, v := range entries {
		res = append(res, api.TranscriptEntry_builder{
			Kind:      new(toTranscriptEntryKind(v.Kind)),
			Time:      new(v.Time.UnixMilli()),
			Text:      new(v.Text),
			ToolName:  new(v.ToolName),
			ToolUseId: new(v.ToolUseID),
			IsError:   new(v.IsError),
		}.Build())
	}
	return res
}

func toTranscriptEntryKind(k claudecode.EntryKind) api.TranscriptEntryKind {
	switch k {
	case claudecode.EntryKindUserPrompt:
		return api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_USER_PROMPT
	case claudecode.EntryKindAssistantText:
		return api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_ASSISTANT_TEXT
	case claudecode.EntryKindToolUse:
		return api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_TOOL_USE
	case claudecode.EntryKindToolResult:
		return api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_TOOL_RESULT
	default:
		return api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_UNSPECIFIED
	}
}
