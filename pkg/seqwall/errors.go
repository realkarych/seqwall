package seqwall

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

func ErrNoMigrationFiles() error { return sentinelError("no migration files found") }
func ErrNoMigrations() error     { return sentinelError("no migrations found") }
func ErrBaselineNotFound() error { return sentinelError("baseline not found") }
func ErrSnapshotsDiffer() error  { return sentinelError("schema snapshots differ") }
func ErrPostgresURLRequired() error {
	return sentinelError("postgres URL or DATABASE_URL env is required")
}
