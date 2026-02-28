package core

import (
	"context"
	"sync"
)

// Service defines the lifecycle of a background task.
// Any custom service must implement this interface.
type Service interface {
	// Name returns the unique name of the service.
	Name() string
	// Start begins the service execution. It should be non-blocking or block until ctx is canceled.
	Start(ctx context.Context) error
	// Stop gracefully terminates the service.
	Stop() error
	// Status returns the current running status or health information.
	Status() string
}

// Manager handles the registration and concurrent execution of multiple services.
type Manager struct {
	services []Service
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// NewManager creates a new service manager.
func NewManager() *Manager {
	return &Manager{
		services: make([]Service, 0),
	}
}

// Register adds a new service to the manager.
func (m *Manager) Register(s Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services = append(m.services, s)
}

// StartAll starts all registered services concurrently.
func (m *Manager) StartAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.services {
		m.wg.Add(1)
		// Run each service in its own goroutine for parallel execution
		go func(srv Service) {
			defer m.wg.Done()
			_ = srv.Start(ctx) // In a production app, handle this error (e.g., logging)
		}(s)
	}
}

// Wait blocks until all services have stopped.
func (m *Manager) Wait() {
	m.wg.Wait()
}

// GetStatuses returns a map of service names to their current statuses.
func (m *Manager) GetStatuses() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]string)
	for _, s := range m.services {
		statuses[s.Name()] = s.Status()
	}
	return statuses
}
