package dao

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/xerrors"

	"go.f110.dev/mono/tools/build/pkg/database"
)

type SourceRepository struct {
	conn *sql.DB
}

func NewSourceRepository(conn *sql.DB) *SourceRepository {
	return &SourceRepository{conn: conn}
}

func (r *SourceRepository) List(ctx context.Context) ([]*database.SourceRepository, error) {
	rows, err := r.conn.QueryContext(ctx, "SELECT `id`, `url`, `clone_url`, `name`, `created_at`, `updated_at` FROM `source_repository`")
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	res := make([]*database.SourceRepository, 0)
	for rows.Next() {
		r := &database.SourceRepository{}
		if err := rows.Scan(&r.Id, &r.Url, &r.CloneUrl, &r.Name, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		r.ResetMark()
		res = append(res, r)
	}

	return res, nil
}

func (r *SourceRepository) SelectById(ctx context.Context, id int32) (*database.SourceRepository, error) {
	row := r.conn.QueryRowContext(ctx, "SELECT `url`, `clone_url`, `name`, `created_at`, `updated_at` FROM `source_repository` WHERE `id` = ?", id)

	repo := &database.SourceRepository{Id: id}
	if err := row.Scan(&repo.Url, &repo.CloneUrl, &repo.Name, &repo.CreatedAt, &repo.UpdatedAt); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	repo.ResetMark()
	return repo, nil
}

func (r *SourceRepository) SelectByUrl(ctx context.Context, url string) (*database.SourceRepository, error) {
	row := r.conn.QueryRowContext(ctx, "SELECT `id`, `clone_url`, `name`, `created_at`, `updated_at` FROM `source_repository` WHERE `url` = ?", url)

	repo := &database.SourceRepository{Url: url}
	if err := row.Scan(&repo.Id, &repo.CloneUrl, &repo.Name, &repo.CreatedAt, &repo.UpdatedAt); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	repo.ResetMark()
	return repo, nil
}

func (r *SourceRepository) Create(ctx context.Context, repo *database.SourceRepository) (*database.SourceRepository, error) {
	res, err := r.conn.ExecContext(ctx, "INSERT INTO `source_repository` (`url`, `clone_url`, `name`, `created_at`) VALUES (?, ?, ?, ?)", repo.Url, repo.CloneUrl, repo.Name, time.Now())
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return nil, sql.ErrNoRows
	}

	repo = repo.Copy()
	insertedId, err := res.LastInsertId()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	repo.Id = int32(insertedId)

	repo.ResetMark()
	return repo, nil
}

func (r *SourceRepository) Delete(ctx context.Context, id int32) error {
	res, err := r.conn.ExecContext(ctx, "DELETE FROM `source_repository` WHERE `id` = ?", id)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}
