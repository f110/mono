package dao

import (
	"database/sql"
)

type Options struct {
	Repository        *SourceRepository
	Job               *Job
	Task              *Task
	TrustedUser       *TrustedUser
	PermitPullRequest *PermitPullRequest

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
