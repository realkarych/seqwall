package seqwall

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const migrationTestEnv = "SEQWALL_CURRENT_MIGRATION"

var unusualMigrations = []string{
	"migrations/001_create.sql", "space name.sql", "tab\tname.sql", "line\nname.sql",
	"apostrophe'.sql", "double\"quote.sql", "dollar$.sql", "back`tick.sql",
	"semicolon;.sql", "миграция.sql", "glob*?[abc].sql", "ampersand&.sql",
	`back\slash.sql`, "$(printf AUDIT_MARKER).sql", "`printf AUDIT_MARKER`.sql",
	"{current_migration}.sql", "%PATH%!SEQWALL_TEST!^&().sql", "pipe|colon:.sql",
	"space '\"$`;&\\миграция*?%!.sql", "",
}

type commandResult struct {
	Migration string
	Args      []string
	Inherited string
	Content   string
}

func TestMigrationCommandHelper(t *testing.T) {
	if os.Getenv("SEQWALL_COMMAND_HELPER") != "1" {
		return
	}
	args := os.Args[3:]
	result := commandResult{
		Migration: os.Getenv(migrationTestEnv),
		Args:      args[1:],
		Inherited: os.Getenv("SEQWALL_TEST_INHERITED"),
	}
	switch args[0] {
	case "grandchild":
		cmd := exec.Command(os.Args[0], "-test.run=^TestMigrationCommandHelper$", "--", "read")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	case "file":
		content, err := os.ReadFile(result.Migration)
		if err != nil {
			t.Fatal(err)
		}
		result.Content = string(content)
	case "record":
		file, err := os.OpenFile(os.Getenv("SEQWALL_TEST_LOG"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(file).Encode(result); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	case "fail":
		fmt.Fprint(os.Stdout, "stdout\n")
		fmt.Fprint(os.Stderr, "stderr\n")
		os.Exit(42)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func commandHelper(t *testing.T) string {
	t.Helper()
	t.Setenv("SEQWALL_COMMAND_HELPER", "1")
	if runtime.GOOS != "windows" {
		t.Setenv("SHELL", "sh")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return quoteCommandPath(executable) + " -test.run=^TestMigrationCommandHelper$ --"
}

func quoteCommandPath(path string) string {
	if runtime.GOOS == "windows" {
		return `"` + path + `"`
	}
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}

func runCommandResult(t *testing.T, command, migration string) commandResult {
	t.Helper()
	output, err := (&StaircaseWorker{}).executeCommand(command, migration)
	if err != nil {
		t.Fatalf("executeCommand(%q, %q): %v; output: %q", command, migration, err, output)
	}
	var result commandResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode output %q: %v", output, err)
	}
	return result
}

func TestCommandEnvironmentLiteral(t *testing.T) {
	command := commandHelper(t)
	if CurrentMigrationEnv != migrationTestEnv {
		t.Fatalf("environment name = %q, want %q", CurrentMigrationEnv, migrationTestEnv)
	}
	t.Setenv("SEQWALL_TEST_INHERITED", "inherited value")
	for _, migration := range unusualMigrations {
		t.Run(migration, func(t *testing.T) {
			result := runCommandResult(t, command+" read", migration)
			if result.Migration != migration || result.Inherited != "inherited value" || len(result.Args) != 0 {
				t.Fatalf("child received %+v; want migration %q and inherited environment", result, migration)
			}
		})
	}
}

func TestCommandEnvironmentIsolation(t *testing.T) {
	command := commandHelper(t)
	key := migrationTestEnv
	if runtime.GOOS == "windows" {
		key = "Seqwall_Current_Migration"
	}
	t.Setenv(key, "stale parent value")
	for _, migration := range []string{"first migration.sql", "second ' migration.sql"} {
		result := runCommandResult(t, command+" grandchild", migration)
		if result.Migration != migration {
			t.Fatalf("grandchild migration = %q, want %q", result.Migration, migration)
		}
		if got := os.Getenv(key); got != "stale parent value" {
			t.Fatalf("parent environment changed to %q", got)
		}
	}
}

func TestCommandCombinedOutputAndStatus(t *testing.T) {
	command := commandHelper(t)
	output, err := (&StaircaseWorker{}).executeCommand(command+" fail", "unusual '\";$ migration.sql")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 42 {
		t.Fatalf("error = %v, want exit status 42", err)
	}
	if output != "stdout\nstderr\n" {
		t.Fatalf("combined output = %q", output)
	}
}

func TestCommandLegacyAccepted(t *testing.T) {
	command := commandHelper(t)
	migrations := []string{"001_name-2.sql", "./migrations/001.sql", "/migrations/001.sql"}
	if runtime.GOOS == "windows" {
		migrations = append(migrations, `C:\migrations\001.sql`, `\\server\share\001.sql`)
	}
	for _, migration := range migrations {
		for _, template := range []string{CurrentMigrationPlaceholder, `"{current_migration}"`, "--file={current_migration}", "{current_migration} {current_migration}"} {
			result := runCommandResult(t, command+" read "+template, migration)
			want := []string{migration}
			if strings.HasPrefix(template, "--file=") {
				want = []string{"--file=" + migration}
			} else if strings.Count(template, CurrentMigrationPlaceholder) == 2 {
				want = []string{migration, migration}
			}
			if !reflect.DeepEqual(result.Args, want) {
				t.Fatalf("template %q, migration %q: argv = %q, want %q", template, migration, result.Args, want)
			}
		}
	}
}

func TestCommandLegacyRejectedBeforeExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	contexts := []string{"{current_migration}", `"{current_migration}"`, "'{current_migration}'", "--file={current_migration}", "{current_migration} {current_migration}"}
	excluded := " \t\n\r'\"$`;*?[]&|(){}!%^#~+=,@<>\x01\x7fмиграция"
	separator := " & "
	if runtime.GOOS != "windows" {
		excluded += "\\:"
		separator = "; "
	}
	for _, char := range excluded {
		for _, context := range contexts {
			command := "echo started > " + quoteCommandPath(marker) + separator + "echo " + context
			output, err := (&StaircaseWorker{}).executeCommand(command, "name"+string(char)+".sql")
			if err == nil || !strings.Contains(err.Error(), migrationTestEnv) {
				t.Fatalf("character %q, context %q: output %q, error %v; want environment guidance", char, context, output, err)
			}
			if output != "" {
				t.Fatalf("rejected command produced output %q", output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("rejected command started: marker stat error %v", err)
			}
		}
	}
}

func TestProcessStaircaseMigrationEnvironment(t *testing.T) {
	command := commandHelper(t)
	logPath := filepath.Join(t.TempDir(), "steps.json")
	t.Setenv("SEQWALL_TEST_LOG", logPath)
	migrations := []string{"1 first.sql", "2 'second.sql"}
	worker := &StaircaseWorker{upgradeCmd: command + " record up", downgradeCmd: command + " record down"}
	if err := worker.processStaircase(migrations); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var got []string
	decoder := json.NewDecoder(file)
	for {
		var result commandResult
		if err := decoder.Decode(&result); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		got = append(got, result.Args[0]+":"+result.Migration)
	}
	want := []string{
		"up:1 first.sql", "up:2 'second.sql",
		"down:2 'second.sql", "up:2 'second.sql", "down:2 'second.sql",
		"down:1 first.sql", "up:1 first.sql", "down:1 first.sql",
		"up:1 first.sql", "up:2 'second.sql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("phase migrations = %q, want %q", got, want)
	}
}
