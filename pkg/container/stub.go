package container

import (
	"context"
	"fmt"
	
	"github.com/sirupsen/logrus"
	"github.com/fake-compose/fake-compose/pkg/compose"
)

type Manager struct {
	logger *logrus.Logger
}

func NewManager(logger *logrus.Logger) (*Manager, error) {
	return &Manager{
		logger: logger,
	}, nil
}

func (m *Manager) RunInitContainer(ctx context.Context, serviceName string, init *compose.InitContainer) error {
	m.logger.Infof("[STUB] Would run init container: %s for service %s", init.Name, serviceName)
	return nil
}

func (m *Manager) RunPostContainer(ctx context.Context, serviceName string, post *compose.PostContainer) error {
	m.logger.Infof("[STUB] Would run post container: %s for service %s", post.Name, serviceName)
	return nil
}

func (m *Manager) CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error) {
	m.logger.Infof("[STUB] Would create service container: %s", serviceName)
	return fmt.Sprintf("stub-container-%s", serviceName), nil
}

func (m *Manager) StartContainer(ctx context.Context, containerID string) error {
	m.logger.Infof("[STUB] Would start container: %s", containerID)
	return nil
}

func (m *Manager) StopContainer(ctx context.Context, containerID string, timeout int) error {
	m.logger.Infof("[STUB] Would stop container: %s with timeout %d", containerID, timeout)
	return nil
}

func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	m.logger.Infof("[STUB] Would remove container: %s", containerID)
	return nil
}

func (m *Manager) Close() error {
	return nil
}