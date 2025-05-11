//go:build windows
// +build windows

package seqwall

import (
	"os"
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
