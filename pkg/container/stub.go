package container

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose-extended/pkg/compose"
)

type Manager struct {
	logger *logrus.Logger
}

func NewManager(logger *logrus.Logger) (*Manager, error) {
	return &Manager{
		logger: logger,
	}, nil
}

func (m *Manager) CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error) {
	containerID := fmt.Sprintf("%s_container_%d", serviceName, time.Now().Unix())
	m.logger.Infof("[STUB] Creating container %s for service %s (image: %s)", containerID, serviceName, service.Image)
	
	// Simulate container creation time
	time.Sleep(100 * time.Millisecond)
	
	return containerID, nil
}

func (m *Manager) StartContainer(ctx context.Context, containerID string) error {
	m.logger.Infof("[STUB] Starting container %s", containerID)
	
	// Simulate container startup time
	time.Sleep(200 * time.Millisecond)
	
	return nil
}

func (m *Manager) StopContainer(ctx context.Context, containerID string, timeout int) error {
	m.logger.Infof("[STUB] Stopping container %s (timeout: %ds)", containerID, timeout)
	
	// Simulate container stop time
	time.Sleep(100 * time.Millisecond)
	
	return nil
}

func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	m.logger.Infof("[STUB] Removing container %s", containerID)
	
	// Simulate container removal time
	time.Sleep(50 * time.Millisecond)
	
	return nil
}

func (m *Manager) RunInitContainer(ctx context.Context, serviceName string, initContainer *compose.InitContainer) error {
	m.logger.Infof("[STUB] Running init container %s for service %s (image: %s)", initContainer.Name, serviceName, initContainer.Image)
	
	// Simulate init container execution
	time.Sleep(300 * time.Millisecond)
	
	m.logger.Infof("[STUB] Init container %s completed successfully", initContainer.Name)
	return nil
}

func (m *Manager) RunPostContainer(ctx context.Context, serviceName string, postContainer *compose.PostContainer) error {
	m.logger.Infof("[STUB] Running post container %s for service %s (image: %s)", postContainer.Name, serviceName, postContainer.Image)
	
	// Wait for specified duration if configured
	if postContainer.WaitFor != "" {
		if duration, err := time.ParseDuration(postContainer.WaitFor); err == nil {
			m.logger.Infof("[STUB] Waiting %s before running post container", postContainer.WaitFor)
			time.Sleep(duration)
		}
	}
	
	// Simulate post container execution
	time.Sleep(200 * time.Millisecond)
	
	m.logger.Infof("[STUB] Post container %s completed successfully", postContainer.Name)
	return nil
}

func (m *Manager) Close() error {
	m.logger.Info("[STUB] Closing container manager")
	return nil
}