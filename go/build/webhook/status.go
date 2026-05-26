package webhook

import "time"

// PushStatus is the progress checkpoint for a `push` event reconciler. Each
// optional field is set as the corresponding step succeeds so that re-running
// the reconciler resumes from the right point.
type PushStatus struct {
	Skipped              bool       `json:"skipped,omitempty"`
	SkipReason           string     `json:"skip_reason,omitempty"`
	ConfigFetchedAt      *time.Time `json:"config_fetched_at,omitempty"`
	ExternalReconciledAt *time.Time `json:"external_reconciled_at,omitempty"`
	DispatchedTaskIDs    []int32    `json:"dispatched_task_ids,omitempty"`
}

// PullRequestStatus is the progress checkpoint for a `pull_request` event.
type PullRequestStatus struct {
	Skipped           bool    `json:"skipped,omitempty"`
	SkipReason        string  `json:"skip_reason,omitempty"`
	NotAllowed        bool    `json:"not_allowed,omitempty"`
	CommentPosted     bool    `json:"comment_posted,omitempty"`
	DispatchedTaskIDs []int32 `json:"dispatched_task_ids,omitempty"`
	ConfigValidated   bool    `json:"config_validated,omitempty"`
	PermitDeletedId   int32   `json:"permit_deleted_id,omitempty"`
}

// ReleaseStatus is the progress checkpoint for a `release` event.
type ReleaseStatus struct {
	Skipped           bool    `json:"skipped,omitempty"`
	SkipReason        string  `json:"skip_reason,omitempty"`
	DispatchedTaskIDs []int32 `json:"dispatched_task_ids,omitempty"`
}

// IssueCommentStatus is the progress checkpoint for an `issue_comment` event.
type IssueCommentStatus struct {
	Skipped           bool    `json:"skipped,omitempty"`
	SkipReason        string  `json:"skip_reason,omitempty"`
	PermitCreatedId   int32   `json:"permit_created_id,omitempty"`
	CommentPosted     bool    `json:"comment_posted,omitempty"`
	DispatchedTaskIDs []int32 `json:"dispatched_task_ids,omitempty"`
}
