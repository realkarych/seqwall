package seqwall

import (
	"database/sql"
	"encoding/json"
	"log"
	"os/exec"
	"regexp"
	"runtime/debug"

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

func (s *StaircaseCli) Run() {
	client, err := driver.NewPostgresClient(s.postgresUrl)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	s.dbClient = client
	defer s.dbClient.Close()

	migrations, err := loadMigrations(s.migrationsPath)
	if err != nil {
		log.Fatalf("Failed to load migrations: %v", err)
	}
	if len(migrations) == 0 {
		log.Fatalf("No migrations found in %s", s.migrationsPath)
	}

	log.Printf("Successfully recognized %d migrations!", len(migrations))
	log.Println("Processing staircase...")

	s.processStaircase(migrations)
}

func (s *StaircaseCli) processStaircase(migrations []string) {
	log.Println("Step 1: DB actualisation. Migrating up all migrations...")
	s.actualiseDb(migrations)
	s.processDownUpDown(migrations)
	s.processUpDownUp(migrations)
}

func (s *StaircaseCli) actualiseDb(migrations []string) {
	for iter, migration := range migrations {
		log.Printf("Running migration %d: %s", iter+1, migration)
		output, err := s.executeCommand(s.migrateUp)
		if err != nil {
			log.Fatalf("Migration %s failed: %v", migration, err)
		}
		log.Println("Migration output:", output)
	}
}

func (s *StaircaseCli) processDownUpDown(migrations []string) {
	log.Println("Step 2: Run staircase test (down-up-down)...")
	steps := s.calculateStairDepth(migrations)
	log.Printf("Running staircase test with %d steps", steps)

	for i := 1; i <= steps; i++ {
		migration := migrations[len(migrations)-i]
		var snapBefore, snapAfter *driver.SchemaSnapshot
		if s.testSchema {
			snapBefore, _ = s.makeSchemaSnapshot()
		}
		s.makeDownStep(migration, i)
		s.makeUpStep(migration, i)
		if s.testSchema {
			snapAfter, _ = s.makeSchemaSnapshot()
			s.compareSchemas(snapBefore, snapAfter)
			log.Printf("schema snapshots are equal for migration %s at step %d", migration, i)
		}
		s.makeDownStep(migration, i)
	}

	log.Println("Staircase test (down-up-down) completed successfully!")
}

func (s *StaircaseCli) processUpDownUp(migrations []string) {
	log.Println("Step 3: Run staircase test (up-down-up)...")
	steps := s.calculateStairDepth(migrations)
	log.Printf("Running staircase test with %d steps", steps)

	for i := 1; i <= steps; i++ {
		migration := migrations[i-1]
		var snapBefore, snapAfter *driver.SchemaSnapshot
		if s.testSchema {
			snapBefore, _ = s.makeSchemaSnapshot()
		}
		s.makeUpStep(migration, i)
		s.makeDownStep(migration, i)
		if s.testSchema {
			snapAfter, _ = s.makeSchemaSnapshot()
			s.compareSchemas(snapBefore, snapAfter)
			log.Printf("schema snapshots are equal for migration %s at step %d", migration, i)
		}
		s.makeUpStep(migration, i)
	}

	log.Println("Staircase test (up-down-up) completed successfully!")
}

func (s *StaircaseCli) makeUpStep(migration string, step int) {
	log.Printf("Applying migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.migrateUp)
	if err != nil {
		log.Fatalf("Migration %s failed: %v", migration, err)
	}
	log.Println("Migration applied:", output)
}

func (s *StaircaseCli) makeDownStep(migration string, step int) {
	log.Printf("Reverting migration %s (step %d)", migration, step)
	output, err := s.executeCommand(s.migrateDown)
	if err != nil {
		log.Fatalf("Migration %s failed: %v", migration, err)
	}
	log.Println("Migration reverted:", output)
}

func (s *StaircaseCli) executeCommand(command string) (string, error) {
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

	columnsQuery := `
        SELECT table_name, column_name, data_type, is_nullable, column_default, character_maximum_length, numeric_precision, numeric_scale
        FROM information_schema.columns
        WHERE table_schema = 'public'
        ORDER BY table_name, ordinal_position;
    `
	colRows, err := s.dbClient.Execute(columnsQuery)
	if err != nil {
		log.Fatalf("error querying columns: %v", err)
	}
	defer colRows.Rows.Close()
	for colRows.Rows.Next() {
		var tableName, columnName, dataType, isNullable string
		var columnDefault sql.NullString
		var charMaxLen, numPrecision, numScale sql.NullInt64
		if err := colRows.Rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault, &charMaxLen, &numPrecision, &numScale); err != nil {
			log.Fatalf("error scanning column row: %v", err)
		}
		colDef := driver.ColumnDefinition{
			ColumnName:             columnName,
			DataType:               dataType,
			IsNullable:             isNullable,
			ColumnDefault:          columnDefault,
			CharacterMaximumLength: charMaxLen,
			NumericPrecision:       numPrecision,
			NumericScale:           numScale,
		}
		tableDef, exists := snapshot.Tables[tableName]
		if !exists {
			tableDef = driver.TableDefinition{}
		}
		tableDef.Columns = append(tableDef.Columns, colDef)
		snapshot.Tables[tableName] = tableDef
	}

	viewsQuery := `
        SELECT table_name, view_definition
        FROM information_schema.views
        WHERE table_schema = 'public';
    `
	viewRows, err := s.dbClient.Execute(viewsQuery)
	if err != nil {
		log.Fatalf("error querying views: %v", err)
	}
	defer viewRows.Rows.Close()
	for viewRows.Rows.Next() {
		var viewName, viewDefinition string
		if err := viewRows.Rows.Scan(&viewName, &viewDefinition); err != nil {
			log.Fatalf("error scanning view row: %v", err)
		}
		snapshot.Views[viewName] = driver.ViewDefinition{Definition: viewDefinition}
	}

	indexesQuery := `
        SELECT indexname, indexdef
        FROM pg_indexes
        WHERE schemaname = 'public';
    `
	indexRows, err := s.dbClient.Execute(indexesQuery)
	if err != nil {
		log.Fatalf("error querying indexes: %v", err)
	}
	defer indexRows.Rows.Close()
	for indexRows.Rows.Next() {
		var indexName, indexDef string
		if err := indexRows.Rows.Scan(&indexName, &indexDef); err != nil {
			log.Fatalf("error scanning index row: %v", err)
		}
		snapshot.Indexes[indexName] = driver.IndexDefinition{IndexDef: indexDef}
	}

	constraintsQuery := `
        SELECT tc.constraint_name, tc.table_name, tc.constraint_type, cc.check_clause
        FROM information_schema.table_constraints tc
        LEFT JOIN information_schema.check_constraints cc ON tc.constraint_name = cc.constraint_name
        WHERE tc.table_schema = 'public';
    `
	constrRows, err := s.dbClient.Execute(constraintsQuery)
	if err != nil {
		log.Fatalf("error querying constraints: %v", err)
	}
	defer constrRows.Rows.Close()
	for constrRows.Rows.Next() {
		var constraintName, tableName, constraintType string
		var checkClause sql.NullString
		if err := constrRows.Rows.Scan(&constraintName, &tableName, &constraintType, &checkClause); err != nil {
			log.Fatalf("error scanning constraint row: %v", err)
		}
		snapshot.Constraints[constraintName] = driver.ConstraintDefinition{
			TableName:      tableName,
			ConstraintType: constraintType,
			Definition:     checkClause,
		}
	}

	enumQuery := `
        SELECT t.typname, e.enumlabel
        FROM pg_type t
        JOIN pg_enum e ON t.oid = e.enumtypid
        ORDER BY t.typname, e.enumsortorder;
    `
	enumRows, err := s.dbClient.Execute(enumQuery)
	if err != nil {
		log.Fatalf("error querying enum types: %v", err)
	}
	defer enumRows.Rows.Close()
	for enumRows.Rows.Next() {
		var typeName, enumLabel string
		if err := enumRows.Rows.Scan(&typeName, &enumLabel); err != nil {
			log.Fatalf("error scanning enum row: %v", err)
		}
		enumDef, exists := snapshot.EnumTypes[typeName]
		if !exists {
			enumDef = driver.EnumDefinition{}
		}
		enumDef.Labels = append(enumDef.Labels, enumLabel)
		snapshot.EnumTypes[typeName] = enumDef
	}

	foreignKeysQuery := `
        SELECT
            tc.constraint_name,
            tc.table_name,
            kcu.column_name,
            ccu.table_name AS foreign_table_name,
            ccu.column_name AS foreign_column_name,
            rc.update_rule,
            rc.delete_rule
        FROM
            information_schema.table_constraints AS tc
        JOIN information_schema.key_column_usage AS kcu
            ON tc.constraint_name = kcu.constraint_name
        JOIN information_schema.referential_constraints AS rc
            ON tc.constraint_name = rc.constraint_name
        JOIN information_schema.constraint_column_usage AS ccu
            ON ccu.constraint_name = tc.constraint_name
        WHERE tc.constraint_type = 'FOREIGN KEY'
        AND tc.table_schema = 'public';
    `
	foreignRows, err := s.dbClient.Execute(foreignKeysQuery)
	if err != nil {
		log.Fatalf("error querying foreign keys: %v", err)
	}
	defer foreignRows.Rows.Close()
	for foreignRows.Rows.Next() {
		var constraintName, tableName, columnName, foreignTableName, foreignColumnName, updateRule, deleteRule string
		if err := foreignRows.Rows.Scan(&constraintName, &tableName, &columnName, &foreignTableName, &foreignColumnName, &updateRule, &deleteRule); err != nil {
			log.Fatalf("error scanning foreign key row: %v", err)
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
	triggersQuery := `
		SELECT trigger_name, event_manipulation, event_object_table, action_timing, action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = 'public'
        ORDER BY trigger_name;
	`
	triggerRows, err := s.dbClient.Execute(triggersQuery)
	if err != nil {
		log.Fatalf("error querying triggers: %v", err)
	}
	defer triggerRows.Rows.Close()

	if snapshot.Triggers == nil {
		snapshot.Triggers = make(map[string]driver.TriggerDefinition)
	}
	for triggerRows.Rows.Next() {
		var triggerName, eventManipulation, eventObjectTable, actionTiming, actionStatement string
		if err := triggerRows.Rows.Scan(&triggerName, &eventManipulation, &eventObjectTable, &actionTiming, &actionStatement); err != nil {
			log.Fatalf("error scanning trigger row: %v", err)
		}
		snapshot.Triggers[triggerName] = driver.TriggerDefinition{
			TriggerName:       triggerName,
			EventManipulation: eventManipulation,
			EventObjectTable:  eventObjectTable,
			ActionTiming:      actionTiming,
			ActionStatement:   actionStatement,
		}
	}

	functionsQuery := `
		SELECT routine_name, routine_type, data_type, routine_definition
		FROM information_schema.routines
		WHERE specific_schema = 'public'
		ORDER BY routine_name;
	`
	funcRows, err := s.dbClient.Execute(functionsQuery)
	if err != nil {
		log.Fatalf("error querying functions: %v", err)
	}
	defer funcRows.Rows.Close()

	if snapshot.Functions == nil {
		snapshot.Functions = make(map[string]driver.FunctionDefinition)
	}
	for funcRows.Rows.Next() {
		var routineName, routineType, returnType, routineDefinition string
		if err := funcRows.Rows.Scan(&routineName, &routineType, &returnType, &routineDefinition); err != nil {
			log.Fatalf("error scanning function row: %v", err)
		}
		snapshot.Functions[routineName] = driver.FunctionDefinition{
			RoutineName:       routineName,
			RoutineType:       routineType,
			ReturnType:        returnType,
			RoutineDefinition: routineDefinition,
		}
	}

	sequencesQuery := `
		SELECT sequence_name, data_type, start_value, minimum_value, maximum_value, increment, cycle_option
		FROM information_schema.sequences
		WHERE sequence_schema = 'public'
		ORDER BY sequence_name;
	`
	seqRows, err := s.dbClient.Execute(sequencesQuery)
	if err != nil {
		log.Fatalf("error querying sequences: %v", err)
	}
	defer seqRows.Rows.Close()

	if snapshot.Sequences == nil {
		snapshot.Sequences = make(map[string]driver.SequenceDefinition)
	}
	for seqRows.Rows.Next() {
		var sequenceName, dataType, startValue, minValue, maxValue, increment, cycleOption string
		if err := seqRows.Rows.Scan(&sequenceName, &dataType, &startValue, &minValue, &maxValue, &increment, &cycleOption); err != nil {
			log.Fatalf("error scanning sequence row: %v", err)
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
	return snapshot, nil
}

func (s *StaircaseCli) compareSchemas(snapBefore, snapAfter *driver.SchemaSnapshot) {
	normalizeConstraints := func(constraints map[string]driver.ConstraintDefinition) map[string]driver.ConstraintDefinition {
		normalized := make(map[string]driver.ConstraintDefinition)
		re := regexp.MustCompile(`^([A-Za-z0-9_]+)\s+IS\s+NOT\s+NULL$`)
		for _, cons := range constraints {
			if cons.ConstraintType == "CHECK" && cons.Definition.Valid {
				matches := re.FindStringSubmatch(cons.Definition.String)
				if len(matches) == 2 {
					colName := matches[1]
					newKey := cons.TableName + "_" + colName + "_not_null"
					normalized[newKey] = cons
					continue
				}
			}
		}
		for key, cons := range constraints {
			if cons.ConstraintType == "CHECK" && cons.Definition.Valid {
				matches := re.FindStringSubmatch(cons.Definition.String)
				if len(matches) == 2 {
					continue
				}
			}
			normalized[key] = cons
		}
		return normalized
	}

	snapBefore.Constraints = normalizeConstraints(snapBefore.Constraints)
	snapAfter.Constraints = normalizeConstraints(snapAfter.Constraints)

	beforeJson, err := json.MarshalIndent(snapBefore, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling snapshot before: %v", err)
	}
	afterJson, err := json.MarshalIndent(snapAfter, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling snapshot after: %v", err)
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(beforeJson)),
		B:        difflib.SplitLines(string(afterJson)),
		FromFile: "Snapshot Before",
		ToFile:   "Snapshot After",
		Context:  3,
	}
	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		log.Fatalf("Error generating diff: %v", err)
	}
	if diffText != "" {
		log.Fatalf("Schema snapshots differ:\n%s", diffText)
	}
}
