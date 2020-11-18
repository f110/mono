package dao

import (
	"database/sql"
)

type Options struct {
	Repository        SourceRepositoryInterface
	Job               JobInterface
	Task              TaskInterface
	TrustedUser       TrustedUserInterface
	PermitPullRequest PermitPullRequestInterface

	RawConnection *sql.DB
}

func NewOptions(conn *sql.DB) Options {
	return Options{
		Repository:        NewSourceRepository(conn),
		Job:               NewJob(conn),
		Task:              NewTask(conn),
		TrustedUser:       NewTrustedUser(conn),
		PermitPullRequest: NewPermitPullRequest(conn),
		RawConnection:     conn,
	}
}
