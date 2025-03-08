package seqwall

import "fmt"

type StaircaseCli struct {
	migrations string
	testSchema bool
	depth      int
}

func NewStaircaseCli(migrations string, testSchema bool, depth int) *StaircaseCli {
	return &StaircaseCli{
		migrations: migrations,
		testSchema: testSchema,
		depth:      depth,
	}
}

func (s *StaircaseCli) Run() {
	fmt.Println("Running StaircaseCli with the following parameters:")
	fmt.Printf("Path to migrations: %s\n", s.migrations)
	fmt.Printf("Check schema: %v\n", s.testSchema)
	fmt.Printf("Stair depth: %d\n", s.depth)
}
