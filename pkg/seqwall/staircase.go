package seqwall

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/lib/pq"
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
	log.Println("Processing staircase...")
	if err := s.processStaircase(migrations); err != nil {
		return fmt.Errorf("staircase failed: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) processStaircase(migrations []string) error {
	if s.compareSchemaSnapshots {
		snapshot, err := s.makeSchemaSnapshot()
		if err != nil {
			return fmt.Errorf("initial snapshot: %w", err)
		}
		s.initialBaseline = snapshot
	}
	log.Println("✨ Step 1: DB actualisation — migrating all migrations up...")
	if err := s.actualiseDb(migrations); err != nil {
		return fmt.Errorf("actualise db: %w", err)
	}
	log.Println("🕵️‍♂️ Step 2: Down-Up-Down phase — testing schema consistency...")
	if err := s.processDownUpDown(migrations); err != nil {
		return fmt.Errorf("down-up-down phase: %w", err)
	}
	depth := s.calculateStairDepth(migrations)
	tail := migrations[len(migrations)-depth:]
	log.Printf("🚚 Step 3: Re-applying %d migration(s) to reach the latest schema...", len(tail))
	if err := s.reapplyMigrations(tail); err != nil {
		return fmt.Errorf("re-actualise phase: %w", err)
	}
	log.Println("🎉 Staircase test completed successfully!")
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
		if s.compareSchemaSnapshots {
			snap, err := s.makeSchemaSnapshot()
			if err != nil {
				return fmt.Errorf("snapshot after %q: %w", migration, err)
			}
			s.baseline[migration] = snap
		}
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

func (s *StaircaseWorker) processDownUpDown(migs []string) error {
	steps := s.calculateStairDepth(migs)
	for i := 1; i <= steps; i++ {
		mig := migs[len(migs)-i]
		var cur, prev *driver.SchemaSnapshot
		if s.compareSchemaSnapshots {
			var ok bool
			cur, ok = s.baseline[mig]
			if !ok {
				return fmt.Errorf("%w: %s", ErrBaselineNotFound(), mig)
			}
			idx := len(migs) - i
			if idx == 0 {
				prev = s.initialBaseline
				if prev == nil {
					return fmt.Errorf("%w: initial snapshot", ErrBaselineNotFound())
				}
			} else {
				predecessor := migs[idx-1]
				prev, ok = s.baseline[predecessor]
				if !ok {
					return fmt.Errorf("%w: %s", ErrBaselineNotFound(), predecessor)
				}
			}
		}
		if err := s.runDownUpDown(mig, i, cur, prev); err != nil {
			return err
		}
	}
	log.Println("Step 2 (down-up-down) completed successfully!")
	return nil
}

func (s *StaircaseWorker) reapplyMigrations(migrations []string) error {
	for i, mig := range migrations {
		log.Printf("Re-applying migration %d/%d: %s", i+1, len(migrations), mig)
		out, err := s.executeCommand(s.upgradeCmd, mig)
		if err != nil {
			return fmt.Errorf("re-apply migration %q (step %d): %w", mig, i+1, err)
		}
		log.Println("Migration output:", out)

		if s.compareSchemaSnapshots {
			exp, ok := s.baseline[mig]
			if !ok {
				return fmt.Errorf("%w: %s", ErrBaselineNotFound(), mig)
			}
			if err := s.compareAndSnapshot(exp, fmt.Sprintf("snapshot after re-apply %q", mig)); err != nil {
				return err
			}
		}
	}
	log.Println("Re-actualise completed successfully!")
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
	if strings.Contains(command, CurrentMigrationPlaceholder) {
		if !supportsLegacyPlaceholder(migration) {
			usage := `"$` + CurrentMigrationEnv + `"`
			if runtime.GOOS == "windows" {
				usage = "a helper that reads " + CurrentMigrationEnv
			}
			return "", fmt.Errorf("cannot substitute %s for this filename; use %s in the migration command", CurrentMigrationPlaceholder, usage)
		}
		command = strings.ReplaceAll(command, CurrentMigrationPlaceholder, migration)
	}
	cmd := newShellCommand(command)
	cmd.Env = append(os.Environ(), CurrentMigrationEnv+"="+migration)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command %q failed: %v\nCallback:\n%s\nStacktrace:\n%s",
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
		MatViews:    make(map[string]driver.MatViewDefinition),
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
		{s.scanMatViews, "matviews"},
		{s.scanPrivileges, "privileges"},
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
            SELECT pg_catalog.format('%%I.%%I', schemaname, tablename)
            FROM pg_catalog.pg_tables
            WHERE %s
            ORDER BY schemaname, tablename;
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
            pg_catalog.format('%%I.%%I', c.table_schema, c.table_name),
            c.column_name,
            c.data_type,
            c.udt_name,
            t.typtype,
            t.typcategory,
            CASE WHEN t.typtype = 'e' OR (t.typtype = 'd' AND t.typcategory = 'E')
                THEN 0 ELSE a.atttypid END AS type_oid,
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
        JOIN pg_catalog.pg_namespace n
            ON n.nspname = c.table_schema
        JOIN pg_catalog.pg_class r
            ON r.relnamespace = n.oid
            AND r.relname = c.table_name
        JOIN pg_catalog.pg_attribute a
            ON a.attrelid = r.oid
            AND a.attname = c.column_name
        JOIN pg_catalog.pg_type t
            ON t.oid = a.atttypid
        WHERE %s
        ORDER BY c.table_schema, c.table_name, c.ordinal_position;
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
                pg_catalog.format('%%I.%%I', schemaname, viewname),
                definition
            FROM pg_views
            WHERE %s
            ORDER BY schemaname, viewname;
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
            SELECT pg_catalog.format('%%I.%%I', schemaname, indexname), indexdef
            FROM pg_indexes
            WHERE %s
            ORDER BY schemaname, indexname;
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
                pg_catalog.format('%%I.%%I.%%I', n.nspname, r.relname, c.conname),
                c.conname,
                n.nspname,
                r.relname,
                CASE c.contype
                    WHEN 'c' THEN 'CHECK'
                    WHEN 'f' THEN 'FOREIGN KEY'
                    WHEN 'p' THEN 'PRIMARY KEY'
                    WHEN 'u' THEN 'UNIQUE'
                    WHEN 'x' THEN 'EXCLUDE'
                END,
                pg_catalog.pg_get_constraintdef(c.oid, false),
                c.condeferrable,
                c.condeferred,
                c.convalidated,
                c.connoinherit,
                COALESCE((to_jsonb(c)->>'conenforced')::boolean, true)
            FROM pg_catalog.pg_constraint c
            JOIN pg_catalog.pg_class r ON r.oid = c.conrelid
            JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
            WHERE %s
              AND c.contype IN ('c', 'f', 'p', 'u', 'x')
            ORDER BY n.nspname, r.relname, c.conname;
        `,
		s.buildSchemaCond("n.nspname"),
	)
	constrRows, err := s.dbClient.Execute(constraintsQuery)
	if err != nil {
		return fmt.Errorf("query constraints: %w", err)
	}
	defer constrRows.Rows.Close()
	for constrRows.Rows.Next() {
		var (
			constraintKey, constraintName  string
			tableSchema, tableName         string
			constraintType                 string
			definition                     sql.NullString
			deferrable, initiallyDeferred  bool
			validated, noInherit, enforced bool
		)
		if err := constrRows.Rows.Scan(
			&constraintKey,
			&constraintName,
			&tableSchema,
			&tableName,
			&constraintType,
			&definition,
			&deferrable,
			&initiallyDeferred,
			&validated,
			&noInherit,
			&enforced,
		); err != nil {
			return fmt.Errorf("scan constraint row: %w", err)
		}
		snapshot.Constraints[constraintKey] = driver.ConstraintDefinition{
			TableSchema:       tableSchema,
			TableName:         tableName,
			ConstraintType:    constraintType,
			Definition:        definition,
			Deferrable:        deferrable,
			InitiallyDeferred: initiallyDeferred,
			Validated:         validated,
			NoInherit:         noInherit,
			Enforced:          enforced,
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
                pg_catalog.format('%%I.%%I', n.nspname, t.typname),
                e.enumlabel
            FROM pg_type t
            JOIN pg_enum e ON t.oid = e.enumtypid
            JOIN pg_namespace n ON n.oid = t.typnamespace
            WHERE %s
            ORDER BY n.nspname, t.typname, e.enumsortorder;
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
                pg_catalog.format('%%I.%%I.%%I', n.nspname, r.relname, c.conname),
                c.conname,
                n.nspname,
                r.relname,
                ARRAY(
                    SELECT a.attname::text
                    FROM unnest(c.conkey) WITH ORDINALITY k(attnum, ord)
                    JOIN pg_catalog.pg_attribute a
                      ON a.attrelid = c.conrelid
                     AND a.attnum = k.attnum
                    ORDER BY k.ord
                ),
                fn.nspname,
                fr.relname,
                ARRAY(
                    SELECT a.attname::text
                    FROM unnest(c.confkey) WITH ORDINALITY k(attnum, ord)
                    JOIN pg_catalog.pg_attribute a
                      ON a.attrelid = c.confrelid
                     AND a.attnum = k.attnum
                    ORDER BY k.ord
                ),
                pg_catalog.pg_get_constraintdef(c.oid, false),
                CASE c.confupdtype
                    WHEN 'a' THEN 'NO ACTION'
                    WHEN 'r' THEN 'RESTRICT'
                    WHEN 'c' THEN 'CASCADE'
                    WHEN 'n' THEN 'SET NULL'
                    WHEN 'd' THEN 'SET DEFAULT'
                END,
                CASE c.confdeltype
                    WHEN 'a' THEN 'NO ACTION'
                    WHEN 'r' THEN 'RESTRICT'
                    WHEN 'c' THEN 'CASCADE'
                    WHEN 'n' THEN 'SET NULL'
                    WHEN 'd' THEN 'SET DEFAULT'
                END
            FROM pg_catalog.pg_constraint c
            JOIN pg_catalog.pg_class r ON r.oid = c.conrelid
            JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
            JOIN pg_catalog.pg_class fr ON fr.oid = c.confrelid
            JOIN pg_catalog.pg_namespace fn ON fn.oid = fr.relnamespace
            WHERE c.contype = 'f'
              AND %s
            ORDER BY n.nspname, r.relname, c.conname;
        `,
		s.buildSchemaCond("n.nspname"),
	)
	rows, err := s.dbClient.Execute(foreignKeysQuery)
	if err != nil {
		return fmt.Errorf("query foreign keys: %w", err)
	}
	defer rows.Rows.Close()
	for rows.Rows.Next() {
		var (
			constraintKey, constraintName string
			tableSchema, tableName        string
			columnNames                   pq.StringArray
			foreignTableSchema            string
			foreignTableName              string
			foreignColumnNames            pq.StringArray
			definition, updateRule        string
			deleteRule                    string
		)
		if err := rows.Rows.Scan(
			&constraintKey,
			&constraintName,
			&tableSchema,
			&tableName,
			&columnNames,
			&foreignTableSchema,
			&foreignTableName,
			&foreignColumnNames,
			&definition,
			&updateRule,
			&deleteRule,
		); err != nil {
			return fmt.Errorf("scan foreign key row: %w", err)
		}
		snapshot.ForeignKeys[constraintKey] = driver.ForeignKeyDefinition{
			ConstraintName:     constraintName,
			TableSchema:        tableSchema,
			TableName:          tableName,
			ColumnNames:        columnNames,
			ForeignTableSchema: foreignTableSchema,
			ForeignTableName:   foreignTableName,
			ForeignColumnNames: foreignColumnNames,
			Definition:         definition,
			UpdateRule:         updateRule,
			DeleteRule:         deleteRule,
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
                pg_catalog.format('%%I.%%I.%%I', n.nspname, r.relname, t.tgname),
                t.tgname,
                n.nspname,
                r.relname,
                pg_catalog.pg_get_triggerdef(t.oid, false),
                t.tgenabled::text
            FROM pg_catalog.pg_trigger t
            JOIN pg_catalog.pg_class r ON r.oid = t.tgrelid
            JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
            WHERE NOT t.tgisinternal AND %s
            ORDER BY n.nspname, r.relname, t.tgname;
        `,
		s.buildSchemaCond("n.nspname"),
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
			triggerKey, triggerName string
			tableSchema, tableName  string
			definition, enabled     string
		)
		if err := rows.Rows.Scan(
			&triggerKey,
			&triggerName,
			&tableSchema,
			&tableName,
			&definition,
			&enabled,
		); err != nil {
			return fmt.Errorf("scan trigger row: %w", err)
		}
		snapshot.Triggers[triggerKey] = driver.TriggerDefinition{
			TriggerName: triggerName,
			TableSchema: tableSchema,
			TableName:   tableName,
			Definition:  definition,
			Enabled:     enabled,
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
            SELECT p.proname,
                   CASE WHEN p.prokind = 'p' THEN 'PROCEDURE' ELSE 'FUNCTION' END,
                   COALESCE(pg_catalog.pg_get_function_result(p.oid), ''),
                   pg_catalog.format('%%I.%%I(%%s)', n.nspname, p.proname,
                                     pg_catalog.pg_get_function_identity_arguments(p.oid)),
                   pg_catalog.pg_get_functiondef(p.oid)
            FROM pg_catalog.pg_proc p
            JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
            WHERE p.prokind IN ('f', 'p', 'w') AND %s
            ORDER BY n.nspname, p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid);
        `,
		s.buildSchemaCond("n.nspname"),
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
		var routine driver.FunctionDefinition
		if err := rows.Rows.Scan(
			&routine.RoutineName,
			&routine.RoutineType,
			&routine.ReturnType,
			&routine.RoutineIdentity,
			&routine.Definition,
		); err != nil {
			return fmt.Errorf("scan function row: %w", err)
		}
		snapshot.Functions[routine.RoutineIdentity] = routine
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate function rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanSeqs(snapshot *driver.SchemaSnapshot) error {
	seqQuery := fmt.Sprintf(
		`
            SELECT pg_catalog.format('%%I.%%I', sequence_schema, sequence_name),
                   sequence_name, data_type, start_value, minimum_value, maximum_value, increment, cycle_option
            FROM information_schema.sequences
            WHERE %s
            ORDER BY sequence_schema, sequence_name;
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
			sequenceKey, sequenceName, dataType string
			startValue                          string
			minValue, maxValue, increment       string
			cycleOption                         string
		)
		if err := rows.Rows.Scan(
			&sequenceKey,
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
		snapshot.Sequences[sequenceKey] = driver.SequenceDefinition{
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

func (s *StaircaseWorker) scanMatViews(snapshot *driver.SchemaSnapshot) error {
	matviewsQuery := fmt.Sprintf(
		`
            SELECT
                pg_catalog.format('%%I.%%I', schemaname, matviewname),
                definition,
                ispopulated
            FROM pg_matviews
            WHERE %s
            ORDER BY schemaname, matviewname;
        `,
		s.buildSchemaCond("schemaname"),
	)
	rows, err := s.dbClient.Execute(matviewsQuery)
	if err != nil {
		return fmt.Errorf("query matviews: %w", err)
	}
	defer rows.Rows.Close()
	for rows.Rows.Next() {
		var matviewName, matviewDefinition string
		var isPopulated bool
		if err := rows.Rows.Scan(
			&matviewName,
			&matviewDefinition,
			&isPopulated,
		); err != nil {
			return fmt.Errorf("scan matview row: %w", err)
		}
		snapshot.MatViews[matviewName] = driver.MatViewDefinition{
			Definition:  matviewDefinition,
			IsPopulated: isPopulated,
		}
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate matview rows: %w", err)
	}
	return nil
}

func (s *StaircaseWorker) scanPrivileges(snapshot *driver.SchemaSnapshot) error {
	privQuery := fmt.Sprintf(
		`
            SELECT table_schema, grantee, table_name, privilege_type, is_grantable
		    FROM information_schema.role_table_grants
		    WHERE %s
		    ORDER BY table_schema, table_name, grantee, privilege_type, is_grantable;
        `,
		s.buildSchemaCond("table_schema"),
	)
	rows, err := s.dbClient.Execute(privQuery)
	if err != nil {
		return fmt.Errorf("query privileges: %w", err)
	}
	defer rows.Rows.Close()
	var privs []driver.PrivilegeDefinition
	for rows.Rows.Next() {
		var tableSchema, grantee, tableName, privilegeType, isGrantable string
		if err := rows.Rows.Scan(&tableSchema, &grantee, &tableName, &privilegeType, &isGrantable); err != nil {
			return fmt.Errorf("scan privilege row: %w", err)
		}
		privs = append(privs, driver.PrivilegeDefinition{
			TableSchema: tableSchema,
			Grantee:     grantee,
			TableName:   tableName,
			Privilege:   privilegeType,
			IsGrantable: isGrantable,
		})
	}
	if err := rows.Rows.Err(); err != nil {
		return fmt.Errorf("iterate privilege rows: %w", err)
	}
	snapshot.Privileges = privs
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
