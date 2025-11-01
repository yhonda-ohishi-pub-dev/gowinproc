package loadbalancer

import (
	"fmt"
	"log"
	"sync"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager manages multiple load balancers
type Manager struct {
	balancers      map[string]*Balancer
	processManager *process.Manager
	mu             sync.RWMutex
}

// NewManager creates a new load balancer manager
func NewManager(configs []models.LoadBalancerConfig, procMgr *process.Manager) (*Manager, error) {
	mgr := &Manager{
		balancers:      make(map[string]*Balancer),
		processManager: procMgr,
	}

	// Create all load balancers
	for _, config := range configs {
		lb, err := NewBalancer(config, procMgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create load balancer %q: %w", config.Name, err)
		}
		mgr.balancers[config.Name] = lb
	}

	return mgr, nil
}

// Start starts all load balancers
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, lb := range m.balancers {
		if err := lb.Start(); err != nil {
			return fmt.Errorf("failed to start load balancer %q: %w", name, err)
		}
	}

	log.Printf("Started %d load balancer(s)", len(m.balancers))
	return nil
}

// Stop stops all load balancers
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, lb := range m.balancers {
		if err := lb.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop load balancer %q: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping load balancers: %v", errors)
	}

	log.Printf("Stopped %d load balancer(s)", len(m.balancers))
	return nil
}

// GetBalancer returns a load balancer by name
func (m *Manager) GetBalancer(name string) (*Balancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lb, ok := m.balancers[name]
	if !ok {
		return nil, fmt.Errorf("load balancer %q not found", name)
	}
	return lb, nil
}

// ListBalancers returns names of all load balancers
func (m *Manager) ListBalancers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.balancers))
	for name := range m.balancers {
		names = append(names, name)
	}
	return names
}
