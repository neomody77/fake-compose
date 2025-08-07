package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose/pkg/compose"
	"github.com/neomody77/fake-compose/pkg/hooks"
)

type Phase string

const (
	PhasePreStart   Phase = "pre-start"
	PhaseStart      Phase = "start"
	PhasePostStart  Phase = "post-start"
	PhaseRunning    Phase = "running"
	PhasePreStop    Phase = "pre-stop"
	PhaseStop       Phase = "stop"
	PhasePostStop   Phase = "post-stop"
	PhaseStopped    Phase = "stopped"
)

type ServiceState struct {
	Name          string
	Phase         Phase
	Status        string
	Error         error
	StartTime     time.Time
	StopTime      time.Time
	InitCompleted bool
	PostCompleted bool
}

type Manager struct {
	services     map[string]*ServiceState
	hookExecutor *hooks.Executor
	mu           sync.RWMutex
	logger       *logrus.Logger
}

func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		services:     make(map[string]*ServiceState),
		hookExecutor: hooks.NewExecutor(logger),
		logger:       logger,
	}
}

func (m *Manager) StartService(ctx context.Context, serviceName string, service *compose.Service) error {
	m.mu.Lock()
	state := &ServiceState{
		Name:      serviceName,
		Phase:     PhasePreStart,
		Status:    "Starting",
		StartTime: time.Now(),
	}
	m.services[serviceName] = state
	m.mu.Unlock()

	if err := m.runInitContainers(ctx, serviceName, service); err != nil {
		return m.setError(serviceName, err)
	}

	if service.Hooks != nil && len(service.Hooks.PreStart) > 0 {
		m.logger.Infof("Running pre-start hooks for service %s", serviceName)
		if err := m.hookExecutor.ExecuteHooks(ctx, service.Hooks.PreStart); err != nil {
			return m.setError(serviceName, fmt.Errorf("pre-start hooks failed: %w", err))
		}
	}

	m.updatePhase(serviceName, PhaseStart)

	m.updatePhase(serviceName, PhasePostStart)

	if service.Hooks != nil && len(service.Hooks.PostStart) > 0 {
		m.logger.Infof("Running post-start hooks for service %s", serviceName)
		if err := m.hookExecutor.ExecuteHooks(ctx, service.Hooks.PostStart); err != nil {
			return m.setError(serviceName, fmt.Errorf("post-start hooks failed: %w", err))
		}
	}

	if err := m.runPostContainers(ctx, serviceName, service, true); err != nil {
		m.logger.Warnf("Post containers failed for service %s: %v", serviceName, err)
	}

	m.updatePhase(serviceName, PhaseRunning)
	m.updateStatus(serviceName, "Running")

	return nil
}

func (m *Manager) StopService(ctx context.Context, serviceName string, service *compose.Service) error {
	m.mu.RLock()
	state, exists := m.services[serviceName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	if state.Phase == PhaseStopped {
		return nil
	}

	m.updatePhase(serviceName, PhasePreStop)

	if service.Hooks != nil && len(service.Hooks.PreStop) > 0 {
		m.logger.Infof("Running pre-stop hooks for service %s", serviceName)
		if err := m.hookExecutor.ExecuteHooks(ctx, service.Hooks.PreStop); err != nil {
			m.logger.Warnf("Pre-stop hooks failed for service %s: %v", serviceName, err)
		}
	}

	m.updatePhase(serviceName, PhaseStop)

	m.updatePhase(serviceName, PhasePostStop)

	if service.Hooks != nil && len(service.Hooks.PostStop) > 0 {
		m.logger.Infof("Running post-stop hooks for service %s", serviceName)
		if err := m.hookExecutor.ExecuteHooks(ctx, service.Hooks.PostStop); err != nil {
			m.logger.Warnf("Post-stop hooks failed for service %s: %v", serviceName, err)
		}
	}

	if err := m.runPostContainers(ctx, serviceName, service, false); err != nil {
		m.logger.Warnf("Post containers (on failure) failed for service %s: %v", serviceName, err)
	}

	m.mu.Lock()
	state.Phase = PhaseStopped
	state.Status = "Stopped"
	state.StopTime = time.Now()
	m.mu.Unlock()

	return nil
}

func (m *Manager) runInitContainers(ctx context.Context, serviceName string, service *compose.Service) error {
	if len(service.InitContainers) == 0 {
		return nil
	}

	m.logger.Infof("Running init containers for service %s", serviceName)

	for _, initContainer := range service.InitContainers {
		m.logger.Infof("Starting init container %s for service %s", initContainer.Name, serviceName)
		
		if err := m.executeInitContainer(ctx, serviceName, &initContainer); err != nil {
			return fmt.Errorf("init container %s failed: %w", initContainer.Name, err)
		}
		
		m.logger.Infof("Init container %s completed successfully", initContainer.Name)
	}

	m.mu.Lock()
	m.services[serviceName].InitCompleted = true
	m.mu.Unlock()

	return nil
}

func (m *Manager) runPostContainers(ctx context.Context, serviceName string, service *compose.Service, onSuccess bool) error {
	if len(service.PostContainers) == 0 {
		return nil
	}

	m.logger.Infof("Running post containers for service %s (onSuccess=%v)", serviceName, onSuccess)

	for _, postContainer := range service.PostContainers {
		shouldRun := (onSuccess && postContainer.OnSuccess) || (!onSuccess && postContainer.OnFailure)
		if !shouldRun {
			continue
		}

		m.logger.Infof("Starting post container %s for service %s", postContainer.Name, serviceName)
		
		if err := m.executePostContainer(ctx, serviceName, &postContainer); err != nil {
			return fmt.Errorf("post container %s failed: %w", postContainer.Name, err)
		}
		
		m.logger.Infof("Post container %s completed successfully", postContainer.Name)
	}

	m.mu.Lock()
	m.services[serviceName].PostCompleted = true
	m.mu.Unlock()

	return nil
}

func (m *Manager) executeInitContainer(ctx context.Context, serviceName string, container *compose.InitContainer) error {
	return nil
}

func (m *Manager) executePostContainer(ctx context.Context, serviceName string, container *compose.PostContainer) error {
	if container.WaitFor != "" {
		waitDuration, err := time.ParseDuration(container.WaitFor)
		if err == nil {
			m.logger.Infof("Waiting %s before starting post container %s", waitDuration, container.Name)
			select {
			case <-time.After(waitDuration):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

func (m *Manager) GetServiceState(serviceName string) (*ServiceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, exists := m.services[serviceName]
	return state, exists
}

func (m *Manager) GetAllServiceStates() map[string]*ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	states := make(map[string]*ServiceState)
	for k, v := range m.services {
		states[k] = v
	}
	return states
}

func (m *Manager) updatePhase(serviceName string, phase Phase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, exists := m.services[serviceName]; exists {
		state.Phase = phase
		m.logger.Debugf("Service %s transitioned to phase %s", serviceName, phase)
	}
}

func (m *Manager) updateStatus(serviceName string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, exists := m.services[serviceName]; exists {
		state.Status = status
	}
}

func (m *Manager) setError(serviceName string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, exists := m.services[serviceName]; exists {
		state.Error = err
		state.Status = "Error"
	}
	return err
}