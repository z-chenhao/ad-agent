package runtime

import (
	"os/exec"
	"strconv"
)

func configureBridgeProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return killBridgeProcess(cmd) }
}

func killBridgeProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// taskkill /T also terminates the native SDK child on Windows.
	return exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
}
