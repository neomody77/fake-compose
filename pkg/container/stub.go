package container

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose/pkg/compose"
)

type Manager struct {
	impl ContainerImplementation
}

// ContainerImplementation defines the interface for container operations
type ContainerImplementation interface {
	CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout int) error
	RemoveContainer(ctx context.Context, containerID string) error
	RunInitContainer(ctx context.Context, serviceName string, initContainer *compose.InitContainer) error
	RunPostContainer(ctx context.Context, serviceName string, postContainer *compose.PostContainer) error
	Close() error
}

func NewManager(logger *logrus.Logger) (*Manager, error) {
	// Try to create Docker manager first
	dockerManager, err := NewDockerManager(logger)
	if err != nil {
		logger.Warnf("Failed to create Docker manager, using stub: %v", err)
		return &Manager{
			impl: &StubManager{logger: logger},
		}, nil
	}

	logger.Info("Using Docker container manager")
	return &Manager{
		impl: dockerManager,
	}, nil
}

// Manager methods delegate to the implementation
func (m *Manager) CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error) {
	return m.impl.CreateService(ctx, serviceName, service)
}

func (m *Manager) StartContainer(ctx context.Context, containerID string) error {
	return m.impl.StartContainer(ctx, containerID)
}

func (m *Manager) StopContainer(ctx context.Context, containerID string, timeout int) error {
	return m.impl.StopContainer(ctx, containerID, timeout)
}

func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	return m.impl.RemoveContainer(ctx, containerID)
}

func (m *Manager) RunInitContainer(ctx context.Context, serviceName string, initContainer *compose.InitContainer) error {
	return m.impl.RunInitContainer(ctx, serviceName, initContainer)
}

func (m *Manager) RunPostContainer(ctx context.Context, serviceName string, postContainer *compose.PostContainer) error {
	return m.impl.RunPostContainer(ctx, serviceName, postContainer)
}

func (m *Manager) Close() error {
	return m.impl.Close()
}

// StubManager provides stub implementations for testing/fallback
type StubManager struct {
	logger *logrus.Logger
}

func (s *StubManager) CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error) {
	containerID := fmt.Sprintf("%s_container_%d", serviceName, time.Now().Unix())
	s.logger.Infof("[STUB] Creating container %s for service %s (image: %s)", containerID, serviceName, service.Image)
	
	// Simulate container creation time
	time.Sleep(100 * time.Millisecond)
	
	return containerID, nil
}

func (s *StubManager) StartContainer(ctx context.Context, containerID string) error {
	s.logger.Infof("[STUB] Starting container %s", containerID)
	
	// Simulate container startup time
	time.Sleep(200 * time.Millisecond)
	
	return nil
}

func (s *StubManager) StopContainer(ctx context.Context, containerID string, timeout int) error {
	s.logger.Infof("[STUB] Stopping container %s (timeout: %ds)", containerID, timeout)
	
	// Simulate container stop time
	time.Sleep(100 * time.Millisecond)
	
	return nil
}

func (s *StubManager) RemoveContainer(ctx context.Context, containerID string) error {
	s.logger.Infof("[STUB] Removing container %s", containerID)
	
	// Simulate container removal time
	time.Sleep(50 * time.Millisecond)
	
	return nil
}

func (s *StubManager) RunInitContainer(ctx context.Context, serviceName string, initContainer *compose.InitContainer) error {
	s.logger.Infof("[STUB] Running init container %s for service %s (image: %s)", initContainer.Name, serviceName, initContainer.Image)
	
	// Simulate init container execution
	time.Sleep(300 * time.Millisecond)
	
	s.logger.Infof("[STUB] Init container %s completed successfully", initContainer.Name)
	return nil
}

func (s *StubManager) RunPostContainer(ctx context.Context, serviceName string, postContainer *compose.PostContainer) error {
	s.logger.Infof("[STUB] Running post container %s for service %s (image: %s)", postContainer.Name, serviceName, postContainer.Image)
	
	// Wait for specified duration if configured
	if postContainer.WaitFor != "" {
		if duration, err := time.ParseDuration(postContainer.WaitFor); err == nil {
			s.logger.Infof("[STUB] Waiting %s before running post container", postContainer.WaitFor)
			time.Sleep(duration)
		}
	}
	
	// Simulate post container execution
	time.Sleep(200 * time.Millisecond)
	
	s.logger.Infof("[STUB] Post container %s completed successfully", postContainer.Name)
	return nil
}

func (s *StubManager) Close() error {
	s.logger.Info("[STUB] Closing container manager")
	return nil
}