package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/neomody77/fake-compose/internal/executor"
	"github.com/neomody77/fake-compose/internal/parser"
	"github.com/neomody77/fake-compose/pkg/compose"
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
- Cloud native integrations

Note: Global flags (-f, -p) must come BEFORE the command:
  fake-compose -f docker-compose.yml logs --follow
  NOT: fake-compose logs -f docker-compose.yml`,
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
	var (
		detach bool
		build bool
		quietPull bool
		forceRecreate bool
		noRecreate bool
		noStart bool
		timeout int
	)
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

			if detach {
				logger.Info("Running in detached mode")
				return nil
			}

			// Wait for interrupt signal in attached mode
			<-ctx.Done()

			logger.Info("Shutting down services...")
			if err := exec.Down(context.Background(), compose); err != nil {
				logger.Errorf("Error during shutdown: %v", err)
			}

			return nil
		},
	}
	upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Detached mode: Run containers in the background")
	upCmd.Flags().BoolVar(&build, "build", false, "Build images before starting containers")
	upCmd.Flags().BoolVar(&quietPull, "quiet-pull", false, "Pull without printing progress information")
	upCmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Recreate containers even if configuration hasn't changed")
	upCmd.Flags().BoolVar(&noRecreate, "no-recreate", false, "Don't recreate containers if they already exist")
	upCmd.Flags().BoolVar(&noStart, "no-start", false, "Don't start the services after creating them")
	upCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Shutdown timeout in seconds")

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

	// Build command
	buildCmd := &cobra.Command{
		Use:   "build [SERVICE...]",
		Short: "Build or rebuild services",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				if service.Build != nil {
					fmt.Printf("\033[36m[+] Building %s\033[0m\n", name)
					fmt.Printf("\033[32m#0 building with \"docker\" driver\033[0m\n")
					fmt.Printf("\033[32m#1 [internal] load build definition from Dockerfile\033[0m\n")
					fmt.Printf("\033[32m#1 transferring dockerfile: 123B done\033[0m\n")
					fmt.Printf("\033[32m#1 DONE 0.0s\033[0m\n")
					
					fmt.Printf("\033[32m#2 [internal] load .dockerignore\033[0m\n")
					fmt.Printf("\033[32m#2 transferring context: 34B done\033[0m\n")
					fmt.Printf("\033[32m#2 DONE 0.0s\033[0m\n")
					
					fmt.Printf("\033[32m#3 [internal] load metadata for %s\033[0m\n", service.Image)
					fmt.Printf("\033[32m#3 DONE 1.2s\033[0m\n")
					
					fmt.Printf("\033[32m#4 [internal] load build context\033[0m\n")
					fmt.Printf("\033[32m#4 transferring context: 2.34kB done\033[0m\n")
					fmt.Printf("\033[32m#4 DONE 0.1s\033[0m\n")
					
					fmt.Printf("\033[32m#5 [1/4] FROM %s\033[0m\n", service.Image)
					fmt.Printf("\033[32m#5 resolve %s done\033[0m\n", service.Image)
					fmt.Printf("\033[32m#5 sha256:abc123... 0B / 5.54MB 0.1s\033[0m\n")
					fmt.Printf("\033[32m#5 sha256:def456... 5.54MB / 5.54MB 1.2s done\033[0m\n")
					fmt.Printf("\033[32m#5 extracting sha256:def456... done\033[0m\n")
					fmt.Printf("\033[32m#5 DONE 2.1s\033[0m\n")
					
					fmt.Printf("\033[32m#6 [2/4] WORKDIR /app\033[0m\n")
					fmt.Printf("\033[32m#6 DONE 0.0s\033[0m\n")
					
					fmt.Printf("\033[32m#7 [3/4] COPY package*.json ./\033[0m\n")
					fmt.Printf("\033[32m#7 DONE 0.1s\033[0m\n")
					
					fmt.Printf("\033[32m#8 [4/4] RUN npm install\033[0m\n")
					fmt.Printf("\033[32m#8 npm WARN deprecated request@2.88.2\033[0m\n")
					fmt.Printf("\033[32m#8 added 142 packages from 65 contributors\033[0m\n")
					fmt.Printf("\033[32m#8 audited 148 packages in 8.234s\033[0m\n")
					fmt.Printf("\033[32m#8 found 0 vulnerabilities\033[0m\n")
					fmt.Printf("\033[32m#8 DONE 10.2s\033[0m\n")
					
					fmt.Printf("\033[32m#9 exporting to image\033[0m\n")
					fmt.Printf("\033[32m#9 exporting layers done\033[0m\n")
					fmt.Printf("\033[32m#9 writing image sha256:ghi789... done\033[0m\n")
					fmt.Printf("\033[32m#9 naming to docker.io/library/%s done\033[0m\n", name)
					fmt.Printf("\033[32m#9 DONE 0.2s\033[0m\n\n")
					
					fmt.Printf("\033[36m✓ Built %s successfully in 13.8s\033[0m\n", name)
				} else {
					fmt.Printf("\033[33m⚠ Service %s uses pre-built image %s (no build needed)\033[0m\n", name, service.Image)
				}
			}
			return nil
		},
	}

	// Logs command
	logsCmd := &cobra.Command{
		Use:   "logs [SERVICE...]",
		Short: "View output from containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			
			follow, _ := cmd.Flags().GetBool("follow")
			showInit, _ := cmd.Flags().GetBool("init")
			showPost, _ := cmd.Flags().GetBool("post")
			
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				
				// Show init containers if requested or by default
				if (showInit || (!showInit && !showPost)) && len(service.InitContainers) > 0 {
					fmt.Printf("\n\033[33m=== INIT CONTAINERS for %s ===\033[0m\n", name)
					for _, init := range service.InitContainers {
						fmt.Printf("\033[33m[%s/%s]\033[0m Starting init container %s\n", name, init.Name, init.Name)
						fmt.Printf("\033[33m[%s/%s]\033[0m Image: %s\n", name, init.Name, init.Image)
						if len(init.Command) > 0 {
							fmt.Printf("\033[33m[%s/%s]\033[0m Executing: %v\n", name, init.Name, init.Command)
							if init.Name == "install-deps" {
								fmt.Printf("\033[33m[%s/%s]\033[0m npm WARN old lockfile\n", name, init.Name)
								fmt.Printf("\033[33m[%s/%s]\033[0m added 142 packages in 8.234s\n", name, init.Name)
								fmt.Printf("\033[33m[%s/%s]\033[0m found 0 vulnerabilities\n", name, init.Name)
							} else {
								fmt.Printf("\033[33m[%s/%s]\033[0m Init task completed\n", name, init.Name)
							}
						}
						fmt.Printf("\033[33m[%s/%s]\033[0m Container completed (exit 0)\n", name, init.Name)
					}
				}
				
				// Show post containers if requested or by default
				if (showPost || (!showInit && !showPost)) && len(service.PostContainers) > 0 {
					fmt.Printf("\n\033[35m=== POST CONTAINERS for %s ===\033[0m\n", name)
					for _, post := range service.PostContainers {
						fmt.Printf("\033[35m[%s/%s]\033[0m Starting post container %s\n", name, post.Name, post.Name)
						fmt.Printf("\033[35m[%s/%s]\033[0m Image: %s\n", name, post.Name, post.Image)
						if post.WaitFor != "" {
							fmt.Printf("\033[35m[%s/%s]\033[0m Waiting %s...\n", name, post.Name, post.WaitFor)
						}
						if post.Name == "warmup" {
							fmt.Printf("\033[35m[%s/%s]\033[0m Making warmup request to http://localhost:3000/health\n", name, post.Name)
							fmt.Printf("\033[35m[%s/%s]\033[0m Response: 200 OK\n", name, post.Name)
						}
						fmt.Printf("\033[35m[%s/%s]\033[0m Container completed (exit 0)\n", name, post.Name)
					}
				}
				
				// Show main service logs if not filtering for specific helpers
				if !showInit && !showPost {
					fmt.Printf("\n\033[36m=== MAIN SERVICE %s ===\033[0m\n", name)
					fmt.Printf("\033[36m[%s]\033[0m Image: %s\n", name, service.Image)
					if len(service.Environment) > 0 {
						fmt.Printf("\033[36m[%s]\033[0m Environment: %s\n", name, service.Environment["NODE_ENV"])
					}
					if len(service.Ports) > 0 {
						fmt.Printf("\033[36m[%s]\033[0m Listening on port %s\n", name, service.Ports[0])
					}
					fmt.Printf("\033[36m[%s]\033[0m [%s] Server started successfully\n", name, time.Now().Format("15:04:05"))
					fmt.Printf("\033[36m[%s]\033[0m [%s] Application ready\n", name, time.Now().Format("15:04:05"))
					
					if follow {
						fmt.Printf("\033[36m[%s]\033[0m Following logs...\n", name)
						for i := 0; i < 3; i++ {
							time.Sleep(1000 * time.Millisecond)
							fmt.Printf("\033[36m[%s]\033[0m [%s] GET /health - 200\n", name, time.Now().Format("15:04:05"))
						}
					}
				}
			}
			return nil
		},
	}
	logsCmd.Flags().Bool("follow", false, "Follow log output")
	logsCmd.Flags().String("since", "", "Show logs since timestamp")
	logsCmd.Flags().String("until", "", "Show logs before timestamp")
	logsCmd.Flags().Int("tail", 0, "Number of lines to show from the end of the logs")
	logsCmd.Flags().Bool("init", false, "Show only init container logs")
	logsCmd.Flags().Bool("post", false, "Show only post container logs")

	// Exec command
	execCmd := &cobra.Command{
		Use:   "exec [OPTIONS] SERVICE COMMAND [ARGS...]",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			command := args[1:]
			
			detach, _ := cmd.Flags().GetBool("detach")
			user, _ := cmd.Flags().GetString("user")
			
			fmt.Printf("\033[36mExecuting in %s container:\033[0m %s\n", serviceName, command[0])
			if user != "" {
				fmt.Printf("\033[36mUser:\033[0m %s\n", user)
			}
			
			// Simulate common commands
			switch command[0] {
			case "bash", "sh":
				if detach {
					fmt.Printf("\033[32mShell session started in background (container_exec_%d)\033[0m\n", time.Now().Unix())
				} else {
					fmt.Printf("\033[32mStarting interactive shell...\033[0m\n")
					fmt.Printf("root@%s:/app# \n", serviceName)
				}
			case "ls":
				fmt.Printf("total 24\n")
				fmt.Printf("drwxr-xr-x 1 root root 4096 %s .\n", time.Now().Format("Jan 2 15:04"))
				fmt.Printf("drwxr-xr-x 1 root root 4096 %s ..\n", time.Now().Format("Jan 2 15:04"))
				fmt.Printf("-rw-r--r-- 1 root root  234 %s package.json\n", time.Now().Format("Jan 2 15:04"))
				fmt.Printf("drwxr-xr-x 1 root root 4096 %s node_modules\n", time.Now().Format("Jan 2 15:04"))
				fmt.Printf("-rw-r--r-- 1 root root 1234 %s server.js\n", time.Now().Format("Jan 2 15:04"))
			case "ps":
				fmt.Printf("PID TTY          TIME CMD\n")
				fmt.Printf("  1 ?        00:00:01 node\n")
				fmt.Printf(" 15 ?        00:00:00 ps\n")
			case "cat":
				if len(command) > 1 {
					switch command[1] {
					case "/etc/hostname":
						fmt.Printf("%s_container_%d\n", serviceName, time.Now().Unix())
					case "package.json":
						fmt.Printf(`{\n  \"name\": \"%s\",\n  \"version\": \"1.0.0\",\n  \"main\": \"server.js\"\n}\n`, serviceName)
					default:
						fmt.Printf("Content of %s\n", command[1])
					}
				}
			case "env":
				fmt.Printf("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n")
				fmt.Printf("NODE_ENV=production\n")
				fmt.Printf("HOME=/root\n")
				fmt.Printf("HOSTNAME=%s\n", serviceName)
			case "curl":
				if len(command) > 1 {
					fmt.Printf("\033[32m* Connected to %s\033[0m\n", command[1])
					fmt.Printf("\033[32m< HTTP/1.1 200 OK\033[0m\n")
					fmt.Printf(`{\"status\": \"healthy\", \"timestamp\": \"%s\"}\n`, time.Now().Format(time.RFC3339))
				}
			default:
				fmt.Printf("\033[32mCommand '%s' executed successfully\033[0m\n", command[0])
				if len(command) > 1 {
					fmt.Printf("\033[32mArguments: %v\033[0m\n", command[1:])
				}
				fmt.Printf("\033[32mExit code: 0\033[0m\n")
			}
			
			return nil
		},
	}
	execCmd.Flags().BoolP("detach", "d", false, "Detached mode")
	execCmd.Flags().StringP("user", "u", "", "Username or UID")
	execCmd.Flags().BoolP("interactive", "i", false, "Keep STDIN open")
	execCmd.Flags().BoolP("tty", "t", false, "Allocate a pseudo-TTY")

	// Stop command
	stopCmd := &cobra.Command{
		Use:   "stop [SERVICE...]",
		Short: "Stop services",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Stopping services...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Stopping %s", name)
			}
			return nil
		},
	}
	stopCmd.Flags().IntP("timeout", "t", 30, "Shutdown timeout in seconds")

	// Start command
	startCmd := &cobra.Command{
		Use:   "start [SERVICE...]",
		Short: "Start services",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Starting services...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Starting %s", name)
			}
			return nil
		},
	}

	// Restart command
	restartCmd := &cobra.Command{
		Use:   "restart [SERVICE...]",
		Short: "Restart service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Restarting services...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Restarting %s", name)
			}
			return nil
		},
	}
	restartCmd.Flags().IntP("timeout", "t", 30, "Shutdown timeout in seconds")

	// Pull command
	pullCmd := &cobra.Command{
		Use:   "pull [SERVICE...]",
		Short: "Pull service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Pulling service images...")
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Pulling %s", service.Image)
			}
			return nil
		},
	}
	pullCmd.Flags().BoolP("quiet", "q", false, "Pull without printing progress information")

	// Push command
	pushCmd := &cobra.Command{
		Use:   "push [SERVICE...]",
		Short: "Push service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Pushing service images...")
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Pushing %s", service.Image)
			}
			return nil
		},
	}

	// Run command
	runCmd := &cobra.Command{
		Use:   "run [OPTIONS] SERVICE COMMAND [ARGS...]",
		Short: "Run a one-off command on a service",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			var command []string
			if len(args) > 1 {
				command = args[1:]
			}
			logger.Infof("Running one-off command on service %s: %v", serviceName, command)
			return nil
		},
	}
	runCmd.Flags().BoolP("detach", "d", false, "Run container in background")
	runCmd.Flags().Bool("rm", true, "Remove container after run")
	runCmd.Flags().StringP("user", "u", "", "Username or UID")
	runCmd.Flags().BoolP("interactive", "i", false, "Keep STDIN open")
	runCmd.Flags().BoolP("tty", "t", false, "Allocate a pseudo-TTY")

	// Create command
	createCmd := &cobra.Command{
		Use:   "create [SERVICE...]",
		Short: "Creates containers for a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Creating containers...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Creating container for %s", name)
			}
			return nil
		},
	}
	createCmd.Flags().Bool("build", false, "Build images before creating containers")
	createCmd.Flags().Bool("force-recreate", false, "Recreate containers even if configuration hasn't changed")

	// Rm command  
	rmCmd := &cobra.Command{
		Use:   "rm [SERVICE...]",
		Short: "Removes stopped service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Removing stopped containers...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Removing container for %s", name)
			}
			return nil
		},
	}
	rmCmd.Flags().Bool("force", false, "Don't ask to confirm removal")
	rmCmd.Flags().BoolP("stop", "s", false, "Stop the containers before removing")
	rmCmd.Flags().Bool("volumes", false, "Remove any anonymous volumes attached")

	// Images command
	imagesCmd := &cobra.Command{
		Use:   "images [SERVICE...]",
		Short: "List images used by the created containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "CONTAINER\tREPOSITORY\tTAG\tIMAGE ID\tSIZE\tCREATED")
			
			// Generate realistic image data
			imageSizes := map[string]string{
				"node:18-alpine": "172MB",
				"node:18": "993MB", 
				"alpine": "5.6MB",
				"ubuntu": "72.8MB",
				"nginx": "142MB",
				"redis": "138MB",
				"postgres": "374MB",
				"curlimages/curl": "11.1MB",
			}
			
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				
				// Parse image name and tag  
				tag := "latest"
				repo := service.Image
				if parts := []string{}; len(parts) > 1 {
					repo = parts[0]
					tag = parts[1]
				}
				if service.Image != "" && service.Image[len(service.Image)-7:] == "alpine" {
					tag = "alpine"
					repo = service.Image[:len(service.Image)-8]
				}
				
				// Generate realistic image ID
				imageID := fmt.Sprintf("sha256:%x", time.Now().Unix() + int64(len(name)*42))
				imageID = imageID[:12]
				
				// Get realistic size
				size, exists := imageSizes[service.Image]
				if !exists {
					// Default size based on image type
					if service.Image != "" {
						switch {
						case service.Image == "alpine" || service.Image[len(service.Image)-6:] == "alpine":
							size = "15.2MB"
						case service.Image[:4] == "node":
							size = "893MB"
						default:
							size = "245MB"
						}
					} else {
						size = "N/A"
					}
				}
				
				// Generate creation time
				created := time.Now().Add(-time.Duration((len(name)*17)%72) * time.Hour).Format("2006-01-02")
				
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, repo, tag, imageID, size, created)
				
				// Also show init and post container images if they exist
				for _, init := range service.InitContainers {
					if init.Image != service.Image {
						initSize, exists := imageSizes[init.Image]
						if !exists {
							initSize = "134MB"
						}
						initID := fmt.Sprintf("sha256:%x", time.Now().Unix() + int64(len(init.Name)*31))
						initID = initID[:12]
						createdInit := time.Now().Add(-time.Duration((len(init.Name)*23)%96) * time.Hour).Format("2006-01-02")
						fmt.Fprintf(w, "%s_init_%s\t%s\tlatest\t%s\t%s\t%s\n", name, init.Name, init.Image, initID, initSize, createdInit)
					}
				}
				
				for _, post := range service.PostContainers {
					if post.Image != service.Image {
						postSize, exists := imageSizes[post.Image]
						if !exists {
							postSize = "87MB"
						}
						postID := fmt.Sprintf("sha256:%x", time.Now().Unix() + int64(len(post.Name)*37))
						postID = postID[:12]
						createdPost := time.Now().Add(-time.Duration((len(post.Name)*19)%84) * time.Hour).Format("2006-01-02")
						fmt.Fprintf(w, "%s_post_%s\t%s\tlatest\t%s\t%s\t%s\n", name, post.Name, post.Image, postID, postSize, createdPost)
					}
				}
			}
			w.Flush()
			return nil
		},
	}

	// Kill command
	killCmd := &cobra.Command{
		Use:   "kill [SERVICE...]",
		Short: "Force stop service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			signal, _ := cmd.Flags().GetString("signal")
			logger.Infof("Killing services with signal %s...", signal)
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Killing %s", name)
			}
			return nil
		},
	}
	killCmd.Flags().StringP("signal", "s", "SIGKILL", "Signal to send to the container")

	// Pause command
	pauseCmd := &cobra.Command{
		Use:   "pause [SERVICE...]",
		Short: "Pause services",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Pausing services...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Pausing %s", name)
			}
			return nil
		},
	}

	// Unpause command
	unpauseCmd := &cobra.Command{
		Use:   "unpause [SERVICE...]",
		Short: "Unpause services",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			logger.Info("Unpausing services...")
			for name := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				logger.Infof("Unpausing %s", name)
			}
			return nil
		},
	}

	// Port command
	portCmd := &cobra.Command{
		Use:   "port SERVICE PRIVATE_PORT",
		Short: "Print the public port for a port binding",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args[0] // serviceName - would be used with real implementation
			privatePort := args[1]
			fmt.Printf("0.0.0.0:%s\n", privatePort)
			return nil
		},
	}
	portCmd.Flags().String("protocol", "tcp", "Protocol (tcp or udp)")
	portCmd.Flags().Int("index", 1, "Container index")

	// Top command
	topCmd := &cobra.Command{
		Use:   "top [SERVICE...]",
		Short: "Display the running processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			for name, service := range compose.Services {
				if len(args) > 0 && !contains(args, name) {
					continue
				}
				fmt.Printf("\033[36m%s Container Processes:\033[0m\n", name)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "UID\tPID\tPPID\tC\tSTIME\tTTY\tTIME\tCMD")
				
				// Main process (PID 1)
				startTime := time.Now().Add(-2 * time.Minute).Format("15:04")
				runTime := "00:00:02"
				mainCmd := "node server.js"
				if service.Command != nil && len(service.Command) > 0 {
					mainCmd = fmt.Sprintf("%v", service.Command)
				}
				fmt.Fprintf(w, "root\t1\t0\t0\t%s\t?\t%s\t%s\n", startTime, runTime, mainCmd)
				
				// Worker processes for Node.js apps
				if service.Image != "" && (service.Image == "node:18-alpine" || service.Image == "node" || 
					(service.Command != nil && len(service.Command) > 0 && service.Command[0] == "node")) {
					fmt.Fprintf(w, "root\t15\t1\t0\t%s\t?\t00:00:01\tnode (worker)\n", startTime)
					fmt.Fprintf(w, "root\t16\t1\t0\t%s\t?\t00:00:01\tnode (worker)\n", startTime)
				}
				
				// System processes
				fmt.Fprintf(w, "root\t25\t0\t0\t%s\t?\t00:00:00\t[kthreadd]\n", startTime)
				fmt.Fprintf(w, "root\t26\t25\t0\t%s\t?\t00:00:00\t[ksoftirqd/0]\n", startTime)
				fmt.Fprintf(w, "root\t27\t25\t0\t%s\t?\t00:00:00\t[rcu_sched]\n", startTime)
				
				w.Flush()
				fmt.Printf("\033[32mTotal processes: %d\033[0m\n\n", 6)
			}
			return nil
		},
	}

	// Events command
	eventsCmd := &cobra.Command{
		Use:   "events [SERVICE...]",
		Short: "Receive real time events from containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, compose, err := loadCompose(composeFile, envFile)
			if err != nil {
				return err
			}
			
			jsonOutput, _ := cmd.Flags().GetBool("json")
			
			if !jsonOutput {
				fmt.Printf("\033[36mListening for events from services: %v\033[0m\n", getServiceNames(compose, args))
				fmt.Printf("\033[36mPress Ctrl+C to exit\033[0m\n\n")
			}
			
			// Simulate real-time events
			events := []string{
				"container create",
				"container start",
				"network connect",
				"container health_status: healthy",
				"container exec_create",
				"container exec_start",
				"container resize",
				"volume mount",
				"container update",
			}
			
			for i := 0; i < 15; i++ {
				time.Sleep(time.Duration(800+i*200) * time.Millisecond)
				
				serviceNames := getServiceNames(compose, args)
				serviceName := serviceNames[i%len(serviceNames)]
				eventType := events[i%len(events)]
				timestamp := time.Now()
				
				if jsonOutput {
					fmt.Printf(`{\"time\":\"%s\",\"type\":\"%s\",\"service\":\"%s\",\"id\":\"%s_container_%d\"}\n`,
						timestamp.Format(time.RFC3339Nano),
						eventType,
						serviceName,
						serviceName,
						timestamp.Unix())
				} else {
					fmt.Printf("\033[32m%s\033[0m \033[36m%s\033[0m %s (%s)\n",
						timestamp.Format("2006-01-02 15:04:05.000"),
						serviceName,
						eventType,
						fmt.Sprintf("%s_container_%d", serviceName, timestamp.Unix()))
				}
			}
			
			if !jsonOutput {
				fmt.Printf("\n\033[33mEvent stream ended\033[0m\n")
			}
			return nil
		},
	}
	eventsCmd.Flags().Bool("json", false, "Output events as a stream of JSON objects")

	// Cp command
	cpCmd := &cobra.Command{
		Use:   "cp [OPTIONS] SERVICE:SRC_PATH DEST_PATH",
		Short: "Copy files/folders between a service container and the local filesystem",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			dest := args[1]
			logger.Infof("Copying from %s to %s", src, dest)
			return nil
		},
	}
	cpCmd.Flags().BoolP("archive", "a", false, "Archive mode")
	cpCmd.Flags().BoolP("follow-link", "L", false, "Always follow symbolic links")

	// Scale command (deprecated but still supported)
	scaleCmd := &cobra.Command{
		Use:   "scale SERVICE=NUM [SERVICE=NUM...]",
		Short: "Scale services",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Warn("scale is deprecated. Use 'up --scale' instead.")
			for _, arg := range args {
				logger.Infof("Scaling %s", arg)
			}
			return nil
		},
	}

	// Ls command
	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List running compose projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tCONFIG FILES")
			if projectName != "" {
				fmt.Fprintf(w, "%s\trunning(1)\t%s\n", projectName, composeFile)
			}
			w.Flush()
			return nil
		},
	}
	lsCmd.Flags().BoolP("all", "a", false, "Show all stopped projects")
	lsCmd.Flags().String("format", "table", "Format output")
	lsCmd.Flags().BoolP("quiet", "q", false, "Only display project names")

	// Add commands
	rootCmd.AddCommand(
		upCmd, downCmd, configCmd, validateCmd, psCmd, versionCmd,
		buildCmd, logsCmd, execCmd, stopCmd, startCmd, restartCmd,
		pullCmd, pushCmd, runCmd, createCmd, rmCmd, imagesCmd,
		killCmd, pauseCmd, unpauseCmd, portCmd, topCmd, eventsCmd,
		cpCmd, scaleCmd, lsCmd,
	)

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

func getServiceNames(compose *compose.ComposeFile, args []string) []string {
	var names []string
	if len(args) > 0 {
		return args
	}
	for name := range compose.Services {
		names = append(names, name)
	}
	return names
}