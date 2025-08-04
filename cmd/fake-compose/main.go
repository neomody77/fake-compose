package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/neomody77/fake-compose-extended/internal/executor"
	"github.com/neomody77/fake-compose-extended/internal/parser"
	"github.com/neomody77/fake-compose-extended/pkg/compose"
	"gopkg.in/yaml.v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var composeFile string
	var envFile string
	var projectName string
	var verbose bool

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	rootCmd := &cobra.Command{
		Use:   "fake-compose",
		Short: "Docker Compose compatible tool with extended features",
		Long: `fake-compose is a Docker Compose compatible tool that adds support for:
- Init containers that run before the main service
- Post containers that run after service start/stop
- Lifecycle hooks at various stages
- Cloud native integrations`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	rootCmd.PersistentFlags().StringVarP(&composeFile, "file", "f", "docker-compose.yml", "Compose file")
	rootCmd.PersistentFlags().StringVarP(&envFile, "env-file", "", "", "Environment file")
	rootCmd.PersistentFlags().StringVarP(&projectName, "project-name", "p", "", "Project name")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}
	}

	// Up command
	upCmd := &cobra.Command{
		Use:   "up [SERVICE...]",
		Short: "Create and start containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}

			if projectName == "" {
				projectName = "fake-compose"
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigChan
				logger.Info("Received interrupt signal, shutting down...")
				cancel()
			}()

			exec, err := executor.New(logger, projectName)
			if err != nil {
				return fmt.Errorf("failed to create executor: %w", err)
			}
			defer exec.Close()

			if err := exec.Up(ctx, compose); err != nil {
				return fmt.Errorf("failed to start services: %w", err)
			}

			logger.Info("All services started successfully")

			<-ctx.Done()

			logger.Info("Shutting down services...")
			if err := exec.Down(context.Background(), compose); err != nil {
				logger.Errorf("Error during shutdown: %v", err)
			}

			return nil
		},
	}

	// Down command
	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove containers, networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}

			if projectName == "" {
				projectName = "fake-compose"
			}

			exec, err := executor.New(logger, projectName)
			if err != nil {
				return fmt.Errorf("failed to create executor: %w", err)
			}
			defer exec.Close()

			if err := exec.Down(context.Background(), compose); err != nil {
				return fmt.Errorf("failed to stop services: %w", err)
			}

			logger.Info("All services stopped successfully")
			return nil
		},
	}

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Validate and view the Compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}

			output, err := yaml.Marshal(compose)
			if err != nil {
				return fmt.Errorf("failed to marshal compose file: %w", err)
			}
			fmt.Print(string(output))
			return nil
		},
	}

	// Validate command
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}

			logger.Infof("Compose file is valid")
			logger.Infof("Found %d services", len(compose.Services))
			
			for name, service := range compose.Services {
				logger.Infof("Service: %s", name)
				if len(service.InitContainers) > 0 {
					logger.Infof("  - %d init containers", len(service.InitContainers))
				}
				if len(service.PostContainers) > 0 {
					logger.Infof("  - %d post containers", len(service.PostContainers))
				}
				if service.Hooks != nil {
					hookCount := len(service.Hooks.PreStart) + len(service.Hooks.PostStart) +
						len(service.Hooks.PreStop) + len(service.Hooks.PostStop)
					if hookCount > 0 {
						logger.Infof("  - %d hooks configured", hookCount)
					}
				}
			}

			return nil
		},
	}

	// PS command
	psCmd := &cobra.Command{
		Use:   "ps [SERVICE...]",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tIMAGE\tCOMMAND\tSERVICE\tSTATUS\tPORTS")
			
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				status := "Up 2 minutes"
				ports := ""
				if len(service.Ports) > 0 {
					ports = service.Ports[0]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					name+"-1", service.Image, "stub", name, status, ports)
			}
			w.Flush()
			return nil
		},
	}

	// Version command  
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show the Docker Compose version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Docker Compose version v2.23.0")
			fmt.Printf("fake-compose version %s\n", version)
			return nil
		},
	}

	// Add commands
	rootCmd.AddCommand(upCmd, downCmd, configCmd, validateCmd, psCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}

func loadCompose(composeFile, envFile string) (*parser.Parser, *compose.ComposeFile, error) {
	p := parser.New()
	
	if envFile != "" {
		if err := p.LoadEnvFile(envFile); err != nil {
			return nil, nil, fmt.Errorf("failed to load env file: %w", err)
		}
	}

	compose, err := p.ParseFile(composeFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	return p, compose, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}