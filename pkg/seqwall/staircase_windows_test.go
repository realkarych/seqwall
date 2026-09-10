//go:build windows

package seqwall

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteCommand_Windows(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.bat")
	if err := os.WriteFile(ok, []byte("@echo OFF\r\nECHO OK\r\n"), 0o755); err != nil {
		t.Fatalf("write batch: %v", err)
	}
	w := &StaircaseWorker{}
	out, err := w.executeCommand(ok, "dummy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("output = %q, want OK", out)
	}
	fail := filepath.Join(dir, "fail.bat")
	if err := os.WriteFile(fail, []byte("@EXIT /B 42\r\n"), 0o755); err != nil {
		t.Fatalf("write fail batch: %v", err)
	}
	if _, err = w.executeCommand(fail, "dummy"); err == nil {
		t.Fatalf("expected non-nil error")
	}
}

func TestCommandWindowsLiteralFile(t *testing.T) {
	command := commandHelper(t)
	dir := filepath.Join(t.TempDir(), "migration directory")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	migration := filepath.Join(dir, "001_'$`;миграция%PATH%!SEQWALL_TEST!^&().sql")
	if err := os.WriteFile(migration, []byte("exact migration content"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := runCommandResult(t, command+" file", migration)
	if result.Migration != migration || result.Content != "exact migration content" {
		t.Fatalf("file result = %+v", result)
	}
}

func TestCommandWindowsExecutableWithSpaces(t *testing.T) {
	commandHelper(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "migration helper.exe")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	command := quoteCommandPath(path) + ` -test.run=^TestMigrationCommandHelper$ -- read "quoted argument"`
	result := runCommandResult(t, command, "migration with spaces.sql")
	if len(result.Args) != 1 || result.Args[0] != "quoted argument" || result.Migration != "migration with spaces.sql" {
		t.Fatalf("helper result = %+v", result)
	}
}

func TestCommandWindowsQuotedBatch(t *testing.T) {
	for _, status := range []int{0, 42} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "migration helper.bat")
			body := fmt.Sprintf("@echo OFF\r\nECHO %%~1\r\nEXIT /B %d\r\n", status)
			if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			command := quoteCommandPath(path) + ` "quoted argument"`
			output, err := (&StaircaseWorker{}).executeCommand(command, "migration with spaces.sql")
			if status == 0 && err != nil {
				t.Fatalf("batch failed: %v; output: %q", err, output)
			}
			var exitErr *exec.ExitError
			if status != 0 && (!errors.As(err, &exitErr) || exitErr.ExitCode() != status) {
				t.Fatalf("batch error = %v, want status %d", err, status)
			}
			if output != "quoted argument\r\n" {
				t.Fatalf("batch output = %q", output)
			}
		})
	}
}
