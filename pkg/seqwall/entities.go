package seqwall

type Cli interface {
	Run()
}

const CurrentMigrationPlaceholder = "{current_migration}"
