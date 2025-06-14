package driver

import "database/sql"

type QueryResult struct {
	Result sql.Result `json:"result"`
	Rows   *sql.Rows  `json:"rows"`
}

type DbClient interface {
	Execute(query string, args ...interface{}) (*QueryResult, error)
	Close() error
}

type TypeMeta struct {
	Typtype     string `db:"typtype"     json:"typtype"`
	Typcategory string `db:"typcategory" json:"typcategory"`
	TypeOID     int    `db:"type_oid"    json:"type_oid"`
}

type ColumnDefinition struct {
	ColumnName             string         `db:"column_name"              json:"column_name"`
	DataType               string         `db:"data_type"                json:"data_type"`
	UDTName                string         `db:"udt_name"                 json:"udt_name"`
	TypeMeta               TypeMeta       `db:"type_meta"                json:"type_meta"`
	IsNullable             string         `db:"is_nullable"              json:"is_nullable"`
	IsIdentity             string         `db:"is_identity"              json:"is_identity"`
	IsGenerated            string         `db:"is_generated"             json:"is_generated"`
	CollationName          sql.NullString `db:"collation_name"           json:"collation_name"`
	IdentityGeneration     sql.NullString `db:"identity_generation"      json:"identity_generation"`
	GenerationExpression   sql.NullString `db:"generation_expression"    json:"generation_expression"`
	ColumnDefault          sql.NullString `db:"column_default"           json:"column_default"`
	DateTimePrecision      sql.NullInt64  `db:"datetime_precision"       json:"datetime_precision"`
	CharacterMaximumLength sql.NullInt64  `db:"character_maximum_length" json:"character_maximum_length"`
	NumericPrecision       sql.NullInt64  `db:"numeric_precision"        json:"numeric_precision"`
	NumericScale           sql.NullInt64  `db:"numeric_scale"            json:"numeric_scale"`
}

type TableDefinition struct {
	Columns []ColumnDefinition `db:"columns" json:"columns"`
}

type ViewDefinition struct {
	Definition string `db:"definition" json:"definition"`
}

type IndexDefinition struct {
	IndexDef string `db:"index_def" json:"index_def"`
}

type ConstraintDefinition struct {
	TableName      string         `db:"table_name"      json:"table_name"`
	ConstraintType string         `db:"constraint_type" json:"constraint_type"`
	Definition     sql.NullString `db:"definition"      json:"definition"`
}

type EnumDefinition struct {
	Labels []string `db:"labels" json:"labels"`
}

type ForeignKeyDefinition struct {
	ConstraintName    string `db:"constraint_name"     json:"constraint_name"`
	TableName         string `db:"table_name"          json:"table_name"`
	ColumnName        string `db:"column_name"         json:"column_name"`
	ForeignTableName  string `db:"foreign_table_name"  json:"foreign_table_name"`
	ForeignColumnName string `db:"foreign_column_name" json:"foreign_column_name"`
	UpdateRule        string `db:"update_rule"         json:"update_rule"`
	DeleteRule        string `db:"delete_rule"         json:"delete_rule"`
}

type TriggerDefinition struct {
	TriggerName       string `db:"trigger_name"       json:"trigger_name"`
	EventManipulation string `db:"event_manipulation" json:"event_manipulation"`
	EventObjectTable  string `db:"event_object_table" json:"event_object_table"`
	ActionTiming      string `db:"action_timing"      json:"action_timing"`
	ActionStatement   string `db:"action_statement"   json:"action_statement"`
}

type FunctionDefinition struct {
	RoutineName string `db:"routine_name" json:"routine_name"`
	RoutineType string `db:"routine_type" json:"routine_type"`
	ReturnType  string `db:"return_type"  json:"return_type"`
}

type SequenceDefinition struct {
	SequenceName string `db:"sequence_name" json:"sequence_name"`
	DataType     string `db:"data_type"     json:"data_type"`
	StartValue   string `db:"start_value"   json:"start_value"`
	MinValue     string `db:"min_value"     json:"min_value"`
	MaxValue     string `db:"max_value"     json:"max_value"`
	Increment    string `db:"increment"     json:"increment"`
	CycleOption  string `db:"cycle_option"  json:"cycle_option"`
}

type MatViewDefinition struct {
	Definition  string `db:"definition"   json:"definition"`
	IsPopulated bool   `db:"is_populated" json:"is_populated"`
}

type SchemaSnapshot struct {
	Tables      map[string]TableDefinition      `db:"tables"       json:"tables"`
	Views       map[string]ViewDefinition       `db:"views"        json:"views"`
	MatViews    map[string]MatViewDefinition    `db:"matviews"     json:"matviews"`
	Indexes     map[string]IndexDefinition      `db:"indexes"      json:"indexes"`
	Constraints map[string]ConstraintDefinition `db:"constraints"  json:"constraints"`
	EnumTypes   map[string]EnumDefinition       `db:"enum_types"   json:"enum_types"`
	ForeignKeys map[string]ForeignKeyDefinition `db:"foreign_keys" json:"foreign_keys"`
	Triggers    map[string]TriggerDefinition    `db:"triggers"     json:"triggers"`
	Functions   map[string]FunctionDefinition   `db:"functions"    json:"functions"`
	Sequences   map[string]SequenceDefinition   `db:"sequences"    json:"sequences"`
}
