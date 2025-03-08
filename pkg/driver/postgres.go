package driver

import (
	"database/sql"
	"strings"
)

type PostgresClient struct {
	conn *sql.DB
}

func NewPostgresClient(postgresPath string) (*PostgresClient, error) {
	db, err := sql.Open("postgres", postgresPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresClient{conn: db}, nil
}

func (p *PostgresClient) Execute(query string, args ...interface{}) (*QueryResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if strings.HasPrefix(strings.ToUpper(trimmedQuery), "SELECT") {
		rows, err := p.conn.Query(query, args...)
		if err != nil {
			return nil, err
		}
		return &QueryResult{Rows: rows}, nil
	} else {
		result, err := p.conn.Exec(query, args...)
		if err != nil {
			return nil, err
		}
		return &QueryResult{Result: result}, nil
	}
}

func (p *PostgresClient) Close() error {
	return p.conn.Close()
}
