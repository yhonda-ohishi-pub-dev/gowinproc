package process

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// PIDTracker tracks process IDs in a file for cleanup
type PIDTracker struct {
	filePath string
	mu       sync.Mutex
}

// NewPIDTracker creates a new PID tracker
func NewPIDTracker(filePath string) *PIDTracker {
	return &PIDTracker{
		filePath: filePath,
	}
}

// CleanupOrphans kills all PIDs in the tracking file and removes successfully killed ones
func (pt *PIDTracker) CleanupOrphans() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Read all PIDs from file
	pids, err := pt.readPIDs()
	if err != nil {
		// If file doesn't exist, that's fine
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	if len(pids) == 0 {
		return nil
	}

	log.Printf("Found %d orphaned processes to cleanup", len(pids))

	// Try to kill each process
	remainingPIDs := []int{}
	for _, pid := range pids {
		if err := killProcessByPID(pid); err != nil {
			log.Printf("Failed to kill orphaned PID %d: %v (will retry next time)", pid, err)
			remainingPIDs = append(remainingPIDs, pid)
		} else {
			log.Printf("Successfully killed orphaned PID %d", pid)
		}
	}

	// Rewrite file with only the PIDs that failed to kill
	return pt.writePIDs(remainingPIDs)
}

// Add adds a PID to the tracking file
func (pt *PIDTracker) Add(pid int) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Read existing PIDs
	pids, err := pt.readPIDs()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	// Add new PID
	pids = append(pids, pid)

	// Write back
	return pt.writePIDs(pids)
}

// Remove removes a PID from the tracking file (called after successful kill)
func (pt *PIDTracker) Remove(pid int) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Read existing PIDs
	pids, err := pt.readPIDs()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already empty
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	// Remove the PID
	newPIDs := []int{}
	for _, p := range pids {
		if p != pid {
			newPIDs = append(newPIDs, p)
		}
	}

	// Write back
	return pt.writePIDs(newPIDs)
}

// readPIDs reads all PIDs from the tracking file
func (pt *PIDTracker) readPIDs() ([]int, error) {
	file, err := os.Open(pt.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pids []int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		pid, err := strconv.Atoi(line)
		if err != nil {
			log.Printf("Invalid PID in tracking file: %s", line)
			continue
		}

		pids = append(pids, pid)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading PID file: %w", err)
	}

	return pids, nil
}

// writePIDs writes PIDs to the tracking file (overwrites)
func (pt *PIDTracker) writePIDs(pids []int) error {
	// If no PIDs, remove the file
	if len(pids) == 0 {
		if err := os.Remove(pt.filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove PID file: %w", err)
		}
		return nil
	}

	// Write PIDs to file
	file, err := os.Create(pt.filePath)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %w", err)
	}
	defer file.Close()

	for _, pid := range pids {
		if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
			return fmt.Errorf("failed to write PID to file: %w", err)
		}
	}

	return nil
}
