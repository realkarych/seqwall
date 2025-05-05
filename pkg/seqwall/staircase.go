package seqwall

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/realkarych/seqwall/pkg/driver"
)

func (s *StaircaseWorker) Run() error {
	client, err := driver.NewPostgresClient(s.postgresURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	s.dbClient = client
	defer s.dbClient.Close()
	migrations, err := loadMigrations(s.migrationsPath, s.migrationsExtension)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if len(migrations) == 0 {
		return fmt.Errorf("%w: %s", ErrNoMigrations(), s.migrationsPath)
	}
	log.Printf("Recognized %d migrations", len(migrations))
	log.Println("Processing staircase…")
	if err := s.processStaircase(migrations); err != nil {
		return fmt.Errorf("staircase failed: %w", err)
	}
	log.Println("\n🎉 Staircase test completed successfully!")
	return nil
}

func (s *StaircaseWorker) processStaircase(migrations []string) error {
	log.Println("Step 1: DB actualisation – migrating all migrations up…")
	if err := s.actualiseDb(migrations); err != nil {
		return fmt.Errorf("actualise db: %w", err)
	}
	if err := s.processDownUpDown(migrations); err != nil {
		return fmt.Errorf("down-up-down phase: %w", err)
	}
	if err := s.processUpDownUp(migrations); err != nil {
		return fmt.Errorf("up-down-up phase: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) actualiseDb(migrations []string) error {
	for i, migration := range migrations {
		log.Printf("Running migration %d/%d: %s", i+1, len(migrations), migration)
		out, err := s.executeCommand(s.upgradeCmd, migration)
		if err != nil {
			return fmt.Errorf("apply migration %q (step %d): %w", migration, i+1, err)
		}
		log.Println("Migration output:", out)
		snap, err := s.makeSchemaSnapshot()
		if err != nil {
			return fmt.Errorf("snapshot after %q: %w", migration, err)
		}
		s.baseline[migration] = snap
	}
	log.Println("Step 1 (actualise db) completed successfully!")
	return nil
}

func (s *StaircaseWorker) compareAndSnapshot(exp *driver.SchemaSnapshot, ctx string) error {
	if !s.compareSchemaSnapshots || exp == nil {
		return nil
	}
	snap, err := s.makeSchemaSnapshot()
	if err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	if err := compareSchemas(exp, snap); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	return nil
}

func (s *StaircaseWorker) runDownUpDown(mig string, step int, cur, prev *driver.SchemaSnapshot) error {
	if err := s.makeDownStep(mig, step); err != nil {
		return fmt.Errorf("down step %q: %w", mig, err)
	}
	if err := s.compareAndSnapshot(prev, fmt.Sprintf("snapshot after first down %q", mig)); err != nil {
		return err
	}
	if err := s.makeUpStep(mig, step); err != nil {
		return fmt.Errorf("up step %q: %w", mig, err)
	}
	if err := s.compareAndSnapshot(cur, fmt.Sprintf("snapshot after down-up %q", mig)); err != nil {
		return err
	}
	if err := s.makeDownStep(mig, step); err != nil {
		return fmt.Errorf("final down step %q: %w", mig, err)
	}
	if err := s.compareAndSnapshot(prev, fmt.Sprintf("snapshot after final down %q", mig)); err != nil {
		return err
	}
	log.Printf("Final Down test passed for %s", mig)
	return nil
}

func (s *StaircaseWorker) runUpDownUp(mig string, step int, cur, prev *driver.SchemaSnapshot) error {
	if err := s.makeUpStep(mig, step); err != nil {
		return fmt.Errorf("up step %q: %w", mig, err)
	}
	if err := s.makeDownStep(mig, step); err != nil {
		return fmt.Errorf("down step %q: %w", mig, err)
	}
	if err := s.compareAndSnapshot(prev, fmt.Sprintf("snapshot after down %q", mig)); err != nil {
		return err
	}
	if err := s.makeUpStep(mig, step); err != nil {
		return fmt.Errorf("final up step %q: %w", mig, err)
	}
	if err := s.compareAndSnapshot(cur, fmt.Sprintf("snapshot after final up %q", mig)); err != nil {
		return err
	}
	return nil
}

func (s *StaircaseWorker) processDownUpDown(migs []string) error {
	steps := s.calculateStairDepth(migs)
	for i := 1; i <= steps; i++ {
		mig := migs[len(migs)-i]
		cur, ok := s.baseline[mig]
		if !ok {
			return fmt.Errorf("%w: %s", ErrBaselineNotFound(), mig)
		}
		var prev *driver.SchemaSnapshot
		if idx := len(migs) - i - 1; idx >= 0 {
			prev = s.baseline[migs[idx]]
		}
		if err := s.runDownUpDown(mig, i, cur, prev); err != nil {
			return err
		}
	}
	log.Println("Step 2 (down-up-down) completed successfully!")
	return nil
}

func (s *StaircaseWorker) processUpDownUp(migs []string) error {
	log.Println("Step 3: Run staircase test (up-down-up)...")
	steps := s.calculateStairDepth(migs)
	tail := migs[len(migs)-steps:]
	log.Printf("Running staircase test with %d steps", steps)
	for i, mig := range tail {
		step := i + 1
		cur, ok := s.baseline[mig]
		if !ok {
			return fmt.Errorf("%w: %s", ErrBaselineNotFound(), mig)
		}
		var prev *driver.SchemaSnapshot
		if idx := len(migs) - steps + i - 1; idx >= 0 {
			prev = s.baseline[migs[idx]]
		}
		if err := s.runUpDownUp(mig, step, cur, prev); err != nil {
			return err
		}
	}
	log.Println("Step 3 (up-down-up) completed successfully!")
	return nil
}

func (s *StaircaseWorker) makeUpStep(migration string, step int) error {
	log.Printf("Applying migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.upgradeCmd, migration)
	if err != nil {
		return fmt.Errorf("apply migration %q (step %d): %w", migration, step, err)
	}
	log.Println("Migration applied:", output)
	return nil
}

func (s *StaircaseWorker) makeDownStep(migration string, step int) error {
	log.Printf("Reverting migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.downgradeCmd, migration)
	if err != nil {
		return fmt.Errorf("revert migration %q (step %d): %w", migration, step, err)
	}
	log.Println("Migration reverted:", output)
	return nil
}

func (s *StaircaseWorker) executeCommand(command, migration string) (string, error) {
	command = strings.ReplaceAll(command, CurrentMigrationPlaceholder, migration)
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command '%s' failed with error: %v\nCallback:\n%s\nStacktrace:\n%s",
			command, err, string(output), debug.Stack())
	}
	return string(output), err
}

func (s *StaircaseWorker) calculateStairDepth(migrations []string) int {
	steps := len(migrations)
	if s.depth > 0 && s.depth < steps {
		steps = s.depth
	}
	return steps
}

func (s *StaircaseWorker) makeSchemaSnapshot() (*driver.SchemaSnapshot, error) {
	snap := &driver.SchemaSnapshot{
		Tables:      make(map[string]driver.TableDefinition),
		Views:       make(map[string]driver.ViewDefinition),
		Indexes:     make(map[string]driver.IndexDefinition),
		Constraints: make(map[string]driver.ConstraintDefinition),
		EnumTypes:   make(map[string]driver.EnumDefinition),
		ForeignKeys: make(map[string]driver.ForeignKeyDefinition),
	}
	type scanFn struct {
		fn   func(*driver.SchemaSnapshot) error `json:"-"`
		name string
	}
	scanners := []scanFn{
		{s.scanTables, "tables"},
		{s.scanColumns, "columns"},
		{s.scanConstraints, "constraints"},
		{s.scanEnums, "enums"},
		{s.scanFks, "foreign keys"},
		{s.scanFunctions, "functions"},
		{s.scanIndexes, "indexes"},
		{s.scanSeqs, "sequences"},
		{s.scanTriggers, "triggers"},
		{s.scanViews, "views"},
	}
	for _, sc := range scanners {
		if err := sc.fn(snap); err != nil {
			return nil, fmt.Errorf("scan %s: %w", sc.name, err)
		}
	}
	return snap, nil
}

func (s *StaircaseWorker) scanTables(snapshot *driver.SchemaSnapshot) error {
	tablesQuery := fmt.Sprintf(
		`
            SELECT tablename
            FROM pg_catalog.pg_tables
            WHERE %s
            ORDER BY tablename;
        `,
		s.buildSchemaCond("schemaname"),
	)
	rows, err := s.dbClient.Execute(tablesQuery)
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Rows.Close()

	for rows.Rows.Next() {
		var tableName string
		if err := rows.Rows.Scan(&tableName); err != nil {
			return fmt.Errorf("scan table row: %w", err)
		}
		if _, ok := snapshot.Tables[tableName]; !ok {
			snapshot.Tables[tableName] = driver.TableDefinition{}
		}
	}
	return rows.Rows.Err()
}

func (s *StaircaseWorker) scanColumns(snapshot *driver.SchemaSnapshot) error {
	query := s.buildColumnsQuery()
	rows, err := s.dbClient.Execute(query)
	if err != nil {
		return fmt.Errorf("query columns: %w", err)
	}
	defer rows.Rows.Close()
	for rows.Rows.Next() {
		colDef, tableName, err := scanColumnRow(rows)
		if err != nil {
			return err
		}
		td := snapshot.Tables[tableName]
		td.Columns = append(td.Columns, colDef)
		snapshot.Tables[tableName] = td
	}

	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate column rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) buildColumnsQuery() string {
	return fmt.Sprintf(`
        SELECT
            c.table_name,
            c.column_name,
            c.data_type,
            c.udt_name,
            t.typtype,
            t.typcategory,
            CASE WHEN t.typtype = 'e' THEN 0 ELSE a.atttypid END AS type_oid,
            c.datetime_precision,
            c.is_nullable,
            c.collation_name,
            c.is_identity,
            c.identity_generation,
            c.is_generated,
            c.generation_expression,
            c.column_default,
            c.character_maximum_length,
            c.numeric_precision,
            c.numeric_scale
        FROM information_schema.columns c
        JOIN pg_catalog.pg_type t
            ON c.udt_name = t.typname
        JOIN pg_catalog.pg_attribute a
            ON a.attrelid = (c.table_schema||'.'||c.table_name)::regclass
            AND a.attname = c.column_name
        WHERE %s
        ORDER BY c.table_name, c.ordinal_position;
    `, s.buildSchemaCond("c.table_schema"))
}

func scanColumnRow(rows *driver.QueryResult) (driver.ColumnDefinition, string, error) {
	var (
		table, name, dtype, udt       string
		typtype, typcategory          string
		typeOID                       int
		dtp                           sql.NullInt64
		nullable, identity, generated string
		genExpr, def, coll, idGen     sql.NullString
		charLen, numPrec, numScale    sql.NullInt64
	)
	if err := rows.Rows.Scan(
		&table,
		&name,
		&dtype,
		&udt,
		&typtype,
		&typcategory,
		&typeOID,
		&dtp,
		&nullable,
		&coll,
		&identity,
		&idGen,
		&generated,
		&genExpr,
		&def,
		&charLen,
		&numPrec,
		&numScale,
	); err != nil {
		return driver.ColumnDefinition{}, "", fmt.Errorf("scan column row: %w", err)
	}
	col := driver.ColumnDefinition{
		ColumnName:             name,
		DataType:               dtype,
		UDTName:                udt,
		DateTimePrecision:      dtp,
		IsNullable:             nullable,
		ColumnDefault:          def,
		CharacterMaximumLength: charLen,
		NumericPrecision:       numPrec,
		NumericScale:           numScale,
		IsIdentity:             identity,
		IdentityGeneration:     idGen,
		IsGenerated:            generated,
		GenerationExpression:   genExpr,
		CollationName:          coll,
		TypeMeta: driver.TypeMeta{
			Typtype:     typtype,
			Typcategory: typcategory,
			TypeOID:     typeOID,
		},
	}
	return col, table, nil
}

func (s *StaircaseWorker) scanViews(snapshot *driver.SchemaSnapshot) error {
	viewsQuery := fmt.Sprintf(
		`
            SELECT
                viewname AS table_name,
                pg_get_viewdef(viewname::regclass, true) AS definition
            FROM pg_views
            WHERE %s;
        `,
		s.buildSchemaCond("schemaname"),
	)
	viewRows, err := s.dbClient.Execute(viewsQuery)
	if err != nil {
		return fmt.Errorf("query views: %w", err)
	}
	defer viewRows.Rows.Close()
	for viewRows.Rows.Next() {
		var viewName, viewDefinition string
		if err := viewRows.Rows.Scan(
			&viewName,
			&viewDefinition,
		); err != nil {
			return fmt.Errorf("scan view row: %w", err)
		}
		snapshot.Views[viewName] = driver.ViewDefinition{Definition: viewDefinition}
	}
	if err := viewRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate view rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanIndexes(snapshot *driver.SchemaSnapshot) error {
	indexesQuery := fmt.Sprintf(
		`
            SELECT indexname, indexdef
            FROM pg_indexes
            WHERE %s
            ORDER BY indexname;
        `,
		s.buildSchemaCond("schemaname"),
	)
	indexRows, err := s.dbClient.Execute(indexesQuery)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer indexRows.Rows.Close()
	for indexRows.Rows.Next() {
		var indexName, indexDef string
		if err := indexRows.Rows.Scan(
			&indexName,
			&indexDef,
		); err != nil {
			return fmt.Errorf("scan index row: %w", err)
		}
		snapshot.Indexes[indexName] = driver.IndexDefinition{IndexDef: indexDef}
	}
	if err := indexRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate index rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanConstraints(snapshot *driver.SchemaSnapshot) error {
	constraintsQuery := fmt.Sprintf(
		`
            SELECT
                tc.constraint_name,
                tc.table_name,
                tc.constraint_type,
                cc.check_clause
            FROM information_schema.table_constraints tc
            LEFT JOIN information_schema.check_constraints cc
                   ON tc.constraint_name = cc.constraint_name
            WHERE %s;
        `,
		s.buildSchemaCond("tc.table_schema"),
	)
	constrRows, err := s.dbClient.Execute(constraintsQuery)
	if err != nil {
		return fmt.Errorf("query constraints: %w", err)
	}
	defer constrRows.Rows.Close()
	for constrRows.Rows.Next() {
		var (
			constraintName, tableName, constraintType string
			checkClause                               sql.NullString
		)
		if err := constrRows.Rows.Scan(
			&constraintName,
			&tableName,
			&constraintType,
			&checkClause,
		); err != nil {
			return fmt.Errorf("scan constraint row: %w", err)
		}
		snapshot.Constraints[constraintName] = driver.ConstraintDefinition{
			TableName:      tableName,
			ConstraintType: constraintType,
			Definition:     checkClause,
		}
	}
	if err := constrRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate constraint rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanEnums(snapshot *driver.SchemaSnapshot) error {
	enumQuery := fmt.Sprintf(
		`
            SELECT
                t.typname,
                e.enumlabel
            FROM pg_type t
            JOIN pg_enum e ON t.oid = e.enumtypid
            JOIN pg_namespace n ON n.oid = t.typnamespace
            WHERE %s
            ORDER BY t.typname, e.enumsortorder;
        `,
		s.buildSchemaCond("n.nspname"),
	)
	enumRows, err := s.dbClient.Execute(enumQuery)
	if err != nil {
		return fmt.Errorf("query enum types: %w", err)
	}
	defer enumRows.Rows.Close()
	for enumRows.Rows.Next() {
		var typeName, enumLabel string
		if err := enumRows.Rows.Scan(
			&typeName,
			&enumLabel,
		); err != nil {
			return fmt.Errorf("scan enum row: %w", err)
		}
		def := snapshot.EnumTypes[typeName]
		def.Labels = append(def.Labels, enumLabel)
		snapshot.EnumTypes[typeName] = def
	}
	if err := enumRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate enum rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanFks(snapshot *driver.SchemaSnapshot) error {
	foreignKeysQuery := fmt.Sprintf(
		`
            SELECT
                tc.constraint_name,
                tc.table_name,
                kcu.column_name,
                ccu.table_name  AS foreign_table_name,
                ccu.column_name AS foreign_column_name,
                rc.update_rule,
                rc.delete_rule
            FROM information_schema.table_constraints        AS tc
            JOIN information_schema.key_column_usage         AS kcu ON tc.constraint_name = kcu.constraint_name
            JOIN information_schema.referential_constraints  AS rc  ON tc.constraint_name = rc.constraint_name
            JOIN information_schema.constraint_column_usage  AS ccu ON ccu.constraint_name = tc.constraint_name
            WHERE tc.constraint_type = 'FOREIGN KEY'
              AND %s;
        `,
		s.buildSchemaCond("tc.table_schema"),
	)
	rows, err := s.dbClient.Execute(foreignKeysQuery)
	if err != nil {
		return fmt.Errorf("query foreign keys: %w", err)
	}
	defer rows.Rows.Close()
	for rows.Rows.Next() {
		var (
			constraintName, tableName, columnName string
			foreignTableName, foreignColumnName   string
			updateRule, deleteRule                string
		)
		if err := rows.Rows.Scan(
			&constraintName,
			&tableName,
			&columnName,
			&foreignTableName,
			&foreignColumnName,
			&updateRule,
			&deleteRule,
		); err != nil {
			return fmt.Errorf("scan foreign key row: %w", err)
		}
		snapshot.ForeignKeys[constraintName] = driver.ForeignKeyDefinition{
			ConstraintName:    constraintName,
			TableName:         tableName,
			ColumnName:        columnName,
			ForeignTableName:  foreignTableName,
			ForeignColumnName: foreignColumnName,
			UpdateRule:        updateRule,
			DeleteRule:        deleteRule,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign key rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanTriggers(snapshot *driver.SchemaSnapshot) error {
	triggersQuery := fmt.Sprintf(
		`
            SELECT
                trigger_name,
                event_manipulation,
                event_object_table,
                action_timing,
                action_statement
            FROM information_schema.triggers
            WHERE %s
            ORDER BY trigger_name;
        `,
		s.buildSchemaCond("trigger_schema"),
	)
	rows, err := s.dbClient.Execute(triggersQuery)
	if err != nil {
		return fmt.Errorf("query triggers: %w", err)
	}
	defer rows.Rows.Close()
	if snapshot.Triggers == nil {
		snapshot.Triggers = make(map[string]driver.TriggerDefinition)
	}
	for rows.Rows.Next() {
		var (
			triggerName, eventManipulation, eventObjectTable string
			actionTiming, actionStatement                    string
		)
		if err := rows.Rows.Scan(
			&triggerName,
			&eventManipulation,
			&eventObjectTable,
			&actionTiming,
			&actionStatement,
		); err != nil {
			return fmt.Errorf("scan trigger row: %w", err)
		}
		snapshot.Triggers[triggerName] = driver.TriggerDefinition{
			TriggerName:       triggerName,
			EventManipulation: eventManipulation,
			EventObjectTable:  eventObjectTable,
			ActionTiming:      actionTiming,
			ActionStatement:   actionStatement,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate trigger rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanFunctions(snapshot *driver.SchemaSnapshot) error {
	routinesQuery := fmt.Sprintf(
		`
            SELECT routine_name, routine_type, data_type
            FROM information_schema.routines
            WHERE %s
            ORDER BY routine_name;
        `,
		s.buildSchemaCond("specific_schema"),
	)
	rows, err := s.dbClient.Execute(routinesQuery)
	if err != nil {
		return fmt.Errorf("query functions: %w", err)
	}
	defer rows.Rows.Close()
	if snapshot.Functions == nil {
		snapshot.Functions = make(map[string]driver.FunctionDefinition)
	}
	for rows.Rows.Next() {
		var (
			routineName     string
			routineTypeNull sql.NullString
			returnType      string
		)
		if err := rows.Rows.Scan(
			&routineName,
			&routineTypeNull,
			&returnType,
		); err != nil {
			return fmt.Errorf("scan function row: %w", err)
		}
		routineType := ""
		if routineTypeNull.Valid {
			routineType = routineTypeNull.String
		}
		snapshot.Functions[routineName] = driver.FunctionDefinition{
			RoutineName: routineName,
			RoutineType: routineType,
			ReturnType:  returnType,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate function rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanSeqs(snapshot *driver.SchemaSnapshot) error {
	seqQuery := fmt.Sprintf(
		`
            SELECT sequence_name, data_type, start_value, minimum_value, maximum_value, increment, cycle_option
            FROM information_schema.sequences
            WHERE %s
            ORDER BY sequence_name;
        `,
		s.buildSchemaCond("sequence_schema"),
	)
	rows, err := s.dbClient.Execute(seqQuery)
	if err != nil {
		return fmt.Errorf("query sequences: %w", err)
	}
	defer rows.Rows.Close()
	if snapshot.Sequences == nil {
		snapshot.Sequences = make(map[string]driver.SequenceDefinition)
	}
	for rows.Rows.Next() {
		var (
			sequenceName, dataType, startValue string
			minValue, maxValue, increment      string
			cycleOption                        string
		)
		if err := rows.Rows.Scan(
			&sequenceName,
			&dataType,
			&startValue,
			&minValue,
			&maxValue,
			&increment,
			&cycleOption,
		); err != nil {
			return fmt.Errorf("scan sequence row: %w", err)
		}
		snapshot.Sequences[sequenceName] = driver.SequenceDefinition{
			SequenceName: sequenceName,
			DataType:     dataType,
			StartValue:   startValue,
			MinValue:     minValue,
			MaxValue:     maxValue,
			Increment:    increment,
			CycleOption:  cycleOption,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate sequence rows: %w", err)
	}
	return nil
}

// buildSchemaCond("table_schema") -> "table_schema = 'public'".
// buildSchemaCond("tc.table_schema") -> "tc.table_schema IN ('public','extra')".
func (s *StaircaseWorker) buildSchemaCond(col string) string {
	list := s.schemas
	if len(list) == 0 {
		list = []string{"public"}
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	if len(list) == 1 {
		return fmt.Sprintf("%s = %s", col, quote(list[0]))
	}
	quoted := make([]string, len(list))
	for i, v := range list {
		quoted[i] = quote(v)
	}
	return fmt.Sprintf("%s IN (%s)", col, strings.Join(quoted, ", "))
}
