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

type ColumnDefinition struct {
	ColumnName             string
	DataType               string
	IsNullable             string
	ColumnDefault          sql.NullString
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
