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
	if len(migrations) == 0 {
		log.Fatalf("No migrations found in %s", s.migrationsPath)
	}

	log.Printf("Successfully recognized %d migrations!", len(migrations))
	log.Println("Processing staircase...")

	s.processStaircase(migrations)
	defer client.Close()
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
	log.Printf("Running staircase rollback with %d steps", steps)

	for i := steps; i > 0; i-- {
		log.Printf("Reverting migration %s (step %d)", migrations[i-1], steps-i+1)
		output, err := s.executeCommand(s.migrateDown)
		if err != nil {
			log.Fatalf("Rollback %d failed: %v", i, err)
		}
		log.Println("Migrations rolled back:", output)

		log.Printf("Reapplying 1 migration...")
		output, err = s.executeCommand(s.migrateUp)
		if err != nil {
			log.Fatalf("Reapply migration failed: %v", err)
		}
		log.Println("Migration reapplied:", output)
	}

	log.Println("Staircase test (down-up-down) completed successfully!")
}

func (s *StaircaseCli) processUpDownUp(migrations []string) {
	log.Println("Step 3: Run staircase test (up-down-up)...")

	steps := s.calculateStairDepth(migrations)
	log.Printf("Running staircase upgrade with %d steps", steps)

	for i := 1; i <= steps; i++ {
		log.Printf("Applying migration %s (step %d)", migrations[i-1], i)
		output, err := s.executeCommand(s.migrateUp)
		if err != nil {
			log.Fatalf("Migration %s failed: %v", migrations[i-1], err)
		}
		log.Println("Migration applied:", output)
		log.Printf("Rolling back 1 migration")
		output, err = s.executeCommand(s.migrateDown)
		if err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		log.Println("Migration rolled back:", output)
	}

	log.Println("Staircase test (up-down-up) completed successfully!")
}

func (s *StaircaseCli) executeCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (s *StaircaseCli) calculateStairDepth(migrations []string) int {
	steps := len(migrations)
	if s.depth > 0 && s.depth < steps {
		steps = s.depth
	}
	return steps
}
