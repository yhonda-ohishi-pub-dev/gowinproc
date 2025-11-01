package models

import (
	"bytes"
	"os/exec"
	"sync"
	"time"
)

// ProcessStatus represents the status of a managed process
type ProcessStatus string

const (
	StatusStopped  ProcessStatus = "stopped"
	StatusStarting ProcessStatus = "starting"
	StatusRunning  ProcessStatus = "running"
	StatusStopping ProcessStatus = "stopping"
	StatusFailed   ProcessStatus = "failed"
	StatusUpdating ProcessStatus = "updating"
)

// ProcessInstance represents a running instance of a process
type ProcessInstance struct {
	ID          string
	ProcessName string
	Command     *exec.Cmd
	Status      ProcessStatus
	StartTime   time.Time
	PID         int
	Port        int
	Version     string
	EnvFilePath string
	StderrBuf   *bytes.Buffer  // Capture stderr for error logging
	mu          sync.RWMutex
}

// GetStatus returns the current status of the process instance
func (p *ProcessInstance) GetStatus() ProcessStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status
}

// SetStatus sets the status of the process instance
func (p *ProcessInstance) SetStatus(status ProcessStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
}

// ManagedProcess represents a process configuration with running instances
type ManagedProcess struct {
	Config    ProcessConfig
	Instances []*ProcessInstance
	mu        sync.RWMutex
}

// AddInstance adds a new instance to the managed process
func (m *ManagedProcess) AddInstance(instance *ProcessInstance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Instances = append(m.Instances, instance)
}

// RemoveInstance removes an instance from the managed process
func (m *ManagedProcess) RemoveInstance(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, inst := range m.Instances {
		if inst.ID == instanceID {
			m.Instances = append(m.Instances[:i], m.Instances[i+1:]...)
			break
		}
	}
}

// GetInstances returns all instances
func (m *ManagedProcess) GetInstances() []*ProcessInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances := make([]*ProcessInstance, len(m.Instances))
	copy(instances, m.Instances)
	return instances
}

// GetRunningInstances returns only running instances
func (m *ManagedProcess) GetRunningInstances() []*ProcessInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var running []*ProcessInstance
	for _, inst := range m.Instances {
		if inst.GetStatus() == StatusRunning {
			running = append(running, inst)
		}
	}
	return running
}
