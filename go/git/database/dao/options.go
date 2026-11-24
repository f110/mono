package dao

import (
	"database/sql"
)

type Options struct {
	Repository RepositoryInterface
	Object     ObjectInterface
	TreeEntry  TreeEntryInterface
	Reference  ReferenceInterface
}

func NewOptions(conn *sql.DB) Options {
	return Options{
		Repository: NewRepository(conn),
		Object:     NewObject(conn),
		TreeEntry:  NewTreeEntry(conn),
		Reference:  NewReference(conn),
	}
}
