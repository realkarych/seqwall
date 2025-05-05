package seqwall

import "github.com/realkarych/seqwall/pkg/driver"

type Cli interface {
	Run()
}

type StaircaseWorker struct {
	dbClient               *driver.PostgresClient            `json:"-"`
	baseline               map[string]*driver.SchemaSnapshot `json:"-"`
	migrationsPath         string
	upgradeCmd             string
	downgradeCmd           string
	postgresURL            string
	migrationsExtension    string
	schemas                []string
	depth                  int
	compareSchemaSnapshots bool
}

func NewStaircaseWorker(
	migrationsPath string,
	compareSchemaSnapshots bool,
	depth int,
	upgradeCmd string,
	downgradeCmd string,
	postgresURL string,
	schemas []string,
	migrationsExtension string,
) *StaircaseWorker {
	return &StaircaseWorker{
		migrationsPath:         migrationsPath,
		compareSchemaSnapshots: compareSchemaSnapshots,
		depth:                  depth,
		upgradeCmd:             upgradeCmd,
		downgradeCmd:           downgradeCmd,
		postgresURL:            postgresURL,
		baseline:               make(map[string]*driver.SchemaSnapshot),
		schemas:                schemas,
		migrationsExtension:    migrationsExtension,
	}
}
