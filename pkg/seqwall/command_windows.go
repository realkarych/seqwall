package seqwall

import (
	"os/exec"
	"syscall"
)

func init() {
	newShellCommand = func(command string) *exec.Cmd {
		cmd := exec.Command("cmd")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: syscall.EscapeArg(cmd.Path) + ` /S /C "` + command + `"`,
		}
		return cmd
	}
}
