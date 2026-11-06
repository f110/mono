// Package filter translates the filter of the tickets written in AIP-160 into the condition of SQL.
//
// The fields which the filter can use are defined in fields. The value without the field (e.g. crash or "the crash")
// matches the title, the body and the comments which are not deleted. AND, OR, NOT, - and the parentheses are
// supported, and the sequence of the terms means AND.
//
// The condition is for the table ticket and refers to the tables component, actor and comment.
package filter

import (
	"fmt"
	"strings"

	"go.einride.tech/aip/filtering"
	"go.f110.dev/xerrors"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"go.f110.dev/mono/go/issue/database"
)

var ErrInvalidFilter = xerrors.Define("invalid filter")

// fields is the fields which the filter can use. The key is the name of the field in the filter.
var fields = map[string]field{
	"status":    {column: "status", kind: enumKind{values: statuses}},
	"priority":  {column: "priority", kind: priorityKind{}},
	"type":      {column: "type", kind: enumKind{values: types, none: database.TicketTypeNone}},
	"component": {column: "component_id", kind: referenceKind{table: "component"}},
	"assignee":  {column: "assignee_id", kind: referenceKind{table: "actor"}},
	"reporter":  {column: "reporter_id", kind: referenceKind{table: "actor", required: true}},
	"title":     {column: "title", kind: textKind{}},
	"body":      {column: "body", kind: textKind{}},
}

var statuses = map[string]any{
	"OPEN":        database.TicketStatusOpen,
	"IN_PROGRESS": database.TicketStatusInProgress,
	"DONE":        database.TicketStatusDone,
	"WONT_FIX":    database.TicketStatusWontFix,
}

var types = map[string]any{
	"BUG":       database.TicketTypeBug,
	"FEATURE":   database.TicketTypeFeature,
	"TASK":      database.TicketTypeTask,
	"OPERATION": database.TicketTypeOperation,
}

// field is a field of the filter. kind decides the operators and the values.
type field struct {
	column string
	kind   kind
}

type kind interface {
	// condition returns the condition that column satisfies op with value. op is the operator of AIP-160.
	condition(column, op, value string) (string, []any, error)
}

// enumKind is the field of an enum. The value is the name in values. = and != are supported. :* (the field is set) is
// supported if none, the value which means that the field is not set, is not nil.
type enumKind struct {
	values map[string]any
	none   any
}

func (k enumKind) condition(column, op, value string) (string, []any, error) {
	if op == filtering.FunctionHas {
		if value != "*" || k.none == nil {
			return "", nil, unsupported(column, op+value)
		}
		return fmt.Sprintf("`%s` != ?", column), []any{k.none}, nil
	}
	v, ok := k.values[value]
	if !ok {
		return "", nil, xerrors.WithMessagef(ErrInvalidFilter, "unknown value of %s: %s", column, value)
	}
	switch op {
	case filtering.FunctionEquals, filtering.FunctionNotEquals:
		return fmt.Sprintf("`%s` %s ?", column, op), []any{v}, nil
	}
	return "", nil, unsupported(column, op)
}

// priorityKind is the field of the priority. The value is P0 to P4, and P0 is the highest. =, !=, <, <=, >, >= and :*
// (the priority is set) are supported. The comparison does not match the ticket without the priority.
type priorityKind struct{}

var priorities = map[string]database.Priority{
	"P0": database.PriorityP0,
	"P1": database.PriorityP1,
	"P2": database.PriorityP2,
	"P3": database.PriorityP3,
	"P4": database.PriorityP4,
}

func (priorityKind) condition(column, op, value string) (string, []any, error) {
	if op == filtering.FunctionHas {
		if value != "*" {
			return "", nil, unsupported(column, op+value)
		}
		return fmt.Sprintf("`%s` != ?", column), []any{database.PriorityNone}, nil
	}
	p, ok := priorities[value]
	if !ok {
		return "", nil, xerrors.WithMessagef(ErrInvalidFilter, "unknown priority: %s", value)
	}
	switch op {
	case filtering.FunctionEquals, filtering.FunctionNotEquals:
		return fmt.Sprintf("`%s` %s ?", column, op), []any{p}, nil
	case filtering.FunctionLessThan, filtering.FunctionLessEquals, filtering.FunctionGreaterThan, filtering.FunctionGreaterEquals:
		return fmt.Sprintf("(`%s` != ? AND `%s` %s ?)", column, column, op), []any{database.PriorityNone, p}, nil
	}
	return "", nil, unsupported(column, op)
}

// referenceKind is the field which refers to the row of table. The value is the name of the row. = and != are
// supported. :* (the field is set) is supported unless the field is required.
type referenceKind struct {
	table    string
	required bool
}

func (k referenceKind) condition(column, op, value string) (string, []any, error) {
	switch op {
	case filtering.FunctionHas:
		if value != "*" || k.required {
			return "", nil, unsupported(column, op+value)
		}
		return fmt.Sprintf("`%s` IS NOT NULL", column), nil, nil
	case filtering.FunctionEquals:
		return fmt.Sprintf("`%s` IN (SELECT `id` FROM `%s` WHERE `name` = ?)", column, k.table), []any{value}, nil
	case filtering.FunctionNotEquals:
		return fmt.Sprintf("(`%s` IS NULL OR `%s` NOT IN (SELECT `id` FROM `%s` WHERE `name` = ?))", column, column, k.table), []any{value}, nil
	}
	return "", nil, unsupported(column, op)
}

// textKind is the field of a text. : is the substring match, and = and != are the exact match.
type textKind struct{}

func (textKind) condition(column, op, value string) (string, []any, error) {
	switch op {
	case filtering.FunctionHas:
		return fmt.Sprintf("`%s` LIKE ?", column), []any{contains(value)}, nil
	case filtering.FunctionEquals, filtering.FunctionNotEquals:
		return fmt.Sprintf("`%s` %s ?", column, op), []any{value}, nil
	}
	return "", nil, unsupported(column, op)
}

// Filter is the parsed filter.
type Filter struct {
	where string
	args  []any
}

// Parse parses the filter. An empty filter matches all tickets.
func Parse(s string) (*Filter, error) {
	if strings.TrimSpace(s) == "" {
		return &Filter{}, nil
	}
	var p filtering.Parser
	p.Init(s)
	parsed, err := p.Parse()
	if err != nil {
		return nil, xerrors.WithMessage(ErrInvalidFilter, err.Error())
	}
	f := &Filter{}
	where, err := f.condition(parsed.GetExpr())
	if err != nil {
		return nil, err
	}
	f.where = where
	return f, nil
}

// Where returns the condition of WHERE clause and its arguments. The condition is empty if the filter matches all
// tickets.
func (f *Filter) Where() (string, []any) {
	return f.where, f.args
}

func (f *Filter) condition(e *expr.Expr) (string, error) {
	switch v := e.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return f.text(v.IdentExpr.GetName()), nil
	case *expr.Expr_ConstExpr:
		return f.text(constantString(v.ConstExpr)), nil
	case *expr.Expr_CallExpr:
		return f.call(v.CallExpr)
	}
	return "", xerrors.WithMessagef(ErrInvalidFilter, "unsupported expression: %v", e)
}

func (f *Filter) call(c *expr.Expr_Call) (string, error) {
	switch c.GetFunction() {
	case filtering.FunctionAnd, filtering.FunctionFuzzyAnd, filtering.FunctionOr:
		op := "AND"
		if c.GetFunction() == filtering.FunctionOr {
			op = "OR"
		}
		conds := make([]string, len(c.GetArgs()))
		for i, a := range c.GetArgs() {
			cond, err := f.condition(a)
			if err != nil {
				return "", err
			}
			conds[i] = cond
		}
		return "(" + strings.Join(conds, " "+op+" ") + ")", nil
	case filtering.FunctionNot:
		if len(c.GetArgs()) != 1 {
			return "", xerrors.WithMessage(ErrInvalidFilter, "NOT takes one argument")
		}
		cond, err := f.condition(c.GetArgs()[0])
		if err != nil {
			return "", err
		}
		return "NOT " + cond, nil
	case filtering.FunctionEquals, filtering.FunctionNotEquals, filtering.FunctionLessThan, filtering.FunctionLessEquals,
		filtering.FunctionGreaterThan, filtering.FunctionGreaterEquals, filtering.FunctionHas:
		return f.restriction(c)
	}
	return "", xerrors.WithMessagef(ErrInvalidFilter, "unsupported function: %s", c.GetFunction())
}

// restriction returns the condition of the comparison of the field. The field is looked up in fields.
func (f *Filter) restriction(c *expr.Expr_Call) (string, error) {
	if len(c.GetArgs()) != 2 {
		return "", xerrors.WithMessagef(ErrInvalidFilter, "%s takes two arguments", c.GetFunction())
	}
	name := c.GetArgs()[0].GetIdentExpr().GetName()
	fd, ok := fields[name]
	if !ok {
		return "", xerrors.WithMessagef(ErrInvalidFilter, "unknown field: %v", c.GetArgs()[0])
	}
	value, ok := literal(c.GetArgs()[1])
	if !ok {
		return "", xerrors.WithMessagef(ErrInvalidFilter, "the right side of %s must be a value", c.GetFunction())
	}
	cond, args, err := fd.kind.condition(fd.column, c.GetFunction(), value)
	if err != nil {
		return "", err
	}
	f.args = append(f.args, args...)
	return cond, nil
}

// text returns the condition that the title, the body or a comment which is not deleted contains s.
func (f *Filter) text(s string) string {
	pattern := contains(s)
	f.args = append(f.args, pattern, pattern, pattern)
	return "(`title` LIKE ? OR `body` LIKE ? OR EXISTS (SELECT 1 FROM `comment` WHERE `comment`.`ticket_id` = `ticket`.`id` AND `comment`.`deleted_at` IS NULL AND `comment`.`body` LIKE ?))"
}

// literal returns the value of the right side of the restriction. The identifier is also a value, e.g. OPEN.
func literal(e *expr.Expr) (string, bool) {
	switch v := e.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return v.IdentExpr.GetName(), true
	case *expr.Expr_ConstExpr:
		return constantString(v.ConstExpr), true
	}
	return "", false
}

func constantString(c *expr.Constant) string {
	switch v := c.GetConstantKind().(type) {
	case *expr.Constant_StringValue:
		return v.StringValue
	case *expr.Constant_Int64Value:
		return fmt.Sprint(v.Int64Value)
	case *expr.Constant_DoubleValue:
		return fmt.Sprint(v.DoubleValue)
	case *expr.Constant_BoolValue:
		return fmt.Sprint(v.BoolValue)
	}
	return ""
}

func unsupported(column, op string) error {
	return xerrors.WithMessagef(ErrInvalidFilter, "%s does not support %s", column, op)
}

func contains(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
}
