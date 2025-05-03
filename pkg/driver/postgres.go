package driver

import (
	"database/sql"
	"strings"

	_ "github.com/lib/pq"
)

type QueryResult struct {
	Result sql.Result
	Rows   *sql.Rows
}

type DbClient interface {
	Execute(query string, args ...interface{}) (*QueryResult, error)
	Close() error
}

type ColumnDefinition struct {
	ColumnName             string
	DataType               string
	UDTName                string
	IsNullable             string
	IsIdentity             string
	IsGenerated            string
	CollationName          sql.NullString
	IdentityGeneration     sql.NullString
	GenerationExpression   sql.NullString
	ColumnDefault          sql.NullString
	DateTimePrecision      sql.NullInt64
	CharacterMaximumLength sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
}

type TableDefinition struct {
	Columns []ColumnDefinition
}

type ViewDefinition struct {
	Definition string
}

type IndexDefinition struct {
	IndexDef string
}

type ConstraintDefinition struct {
	TableName      string
	ConstraintType string
	Definition     sql.NullString
}

type EnumDefinition struct {
	Labels []string
}

type ForeignKeyDefinition struct {
	ConstraintName    string
	TableName         string
	ColumnName        string
	ForeignTableName  string
	ForeignColumnName string
	UpdateRule        string
	DeleteRule        string
}

type TriggerDefinition struct {
	TriggerName       string
	EventManipulation string
	EventObjectTable  string
	ActionTiming      string
	ActionStatement   string
}

type FunctionDefinition struct {
	RoutineName       string
	RoutineType       string
	ReturnType        string
	RoutineDefinition string
}

type SchemaSnapshot struct {
	Tables      map[string]TableDefinition
	Views       map[string]ViewDefinition
	Indexes     map[string]IndexDefinition
	Constraints map[string]ConstraintDefinition
	EnumTypes   map[string]EnumDefinition
	ForeignKeys map[string]ForeignKeyDefinition
	Triggers    map[string]TriggerDefinition
	Functions   map[string]FunctionDefinition
	Sequences   map[string]SequenceDefinition
}

type SequenceDefinition struct {
	SequenceName string
	DataType     string
	StartValue   string
	MinValue     string
	MaxValue     string
	Increment    string
	CycleOption  string
}

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
		if err := rows.Err(); err != nil {
			rows.Close()
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
