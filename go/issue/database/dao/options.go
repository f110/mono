package dao

import (
	"database/sql"
)

type Options struct {
	Actor              ActorInterface
	Component          ComponentInterface
	TicketTemplate     TicketTemplateInterface
	Ticket             TicketInterface
	TicketBodyRevision TicketBodyRevisionInterface
	Relation           RelationInterface
	Mention            MentionInterface
	Comment            CommentInterface
	CommentRevision    CommentRevisionInterface
	Activity           ActivityInterface
	CommitLink         CommitLinkInterface
	PullRequestLink    PullRequestLinkInterface
	AgentSession       AgentSessionInterface
	GithubEvent        GithubEventInterface

	RawConnection *sql.DB
}

func NewOptions(conn *sql.DB) Options {
	return Options{
		Actor:              NewActor(conn),
		Component:          NewComponent(conn),
		TicketTemplate:     NewTicketTemplate(conn),
		Ticket:             NewTicket(conn),
		TicketBodyRevision: NewTicketBodyRevision(conn),
		Relation:           NewRelation(conn),
		Mention:            NewMention(conn),
		Comment:            NewComment(conn),
		CommentRevision:    NewCommentRevision(conn),
		Activity:           NewActivity(conn),
		CommitLink:         NewCommitLink(conn),
		PullRequestLink:    NewPullRequestLink(conn),
		AgentSession:       NewAgentSession(conn),
		GithubEvent:        NewGithubEvent(conn),
		RawConnection:      conn,
	}
}
