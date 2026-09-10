package seqwall

import (
	"os"
	"os/exec"
)

var newShellCommand = func(command string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	return exec.Command(shell, "-c", command)
}
