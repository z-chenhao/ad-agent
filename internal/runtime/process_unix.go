//go:build !windows

package runtime

import (
	"os/exec"
	"syscall"
)

// A bridge can own a native SDK child. Cancel the private process group, not
// just Node, so a cancelled turn cannot leave a model connection running.
func configureBridgeProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killBridgeProcess(cmd) }
}

func killBridgeProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
