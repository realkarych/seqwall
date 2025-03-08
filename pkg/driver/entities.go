package driver

import "database/sql"

type QueryResult struct {
	Rows   *sql.Rows
	Result sql.Result
}

type DbClient interface {
	Execute(query string, args ...interface{}) (*QueryResult, error)
	Close() error
}
