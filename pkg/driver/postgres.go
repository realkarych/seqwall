package driver

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
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
	trimmed := strings.TrimSpace(query)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
		result, err := p.conn.Exec(query, args...)
		if err != nil {
			return nil, err
		}
		return &QueryResult{Result: result}, nil
	}
	rows, err := p.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		if cerr := rows.Close(); cerr != nil {
			return nil, fmt.Errorf("query error: %w; close error: %w", err, cerr)
		}
		return nil, err
	}
	return &QueryResult{Rows: rows}, nil
}

func (p *PostgresClient) Close() error {
	return p.conn.Close()
}
