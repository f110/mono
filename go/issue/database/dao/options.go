package dao

import (
	"database/sql"
)

type Options struct {
	Actor              ActorInterface
	Component          ComponentInterface
	Ticket             TicketInterface
	TicketBodyRevision TicketBodyRevisionInterface
	Relation           RelationInterface
	Comment            CommentInterface
	CommentRevision    CommentRevisionInterface
	Activity           ActivityInterface
	CommitLink         CommitLinkInterface
	PullRequestLink    PullRequestLinkInterface
	GithubEvent        GithubEventInterface

	RawConnection *sql.DB
}

func NewOptions(conn *sql.DB) Options {
	return Options{
		Actor:              NewActor(conn),
		Component:          NewComponent(conn),
		Ticket:             NewTicket(conn),
		TicketBodyRevision: NewTicketBodyRevision(conn),
		Relation:           NewRelation(conn),
		Comment:            NewComment(conn),
		CommentRevision:    NewCommentRevision(conn),
		Activity:           NewActivity(conn),
		CommitLink:         NewCommitLink(conn),
		PullRequestLink:    NewPullRequestLink(conn),
		GithubEvent:        NewGithubEvent(conn),
		RawConnection:      conn,
	}
}
