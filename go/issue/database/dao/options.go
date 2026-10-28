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
		RawConnection:      conn,
	}
}
