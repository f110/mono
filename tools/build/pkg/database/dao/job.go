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

type Job struct {
	conn *sql.DB

	repository *SourceRepository
}

func NewJob(conn *sql.DB) *Job {
	return &Job{conn: conn, repository: NewSourceRepository(conn)}
}

func (j *Job) List(ctx context.Context) ([]*database.Job, error) {
	rows, err := j.conn.QueryContext(
		ctx,
		"SELECT `id`, `repository_id`, `command`, `target`, `active`, `all_revision`, `github_status`, `cpu_limit`, `memory_limit`, `created_at`, `updated_at` FROM `job`",
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	sourceRepositoryIds := make([]interface{}, 0)
	result := make([]*database.Job, 0)
	for rows.Next() {
		j := &database.Job{}
		if err := rows.Scan(&j.Id, &j.RepositoryId, &j.Command, &j.Target, &j.Active, &j.AllRevision, &j.GithubStatus, &j.CpuLimit, &j.MemoryLimit, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		j.ResetMark()
		result = append(result, j)
		sourceRepositoryIds = append(sourceRepositoryIds, j.RepositoryId)
	}

	if len(sourceRepositoryIds) > 0 {
		rows, err = j.conn.QueryContext(
			ctx,
			"SELECT `id`, `url`, `clone_url`, `name` FROM `source_repository` WHERE `id` IN (?"+strings.Repeat(",?", len(sourceRepositoryIds)-1)+")",
			sourceRepositoryIds...,
		)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}

		sourceRepositories := make(map[int32]*database.SourceRepository)
		for rows.Next() {
			s := &database.SourceRepository{}
			if err := rows.Scan(&s.Id, &s.Url, &s.CloneUrl, &s.Name); err != nil {
				return nil, xerrors.Errorf(": %w", err)
			}
			s.ResetMark()
			sourceRepositories[s.Id] = s
		}

		for _, v := range result {
			if sr, ok := sourceRepositories[v.RepositoryId]; ok {
				v.Repository = sr
			}
		}
	}

	return result, nil
}

func (j *Job) ListBySourceRepositoryId(ctx context.Context, repositoryId int32) ([]*database.Job, error) {
	rows, err := j.conn.QueryContext(
		ctx,
		"SELECT `id`, `command`, `target`, `active`, `all_revision`, `github_status`, `cpu_limit`, `memory_limit`, `created_at`, `updated_at` FROM `job` WHERE `repository_id` = ?",
		repositoryId,
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	result := make([]*database.Job, 0)
	for rows.Next() {
		newJob := &database.Job{RepositoryId: repositoryId}
		if err := rows.Scan(&newJob.Id, &newJob.Command, &newJob.Target, &newJob.Active, &newJob.AllRevision, &newJob.GithubStatus, &newJob.CpuLimit, &newJob.MemoryLimit, &newJob.CreatedAt, &newJob.UpdatedAt); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		newJob.ResetMark()
		result = append(result, newJob)
	}
	if len(result) > 0 {
		repo, err := j.repository.SelectById(ctx, repositoryId)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		for _, v := range result {
			v.Repository = repo
		}
	}

	return result, nil
}

func (j *Job) SelectById(ctx context.Context, id int32) (*database.Job, error) {
	row := j.conn.QueryRowContext(
		ctx,
		"SELECT `repository_id`, `command`, `target`, `active`, `all_revision`, `github_status`, `cpu_limit`, `memory_limit`, `created_at`, `updated_at` FROM `job` WHERE `id` = ?",
		id,
	)

	job := &database.Job{Id: id}
	if err := row.Scan(&job.RepositoryId, &job.Command, &job.Target, &job.Active, &job.AllRevision, &job.GithubStatus, &job.CpuLimit, &job.MemoryLimit, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if v, err := j.repository.SelectById(ctx, job.RepositoryId); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else {
		job.Repository = v
	}

	job.ResetMark()
	return job, nil
}

func (j *Job) Update(ctx context.Context, job *database.Job) error {
	if !job.IsChanged() {
		return nil
	}

	changedColumn := job.ChangedColumn()
	cols := make([]string, len(changedColumn)+1)
	values := make([]interface{}, len(changedColumn)+1)
	for i := range changedColumn {
		cols[i] = "`" + changedColumn[i].Name + "` = ?"
		values[i] = changedColumn[i].Value
	}
	cols[len(cols)-1] = "`updated_at` = ?"
	values[len(cols)-1] = time.Now()

	query := fmt.Sprintf("UPDATE `job` SET %s WHERE `id` = ?", strings.Join(cols, ", "))
	res, err := j.conn.ExecContext(ctx, query, append(values, job.Id)...)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return sql.ErrNoRows
	}

	job.ResetMark()
	return nil
}

func (j *Job) Create(ctx context.Context, job *database.Job) (*database.Job, error) {
	res, err := j.conn.ExecContext(
		ctx,
		"INSERT INTO `job` (`repository_id`, `command`, `target`, `active`, `all_revision`, `github_status`, `cpu_liimt`, `memory_limit`, `created_at`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		job.RepositoryId, job.Command, job.Target, job.Active, job.AllRevision, job.GithubStatus, job.CpuLimit, job.MemoryLimit, time.Now(),
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return nil, sql.ErrNoRows
	}

	job = job.Copy()
	insertedId, err := res.LastInsertId()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	job.Id = int32(insertedId)

	job.ResetMark()
	return job, nil
}
