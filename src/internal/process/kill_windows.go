// +build windows

package process

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// getProcessName gets the process name for a given PID
func getProcessName(pid int) string {
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("(Get-Process -Id %d -ErrorAction SilentlyContinue).ProcessName", pid))
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// killProcessByPID kills a process by its PID on Windows
func killProcessByPID(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if error is because process doesn't exist (exit code 128)
		// In this case, treat as success since the goal is to ensure the process is not running
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			// Process not found - this is OK, it's already gone
			return nil
		}
		return fmt.Errorf("taskkill failed for PID %d: %w (output: %s)", pid, err, string(output))
	}
	return nil
}
