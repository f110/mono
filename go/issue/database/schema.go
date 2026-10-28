package database

import _ "embed"

// Schema is the DDL of all tables.
//
//go:embed schema.sql
var Schema string
