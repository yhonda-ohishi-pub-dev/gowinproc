// +build windows

package process

import (
	"fmt"
	"os/exec"
	"strconv"
)

// killProcessByPID kills a process by its PID on Windows
func killProcessByPID(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill failed for PID %d: %w (output: %s)", pid, err, string(output))
	}
	return nil
}
