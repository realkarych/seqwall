package seqwall

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/realkarych/seqwall/pkg/driver"
)

type StaircaseCli struct {
	migrationsPath string
	testSchema     bool
	depth          int
	migrateUp      string
	migrateDown    string
	postgresUrl    string
	dbClient       *driver.PostgresClient
}

func NewStaircaseCli(
	migrationsPath string,
	testSchema bool,
	depth int,
	migrateUp string,
	migrateDown string,
	postgresUrl string,
) *StaircaseCli {
	return &StaircaseCli{
		migrationsPath: migrationsPath,
		testSchema:     testSchema,
		depth:          depth,
		migrateUp:      migrateUp,
		migrateDown:    migrateDown,
		postgresUrl:    postgresUrl,
	}
}

func (s *StaircaseCli) Run() error {
	client, err := driver.NewPostgresClient(s.postgresUrl)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	s.dbClient = client
	defer s.dbClient.Close()
	migrations, err := loadMigrations(s.migrationsPath)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if len(migrations) == 0 {
		return fmt.Errorf("no migrations found in %s", s.migrationsPath)
	}
	log.Printf("Recognized %d migrations", len(migrations))
	log.Println("Processing staircase…")
	if err := s.processStaircase(migrations); err != nil {
		return fmt.Errorf("staircase failed: %w", err)
	}
	return nil
}

func (s *StaircaseCli) processStaircase(migrations []string) error {
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

func (s *StaircaseCli) actualiseDb(migrations []string) error {
	for i, migration := range migrations {
		log.Printf("Running migration %d/%d: %s", i+1, len(migrations), migration)
		output, err := s.executeCommand(s.migrateUp, migration)
		if err != nil {
			return fmt.Errorf("apply migration %q (step %d): %w", migration, i+1, err)
		}
		log.Println("Migration output:", output)
	}
	return nil
}

func (s *StaircaseCli) processDownUpDown(migrations []string) error {
	steps := s.calculateStairDepth(migrations)
	for i := 1; i <= steps; i++ {
		migration := migrations[len(migrations)-i]
		var snapBeforeDown, snapAfterFirstDown *driver.SchemaSnapshot
		if s.testSchema {
			var err error
			snapBeforeDown, err = s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot before first down %q: %w", migration, err)
			}
		}
		if err := s.makeDownStep(migration, i); err != nil {
			return fmt.Errorf("down step %q: %w", migration, err)
		}
		if s.testSchema {
			var err error
			snapAfterFirstDown, err = s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot after first down %q: %w", migration, err)
			}
		}
		if err := s.makeUpStep(migration, i); err != nil {
			return fmt.Errorf("up step %q: %w", migration, err)
		}
		if s.testSchema {
			snapAfterDownUp, err := s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot after down-up %q: %w", migration, err)
			}
			if err := s.compareSchemas(snapBeforeDown, snapAfterDownUp); err != nil {
				return fmt.Errorf("compare down-up %q: %w", migration, err)
			}
			log.Printf("Down→Up test passed for %s", migration)
		}
		if err := s.makeDownStep(migration, i); err != nil {
			return fmt.Errorf("final down step %q: %w", migration, err)
		}
		if s.testSchema {
			snapAfterFinalDown, err := s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot after final down %q: %w", migration, err)
			}
			if err := s.compareSchemas(snapAfterFirstDown, snapAfterFinalDown); err != nil {
				return fmt.Errorf("compare final down %q: %w", migration, err)
			}
			log.Printf("Final Down test passed for %s", migration)
		}
	}
	return nil
}

func (s *StaircaseCli) processUpDownUp(migrations []string) error {
	log.Println("Step 3: Run staircase test (up-down-up)...")
	steps := s.calculateStairDepth(migrations)
	tail := migrations[len(migrations)-steps:]
	log.Printf("Running staircase test with %d steps", steps)
	for i, migration := range tail {
		step := i + 1
		var before *driver.SchemaSnapshot
		if s.testSchema {
			var err error
			before, err = s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot before up %q: %w", migration, err)
			}
		}
		if err := s.makeUpStep(migration, step); err != nil {
			return fmt.Errorf("up step %q: %w", migration, err)
		}
		if err := s.makeDownStep(migration, step); err != nil {
			return fmt.Errorf("down step %q: %w", migration, err)
		}
		if s.testSchema {
			after, err := s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot after down %q: %w", migration, err)
			}
			if err := s.compareSchemas(before, after); err != nil {
				return fmt.Errorf("compare up-down %q: %w", migration, err)
			}
			log.Printf("schema snapshots are equal for migration %s at step %d", migration, step)
		}
		if err := s.makeUpStep(migration, step); err != nil {
			return fmt.Errorf("final up step %q: %w", migration, err)
		}
	}
	log.Println("Staircase test (up-down-up) completed successfully!")
	return nil
}

func (s *StaircaseCli) makeUpStep(migration string, step int) error {
	log.Printf("Applying migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.migrateUp, migration)
	if err != nil {
		return fmt.Errorf("apply migration %q (step %d): %w", migration, step, err)
	}
	log.Println("Migration applied:", output)
	return nil
}

func (s *StaircaseCli) makeDownStep(migration string, step int) error {
	log.Printf("Reverting migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.migrateDown, migration)
	if err != nil {
		return fmt.Errorf("revert migration %q (step %d): %w", migration, step, err)
	}
	log.Println("Migration reverted:", output)
	return nil
}

func (s *StaircaseCli) executeCommand(command, migration string) (string, error) {
	command = strings.Replace(command, CurrentMigrationPlaceholder, migration, -1)
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command '%s' failed with error: %v\nCallback:\n%s\nStacktrace:\n%s",
			command, err, string(output), debug.Stack())
	}
	return string(output), err
}

func (s *StaircaseCli) calculateStairDepth(migrations []string) int {
	steps := len(migrations)
	if s.depth > 0 && s.depth < steps {
		steps = s.depth
	}
	return steps
}

func (s *StaircaseCli) makeSchemaSnapshot() (*driver.SchemaSnapshot, error) {
	snapshot := &driver.SchemaSnapshot{
		Tables:      make(map[string]driver.TableDefinition),
		Views:       make(map[string]driver.ViewDefinition),
		Indexes:     make(map[string]driver.IndexDefinition),
		Constraints: make(map[string]driver.ConstraintDefinition),
		EnumTypes:   make(map[string]driver.EnumDefinition),
		ForeignKeys: make(map[string]driver.ForeignKeyDefinition),
	}
	s.scanColumns(snapshot)
	s.scanConstraints(snapshot)
	s.scanEnums(snapshot)
	s.scanFks(snapshot)
	s.scanFunctions(snapshot)
	s.scanIndexes(snapshot)
	s.scanSeqs(snapshot)
	s.scanTriggers(snapshot)
	s.scanViews(snapshot)
	return snapshot, nil
}

func (s *StaircaseCli) scanColumns(snapshot *driver.SchemaSnapshot) error {
	const columnsQuery = `
        SELECT
            table_name,
            column_name,
            data_type,
            udt_name,
            datetime_precision,
            is_nullable,
            collation_name,
            is_identity,
            identity_generation,
            is_generated,
            generation_expression,
            column_default,
            character_maximum_length,
            numeric_precision,
            numeric_scale
        FROM information_schema.columns
        WHERE table_schema = 'public'
        ORDER BY table_name, ordinal_position;
    `
	colRows, err := s.dbClient.Execute(columnsQuery)
	if err != nil {
		return fmt.Errorf("query columns: %w", err)
	}
	defer colRows.Rows.Close()

	for colRows.Rows.Next() {
		var (
			tableName, columnName, dataType, udtName         string
			isNullable, isIdentity, isGenerated              string
			dateTimePrec                                     sql.NullInt64
			columnDefault, collationName, identityGeneration sql.NullString
			generationExpression                             sql.NullString
			charMaxLen, numPrecision, numScale               sql.NullInt64
		)
		if err := colRows.Rows.Scan(
			&tableName,
			&columnName,
			&dataType,
			&udtName,
			&dateTimePrec,
			&isNullable,
			&collationName,
			&isIdentity,
			&identityGeneration,
			&isGenerated,
			&generationExpression,
			&columnDefault,
			&charMaxLen,
			&numPrecision,
			&numScale,
		); err != nil {
			return fmt.Errorf("scan column row: %w", err)
		}

		colDef := driver.ColumnDefinition{
			ColumnName:             columnName,
			DataType:               dataType,
			UDTName:                udtName,
			DateTimePrecision:      dateTimePrec,
			IsNullable:             isNullable,
			ColumnDefault:          columnDefault,
			CharacterMaximumLength: charMaxLen,
			NumericPrecision:       numPrecision,
			NumericScale:           numScale,
			IsIdentity:             isIdentity,
			IdentityGeneration:     identityGeneration,
			IsGenerated:            isGenerated,
			GenerationExpression:   generationExpression,
			CollationName:          collationName,
		}

		td := snapshot.Tables[tableName]
		td.Columns = append(td.Columns, colDef)
		snapshot.Tables[tableName] = td
	}
	if err := colRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate column rows: %w", err)
	}

	return nil
}

func (s *StaircaseCli) scanViews(snapshot *driver.SchemaSnapshot) error {
	const viewsQuery = `
        SELECT
            viewname AS table_name,
            pg_get_viewdef(viewname::regclass, true) AS definition
        FROM pg_views
        WHERE schemaname = 'public';
    `
	viewRows, err := s.dbClient.Execute(viewsQuery)
	if err != nil {
		return fmt.Errorf("query views: %w", err)
	}
	defer viewRows.Rows.Close()
	for viewRows.Rows.Next() {
		var viewName, viewDefinition string
		if err := viewRows.Rows.Scan(&viewName, &viewDefinition); err != nil {
			return fmt.Errorf("scan view row: %w", err)
		}
		snapshot.Views[viewName] = driver.ViewDefinition{Definition: viewDefinition}
	}
	if err := viewRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate view rows: %w", err)
	}
	return nil
}

func (s *StaircaseCli) scanIndexes(snapshot *driver.SchemaSnapshot) error {
	const indexesQuery = `
        SELECT indexname, indexdef
        FROM pg_indexes
        WHERE schemaname = 'public';
    `
	indexRows, err := s.dbClient.Execute(indexesQuery)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer indexRows.Rows.Close()
	for indexRows.Rows.Next() {
		var indexName, indexDef string
		if err := indexRows.Rows.Scan(&indexName, &indexDef); err != nil {
			return fmt.Errorf("scan index row: %w", err)
		}
		snapshot.Indexes[indexName] = driver.IndexDefinition{IndexDef: indexDef}
	}
	if err := indexRows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate index rows: %w", err)
	}
	return nil
}

func (s *StaircaseCli) scanConstraints(snapshot *driver.SchemaSnapshot) error {
	const constraintsQuery = `
        SELECT
            tc.constraint_name,
            tc.table_name,
            tc.constraint_type,
            cc.check_clause
        FROM information_schema.table_constraints tc
        LEFT JOIN information_schema.check_constraints cc
               ON tc.constraint_name = cc.constraint_name
        WHERE tc.table_schema = 'public';
    `
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

func (s *StaircaseCli) scanEnums(snapshot *driver.SchemaSnapshot) error {
	const enumQuery = `
        SELECT
            t.typname,
            e.enumlabel
        FROM pg_type t
        JOIN pg_enum e ON t.oid = e.enumtypid
        ORDER BY t.typname, e.enumsortorder;
    `
	enumRows, err := s.dbClient.Execute(enumQuery)
	if err != nil {
		return fmt.Errorf("query enum types: %w", err)
	}
	defer enumRows.Rows.Close()
	for enumRows.Rows.Next() {
		var typeName, enumLabel string
		if err := enumRows.Rows.Scan(&typeName, &enumLabel); err != nil {
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

func (s *StaircaseCli) scanFks(snapshot *driver.SchemaSnapshot) error {
	const foreignKeysQuery = `
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
          AND tc.table_schema  = 'public';
    `
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

func (s *StaircaseCli) scanTriggers(snapshot *driver.SchemaSnapshot) error {
	const triggersQuery = `
		SELECT
			trigger_name,
			event_manipulation,
			event_object_table,
			action_timing,
			action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = 'public'
		ORDER BY trigger_name;
	`
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

func (s *StaircaseCli) scanFunctions(snapshot *driver.SchemaSnapshot) error {
	const q = `
		SELECT routine_name, routine_type, data_type, routine_definition
		FROM information_schema.routines
		WHERE specific_schema = 'public'
		ORDER BY routine_name;
	`
	rows, err := s.dbClient.Execute(q)
	if err != nil {
		return fmt.Errorf("query functions: %w", err)
	}
	defer rows.Rows.Close()
	if snapshot.Functions == nil {
		snapshot.Functions = make(map[string]driver.FunctionDefinition)
	}
	for rows.Rows.Next() {
		var routineName, routineType, returnType, routineDefinition string
		if err := rows.Rows.Scan(&routineName, &routineType, &returnType, &routineDefinition); err != nil {
			return fmt.Errorf("scan function row: %w", err)
		}
		snapshot.Functions[routineName] = driver.FunctionDefinition{
			RoutineName:       routineName,
			RoutineType:       routineType,
			ReturnType:        returnType,
			RoutineDefinition: routineDefinition,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate function rows: %w", err)
	}
	return nil
}

func (s *StaircaseCli) scanSeqs(snapshot *driver.SchemaSnapshot) error {
	const q = `
		SELECT sequence_name, data_type, start_value, minimum_value, maximum_value, increment, cycle_option
		FROM information_schema.sequences
		WHERE sequence_schema = 'public'
		ORDER BY sequence_name;
	`
	rows, err := s.dbClient.Execute(q)
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
		if err := rows.Rows.Scan(&sequenceName, &dataType, &startValue, &minValue, &maxValue, &increment, &cycleOption); err != nil {
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

func (s *StaircaseCli) compareSchemas(before, after *driver.SchemaSnapshot) error {
	normalize := func(src map[string]driver.ConstraintDefinition) map[string]driver.ConstraintDefinition {
		res := make(map[string]driver.ConstraintDefinition)
		re := regexp.MustCompile(`^([A-Za-z0-9_]+)\s+IS\s+NOT\s+NULL$`)
		for _, c := range src {
			if c.ConstraintType == "CHECK" && c.Definition.Valid {
				if m := re.FindStringSubmatch(c.Definition.String); len(m) == 2 {
					k := c.TableName + "_" + m[1] + "_not_null"
					res[k] = c
					continue
				}
			}
		}
		for k, c := range src {
			if c.ConstraintType == "CHECK" && c.Definition.Valid {
				if re.MatchString(c.Definition.String) {
					continue
				}
			}
			res[k] = c
		}
		return res
	}
	before.Constraints = normalize(before.Constraints)
	after.Constraints = normalize(after.Constraints)
	b, err := json.MarshalIndent(before, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal before: %w", err)
	}
	a, err := json.MarshalIndent(after, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal after: %w", err)
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(b)),
		B:        difflib.SplitLines(string(a)),
		FromFile: "Snapshot Before",
		ToFile:   "Snapshot After",
		Context:  3,
	}
	out, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	if out != "" {
		return fmt.Errorf("schema snapshots differ:\n%s", out)
	}
	return nil
}
