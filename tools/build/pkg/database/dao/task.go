package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"go.f110.dev/mono/tools/build/pkg/database"
)

type Task struct {
	conn *sql.DB
	job  *Job
}

func NewTask(conn *sql.DB) *Task {
	return &Task{conn: conn, job: NewJob(conn)}
}

func (t *Task) ListByJob(ctx context.Context, jobId int32) ([]*database.Task, error) {
	rows, err := t.conn.QueryContext(ctx, "SELECT `id`, `revision`, `success`, `log_file`, `via`, `command`, `target`, `finished_at`, `created_at`, `updated_at` FROM `task` WHERE `job_id` = ?", jobId)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	result := make([]*database.Task, 0)
	for rows.Next() {
		task := &database.Task{JobId: jobId, Job: &database.Job{Id: jobId}}
		if err := rows.Scan(&task.Id, &task.Revision, &task.Success, &task.LogFile, &task.Via, &task.Command, &task.Target, &task.FinishedAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		task.ResetMark()
		result = append(result, task)
	}

	return result, nil
}

func (t *Task) SelectById(ctx context.Context, id int32) (*database.Task, error) {
	row := t.conn.QueryRowContext(ctx, "SELECT `job_id`, `revision`, `success`, `log_file`, `via`, `command`, `target`, `finished_at`, `created_at`, `updated_at` FROM `task` WHERE `id` = ?", id)

	task := &database.Task{Id: id}
	if err := row.Scan(&task.JobId, &task.Revision, &task.Success, &task.LogFile, &task.Via, &task.Command, &task.Target, &task.FinishedAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if task.JobId > 0 {
		job, err := t.job.SelectById(ctx, task.JobId)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		task.Job = job
	}

	task.ResetMark()
	return task, nil
}

func (t *Task) Update(ctx context.Context, task *database.Task) error {
	if !task.IsChanged() {
		return nil
	}

	changedColumn := task.ChangedColumn()
	cols := make([]string, len(changedColumn)+1)
	values := make([]interface{}, len(changedColumn)+1)
	for i := range changedColumn {
		cols[i] = "`" + changedColumn[i].Name + "` = ?"
		values[i] = changedColumn[i].Value
	}
	cols[len(cols)-1] = "`updated_at` = ?"
	values[len(values)-1] = time.Now()

	query := fmt.Sprintf("UPDATE `task` SET %s WHERE `id` = ?", strings.Join(cols, ", "))
	res, err := t.conn.ExecContext(
		ctx,
		query,
		append(values, task.Id)...,
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return sql.ErrNoRows
	}

	task.ResetMark()
	return nil
}

func (t *Task) Create(ctx context.Context, task *database.Task) (*database.Task, error) {
	res, err := t.conn.ExecContext(
		ctx,
		"INSERT INTO `task` (`job_id`, `revision`, `success`, `log_file`, `via`, `command`, `target`, `finished_at`, `created_at`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		task.JobId, task.Revision, task.Success, task.LogFile, task.Via, task.Command, task.Target, task.FinishedAt, time.Now(),
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return nil, sql.ErrNoRows
	}

	task = task.Copy()
	insertedId, err := res.LastInsertId()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	task.Id = int32(insertedId)

	task.ResetMark()
	return task, nil
}
