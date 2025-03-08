package seqwall

import (
	"log"
	"os/exec"

	"github.com/realkarych/seqwall/pkg/driver"
)

type StaircaseCli struct {
	migrationsPath string
	testSchema     bool
	depth          int
	migrateUp      string
	migrateDown    string
	postgresUrl    string
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
	migrations, err := loadMigrations(s.migrationsPath)
	if err != nil {
		log.Fatalf("Failed to load migrations: %v", err)
	}

	log.Printf("Successfully recognized %d migrations!", len(migrations))
	log.Println("Processing staircase...")

	s.processStaircase(migrations)
	defer client.Close()
}

func (s *StaircaseCli) processStaircase(migrations []string) {
	log.Println("Step 1: DB actualisation. Migrating up all migrations...")
	s.actualiseDb(migrations)
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

func (s *StaircaseCli) executeCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
