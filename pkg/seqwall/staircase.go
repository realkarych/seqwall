package seqwall

import "github.com/realkarych/seqwall/pkg/driver"

type StaircaseCli struct {
	migrations  string
	testSchema  bool
	depth       int
	migrateUp   string
	migrateDown string
	postgresUrl string
}

func NewStaircaseCli(
	migrations string,
	testSchema bool,
	depth int,
	migrateUp string,
	migrateDown string,
	postgresUrl string,
) *StaircaseCli {
	return &StaircaseCli{
		migrations:  migrations,
		testSchema:  testSchema,
		depth:       depth,
		migrateUp:   migrateUp,
		migrateDown: migrateDown,
		postgresUrl: postgresUrl,
	}
}

func (s *StaircaseCli) Run() {
	client, err := driver.NewPostgresClient(s.postgresUrl)
	if err != nil {
		panic(err)
	}
	defer client.Close()
}
