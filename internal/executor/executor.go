package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose/pkg/compose"
	"github.com/neomody77/fake-compose/pkg/container"
	"github.com/neomody77/fake-compose/pkg/lifecycle"
)

type Executor struct {
	projectName       string
	logger           *logrus.Logger
	containerManager *container.Manager
	lifecycleManager *lifecycle.Manager
	runningServices  map[string]string
	mu               sync.RWMutex
}

func New(logger *logrus.Logger, projectName string) (*Executor, error) {
	containerManager, err := container.NewManager(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create container manager: %w", err)
	}

	return &Executor{
		projectName:      projectName,
		logger:          logger,
		containerManager: containerManager,
		lifecycleManager: lifecycle.NewManager(logger),
		runningServices:  make(map[string]string),
	}, nil
}

func (e *Executor) Up(ctx context.Context, compose *compose.ComposeFile) error {
	e.logger.Info("Starting services...")

	ordered := e.orderServices(compose.Services)

	for _, serviceName := range ordered {
		service := compose.Services[serviceName]
		
		if err := e.startService(ctx, serviceName, service); err != nil {
			e.logger.Errorf("Failed to start service %s: %v", serviceName, err)
			
			e.logger.Info("Rolling back started services...")
			e.rollback(context.Background(), compose)
			
			return fmt.Errorf("failed to start service %s: %w", serviceName, err)
		}
	}

	return nil
}

func (e *Executor) Down(ctx context.Context, compose *compose.ComposeFile) error {
	e.logger.Info("Stopping services...")

	ordered := e.orderServices(compose.Services)
	
	for i := len(ordered) - 1; i >= 0; i-- {
		serviceName := ordered[i]
		service := compose.Services[serviceName]
		
		if err := e.stopService(ctx, serviceName, service); err != nil {
			e.logger.Errorf("Failed to stop service %s: %v", serviceName, err)
		}
	}

	return nil
}

func (e *Executor) startService(ctx context.Context, serviceName string, service *compose.Service) error {
	e.logger.Infof("Starting service: %s", serviceName)

	if err := e.lifecycleManager.StartService(ctx, serviceName, service); err != nil {
		return err
	}

	for _, init := range service.InitContainers {
		if err := e.containerManager.RunInitContainer(ctx, serviceName, &init); err != nil {
			return fmt.Errorf("init container %s failed: %w", init.Name, err)
		}
	}

	containerID, err := e.containerManager.CreateService(ctx, serviceName, service)
	if err != nil {
		return fmt.Errorf("failed to create service container: %w", err)
	}

	if err := e.containerManager.StartContainer(ctx, containerID); err != nil {
		e.containerManager.RemoveContainer(ctx, containerID)
		return fmt.Errorf("failed to start service container: %w", err)
	}

	e.mu.Lock()
	e.runningServices[serviceName] = containerID
	e.mu.Unlock()

	for _, post := range service.PostContainers {
		if post.OnSuccess {
			if err := e.containerManager.RunPostContainer(ctx, serviceName, &post); err != nil {
				e.logger.Warnf("Post container %s failed: %v", post.Name, err)
			}
		}
	}

	e.logger.Infof("Service %s started successfully", serviceName)
	return nil
}

func (e *Executor) stopService(ctx context.Context, serviceName string, service *compose.Service) error {
	e.logger.Infof("Stopping service: %s", serviceName)

	e.mu.RLock()
	containerID, exists := e.runningServices[serviceName]
	e.mu.RUnlock()

	if !exists {
		e.logger.Warnf("Service %s not found in running services", serviceName)
		return nil
	}

	if err := e.lifecycleManager.StopService(ctx, serviceName, service); err != nil {
		e.logger.Warnf("Lifecycle stop failed for %s: %v", serviceName, err)
	}

	if err := e.containerManager.StopContainer(ctx, containerID, 30); err != nil {
		e.logger.Warnf("Failed to stop container for %s: %v", serviceName, err)
	}

	if err := e.containerManager.RemoveContainer(ctx, containerID); err != nil {
		e.logger.Warnf("Failed to remove container for %s: %v", serviceName, err)
	}

	for _, post := range service.PostContainers {
		if post.OnFailure {
			if err := e.containerManager.RunPostContainer(ctx, serviceName, &post); err != nil {
				e.logger.Warnf("Post container %s failed: %v", post.Name, err)
			}
		}
	}

	e.mu.Lock()
	delete(e.runningServices, serviceName)
	e.mu.Unlock()

	e.logger.Infof("Service %s stopped", serviceName)
	return nil
}

func (e *Executor) rollback(ctx context.Context, compose *compose.ComposeFile) {
	e.mu.RLock()
	services := make(map[string]string)
	for k, v := range e.runningServices {
		services[k] = v
	}
	e.mu.RUnlock()

	for serviceName, containerID := range services {
		service := compose.Services[serviceName]
		e.logger.Infof("Rolling back service %s", serviceName)
		
		if err := e.containerManager.StopContainer(ctx, containerID, 10); err != nil {
			e.logger.Warnf("Failed to stop container during rollback: %v", err)
		}
		
		if err := e.containerManager.RemoveContainer(ctx, containerID); err != nil {
			e.logger.Warnf("Failed to remove container during rollback: %v", err)
		}
		
		if service != nil {
			e.lifecycleManager.StopService(ctx, serviceName, service)
		}
	}
}

func (e *Executor) orderServices(services map[string]*compose.Service) []string {
	visited := make(map[string]bool)
	result := make([]string, 0, len(services))
	
	var visit func(string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		
		if service, exists := services[name]; exists {
			for dep := range service.DependsOn {
				visit(dep)
			}
		}
		
		result = append(result, name)
	}
	
	for name := range services {
		visit(name)
	}
	
	return result
}

func (e *Executor) Close() error {
	return e.containerManager.Close()
}