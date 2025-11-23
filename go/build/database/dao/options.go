package dao

import (
	"database/sql"
)

type Options struct {
	Repository        SourceRepositoryInterface
	Task              TaskInterface
	TrustedUser       TrustedUserInterface
	PermitPullRequest PermitPullRequestInterface
	TestReport        TestReportInterface
	Job               JobInterface

	RawConnection *sql.DB
}

func NewOptions(conn *sql.DB) Options {
	return Options{
		Repository:        NewSourceRepository(conn),
		Task:              NewTask(conn),
		TrustedUser:       NewTrustedUser(conn),
		PermitPullRequest: NewPermitPullRequest(conn),
		TestReport:        NewTestReport(conn),
		Job:               NewJob(conn),
		RawConnection:     conn,
	}
}
