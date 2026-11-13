package filter

import (
	"errors"
	"testing"

	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/testing/assertion"
)

const textCondition = "(`title` LIKE ? OR `body` LIKE ? OR EXISTS (SELECT 1 FROM `comment` WHERE `comment`.`ticket_id` = `ticket`.`id` AND `comment`.`deleted_at` IS NULL AND `comment`.`body` LIKE ?))"

func TestParse(t *testing.T) {
	cases := []struct {
		filter    string
		wantWhere string
		wantArgs  []any
	}{
		{filter: ""},
		{filter: "status = OPEN", wantWhere: "`status` = ?", wantArgs: []any{database.TicketStatusOpen}},
		{filter: "status != DONE", wantWhere: "`status` != ?", wantArgs: []any{database.TicketStatusDone}},
		{filter: "status = NEED_INVESTIGATION", wantWhere: "`status` = ?", wantArgs: []any{database.TicketStatusNeedInvestigation}},
		{
			filter:    "status = OPEN OR status = IN_PROGRESS",
			wantWhere: "(`status` = ? OR `status` = ?)",
			wantArgs:  []any{database.TicketStatusOpen, database.TicketStatusInProgress},
		},
		{filter: "priority = P1", wantWhere: "`priority` = ?", wantArgs: []any{database.PriorityP1}},
		{
			filter:    "priority < P2",
			wantWhere: "(`priority` != ? AND `priority` < ?)",
			wantArgs:  []any{database.PriorityNone, database.PriorityP2},
		},
		{filter: "priority:*", wantWhere: "`priority` != ?", wantArgs: []any{database.PriorityNone}},
		{filter: "-priority:*", wantWhere: "NOT `priority` != ?", wantArgs: []any{database.PriorityNone}},
		{filter: "type = BUG", wantWhere: "`type` = ?", wantArgs: []any{database.TicketTypeBug}},
		{filter: "type != OPERATION", wantWhere: "`type` != ?", wantArgs: []any{database.TicketTypeOperation}},
		{filter: "type:*", wantWhere: "`type` != ?", wantArgs: []any{database.TicketTypeNone}},
		{
			filter:    `component = "build system"`,
			wantWhere: "`component_id` IN (SELECT `id` FROM `component` WHERE `name` = ?)",
			wantArgs:  []any{"build system"},
		},
		{
			filter:    "assignee != alice",
			wantWhere: "(`assignee_id` IS NULL OR `assignee_id` NOT IN (SELECT `id` FROM `actor` WHERE `name` = ?))",
			wantArgs:  []any{"alice"},
		},
		{filter: "NOT assignee:*", wantWhere: "NOT `assignee_id` IS NOT NULL"},
		{
			filter:    "reporter = bob",
			wantWhere: "`reporter_id` IN (SELECT `id` FROM `actor` WHERE `name` = ?)",
			wantArgs:  []any{"bob"},
		},
		{filter: `title:"50%"`, wantWhere: "`title` LIKE ?", wantArgs: []any{`%50\%%`}},
		{filter: "body = done", wantWhere: "`body` = ?", wantArgs: []any{"done"}},
		{filter: "crash", wantWhere: textCondition, wantArgs: []any{"%crash%", "%crash%", "%crash%"}},
		{filter: `"the crash"`, wantWhere: textCondition, wantArgs: []any{"%the crash%", "%the crash%", "%the crash%"}},
		{
			filter:    "status = OPEN クラッシュ",
			wantWhere: "(`status` = ? AND " + textCondition + ")",
			wantArgs:  []any{database.TicketStatusOpen, "%クラッシュ%", "%クラッシュ%", "%クラッシュ%"},
		},
		{
			filter:    "(status = OPEN OR status = IN_PROGRESS) AND priority <= P1 AND component = build",
			wantWhere: "(((`status` = ? OR `status` = ?) AND (`priority` != ? AND `priority` <= ?)) AND `component_id` IN (SELECT `id` FROM `component` WHERE `name` = ?))",
			wantArgs:  []any{database.TicketStatusOpen, database.TicketStatusInProgress, database.PriorityNone, database.PriorityP1, "build"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			f, err := Parse(tc.filter)
			assertion.MustNoError(t, err)
			where, args := f.Where()
			assertion.Equal(t, where, tc.wantWhere)
			assertion.Equal(t, args, tc.wantArgs)
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, filter := range []string{
		"status = CLOSED",
		"status < OPEN",
		"priority = P5",
		"priority:P1",
		"type = NONE",
		"type < BUG",
		"type:BUG",
		"status:*",
		"component:build",
		"reporter:*",
		"unknown = 1",
		"title > a",
		"status =",
	} {
		t.Run(filter, func(t *testing.T) {
			_, err := Parse(filter)
			if !errors.Is(err, ErrInvalidFilter) {
				t.Fatalf("expected ErrInvalidFilter but got %v", err)
			}
		})
	}
}
