package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose/pkg/compose"
)

// DockerManager implements the Manager interface using the Docker API
type DockerManager struct {
	client *client.Client
	logger *logrus.Logger
}

// NewDockerManager creates a new Docker-based container manager
func NewDockerManager(logger *logrus.Logger) (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test connection to Docker daemon
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	logger.Info("Successfully connected to Docker daemon")

	return &DockerManager{
		client: cli,
		logger: logger,
	}, nil
}

// CreateService creates and configures a container for a service
func (dm *DockerManager) CreateService(ctx context.Context, serviceName string, service *compose.Service) (string, error) {
	dm.logger.Infof("Creating container for service: %s", serviceName)

	// Pull image if needed
	if err := dm.ensureImage(ctx, service.Image); err != nil {
		return "", fmt.Errorf("failed to ensure image %s: %w", service.Image, err)
	}

	// Prepare container configuration
	config := &container.Config{
		Image: service.Image,
		Env:   dm.prepareEnv(service.Environment),
		Cmd:   service.Command,
	}

	// Configure exposed ports
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)
	
	for _, portMapping := range service.Ports {
		parts := strings.Split(portMapping, ":")
		if len(parts) == 2 {
			containerPort := nat.Port(parts[1] + "/tcp")
			exposedPorts[containerPort] = struct{}{}
			portBindings[containerPort] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: parts[0],
				},
			}
		}
	}
	config.ExposedPorts = exposedPorts

	// Host configuration
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		RestartPolicy: container.RestartPolicy{
			Name: service.Restart,
		},
	}

	// Configure volumes
	for _, volume := range service.Volumes {
		if hostConfig.Binds == nil {
			hostConfig.Binds = make([]string, 0)
		}
		hostConfig.Binds = append(hostConfig.Binds, volume)
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{}

	containerName := fmt.Sprintf("%s_1", serviceName)
	
	// Create the container
	resp, err := dm.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	dm.logger.Infof("Created container %s with ID: %s", containerName, resp.ID[:12])
	return resp.ID, nil
}

// StartContainer starts a container
func (dm *DockerManager) StartContainer(ctx context.Context, containerID string) error {
	dm.logger.Infof("Starting container: %s", containerID[:12])

	err := dm.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	dm.logger.Infof("Container %s started successfully", containerID[:12])
	return nil
}

// StopContainer stops a container with timeout
func (dm *DockerManager) StopContainer(ctx context.Context, containerID string, timeoutSecs int) error {
	dm.logger.Infof("Stopping container: %s", containerID[:12])

	timeout := time.Duration(timeoutSecs) * time.Second
	err := dm.client.ContainerStop(ctx, containerID, &timeout)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	dm.logger.Infof("Container %s stopped successfully", containerID[:12])
	return nil
}

// RemoveContainer removes a container
func (dm *DockerManager) RemoveContainer(ctx context.Context, containerID string) error {
	dm.logger.Infof("Removing container: %s", containerID[:12])

	err := dm.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	dm.logger.Infof("Container %s removed successfully", containerID[:12])
	return nil
}

// RunInitContainer runs an init container and waits for completion
func (dm *DockerManager) RunInitContainer(ctx context.Context, serviceName string, initContainer *compose.InitContainer) error {
	dm.logger.Infof("Running init container: %s for service %s", initContainer.Name, serviceName)

	// Ensure image exists
	if err := dm.ensureImage(ctx, initContainer.Image); err != nil {
		return fmt.Errorf("failed to ensure init container image %s: %w", initContainer.Image, err)
	}

	// Container configuration
	config := &container.Config{
		Image: initContainer.Image,
		Cmd:   initContainer.Command,
		Env:   dm.prepareEnv(initContainer.Environment),
	}

	// Host configuration
	hostConfig := &container.HostConfig{}

	// Configure volumes
	for _, volume := range initContainer.Volumes {
		if hostConfig.Binds == nil {
			hostConfig.Binds = make([]string, 0)
		}
		hostConfig.Binds = append(hostConfig.Binds, volume)
	}

	// Create and run the init container
	containerName := fmt.Sprintf("%s_init_%s_%d", serviceName, initContainer.Name, time.Now().Unix())
	
	resp, err := dm.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create init container: %w", err)
	}

	// Start the container
	if err := dm.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return fmt.Errorf("failed to start init container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dm.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
			return fmt.Errorf("error waiting for init container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get logs for debugging
			logs, _ := dm.getContainerLogs(ctx, resp.ID)
			dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
			return fmt.Errorf("init container exited with code %d: %s", status.StatusCode, logs)
		}
	}

	// Clean up the init container
	dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
	
	dm.logger.Infof("Init container %s completed successfully", initContainer.Name)
	return nil
}

// RunPostContainer runs a post container and waits for completion
func (dm *DockerManager) RunPostContainer(ctx context.Context, serviceName string, postContainer *compose.PostContainer) error {
	dm.logger.Infof("Running post container: %s for service %s", postContainer.Name, serviceName)

	// Wait for specified duration if configured
	if postContainer.WaitFor != "" {
		if duration, err := time.ParseDuration(postContainer.WaitFor); err == nil {
			dm.logger.Infof("Waiting %s before running post container", postContainer.WaitFor)
			time.Sleep(duration)
		}
	}

	// Ensure image exists
	if err := dm.ensureImage(ctx, postContainer.Image); err != nil {
		return fmt.Errorf("failed to ensure post container image %s: %w", postContainer.Image, err)
	}

	// Container configuration
	config := &container.Config{
		Image: postContainer.Image,
		Cmd:   postContainer.Command,
		Env:   dm.prepareEnv(postContainer.Environment),
	}

	// Host configuration
	hostConfig := &container.HostConfig{}

	// Configure volumes
	for _, volume := range postContainer.Volumes {
		if hostConfig.Binds == nil {
			hostConfig.Binds = make([]string, 0)
		}
		hostConfig.Binds = append(hostConfig.Binds, volume)
	}

	// Create and run the post container
	containerName := fmt.Sprintf("%s_post_%s_%d", serviceName, postContainer.Name, time.Now().Unix())
	
	resp, err := dm.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create post container: %w", err)
	}

	// Start the container
	if err := dm.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return fmt.Errorf("failed to start post container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dm.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
			return fmt.Errorf("error waiting for post container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get logs for debugging
			logs, _ := dm.getContainerLogs(ctx, resp.ID)
			dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
			return fmt.Errorf("post container exited with code %d: %s", status.StatusCode, logs)
		}
	}

	// Clean up the post container
	dm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
	
	dm.logger.Infof("Post container %s completed successfully", postContainer.Name)
	return nil
}

// Close closes the Docker client
func (dm *DockerManager) Close() error {
	dm.logger.Info("Closing Docker client connection")
	return dm.client.Close()
}

// Helper methods

func (dm *DockerManager) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists locally
	images, err := dm.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				return nil // Image exists
			}
		}
	}

	// Pull the image
	dm.logger.Infof("Pulling image: %s", imageName)
	reader, err := dm.client.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Copy pull output to stdout (shows pull progress)
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return fmt.Errorf("error reading pull output: %w", err)
	}

	return nil
}

func (dm *DockerManager) prepareEnv(envMap map[string]string) []string {
	var env []string
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func (dm *DockerManager) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	reader, err := dm.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(logs), nil
}