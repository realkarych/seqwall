package driver_test

import (
	"strings"
	"testing"

	"github.com/realkarych/seqwall/pkg/driver"
)

func TestNewPostgresClientRegistersPostgresDriver(t *testing.T) {
	_, err := driver.NewPostgresClient("postgres://127.0.0.1:notaport/seqwall?sslmode=disable")
	if err == nil {
		t.Fatal("NewPostgresClient() error = nil, want invalid connection error")
	}
	if strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("NewPostgresClient() error = %q, want registered postgres driver", err)
	}
}
