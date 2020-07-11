package dao

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/xerrors"

	"go.f110.dev/mono/tools/build/pkg/database"
)

type PermitPullRequest struct {
	conn *sql.DB
}

func NewPermitPullRequest(conn *sql.DB) *PermitPullRequest {
	return &PermitPullRequest{conn: conn}
}

func (p *PermitPullRequest) SelectByRepositoryAndNumber(ctx context.Context, repository string, number int32) (*database.PermitPullRequest, error) {
	row := p.conn.QueryRowContext(ctx, "SELECT `id`, `created_at`, `updated_at` FROM `permit_pull_request` WHERE `repository` = ? AND `number` = ?", repository, number)

	permitPullRequest := &database.PermitPullRequest{Repository: repository, Number: number}
	if err := row.Scan(&permitPullRequest.Id, &permitPullRequest.CreatedAt, &permitPullRequest.UpdatedAt); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return permitPullRequest, nil
}

func (p *PermitPullRequest) Create(ctx context.Context, pr *database.PermitPullRequest) (*database.PermitPullRequest, error) {
	res, err := p.conn.ExecContext(
		ctx,
		"INSERT INTO `permit_pull_request` (`repository`, `number`, `created_at`) VALUES (?, ?, ?)",
		pr.Repository, pr.Number, time.Now(),
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else if n == 0 {
		return nil, sql.ErrNoRows
	}

	pr = pr.Copy()
	insertedId, err := res.LastInsertId()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	pr.Id = int32(insertedId)

	pr.ResetMark()
	return pr, nil
}

func (p *PermitPullRequest) Delete(ctx context.Context, id int32) error {
	res, err := p.conn.ExecContext(ctx, "DELETE FROM `permit_pull_request` WHERE `id` = ?", id)
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
