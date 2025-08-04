package parser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"github.com/neomody77/fake-compose-extended/pkg/compose"
)

type Parser struct {
	envVars map[string]string
}

func New() *Parser {
	return &Parser{
		envVars: make(map[string]string),
	}
}

func (p *Parser) ParseFile(filename string) (*compose.ComposeFile, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	expanded := p.expandEnvVars(string(data))

	var composeFile compose.ComposeFile
	if err := yaml.Unmarshal([]byte(expanded), &composeFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := p.resolveRelativePaths(&composeFile, filepath.Dir(filename)); err != nil {
		return nil, fmt.Errorf("failed to resolve paths: %w", err)
	}

	if err := p.validateComposeFile(&composeFile); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &composeFile, nil
}

func (p *Parser) expandEnvVars(content string) string {
	return os.Expand(content, func(key string) string {
		if val, ok := p.envVars[key]; ok {
			return val
		}
		return os.Getenv(key)
	})
}

func (p *Parser) resolveRelativePaths(cf *compose.ComposeFile, baseDir string) error {
	for _, service := range cf.Services {
		if service.Build != nil && service.Build.Context != "" {
			if !filepath.IsAbs(service.Build.Context) {
				service.Build.Context = filepath.Join(baseDir, service.Build.Context)
			}
		}

		for i, envFile := range service.EnvFile {
			if !filepath.IsAbs(envFile) {
				service.EnvFile[i] = filepath.Join(baseDir, envFile)
			}
		}
	}

	for _, config := range cf.Configs {
		if config.File != "" && !filepath.IsAbs(config.File) {
			config.File = filepath.Join(baseDir, config.File)
		}
	}

	for _, secret := range cf.Secrets {
		if secret.File != "" && !filepath.IsAbs(secret.File) {
			secret.File = filepath.Join(baseDir, secret.File)
		}
	}

	return nil
}

func (p *Parser) validateComposeFile(cf *compose.ComposeFile) error {
	if cf.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(cf.Services) == 0 {
		return fmt.Errorf("at least one service is required")
	}

	for name, service := range cf.Services {
		if err := p.validateService(name, service); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}
	}

	return nil
}

func (p *Parser) validateService(name string, service *compose.Service) error {
	if service.Image == "" && service.Build == nil {
		return fmt.Errorf("either image or build must be specified")
	}

	for _, initContainer := range service.InitContainers {
		if initContainer.Name == "" {
			return fmt.Errorf("init container name is required")
		}
		if initContainer.Image == "" {
			return fmt.Errorf("init container %s: image is required", initContainer.Name)
		}
	}

	for _, postContainer := range service.PostContainers {
		if postContainer.Name == "" {
			return fmt.Errorf("post container name is required")
		}
		if postContainer.Image == "" {
			return fmt.Errorf("post container %s: image is required", postContainer.Name)
		}
	}

	if service.Hooks != nil {
		if err := p.validateHooks(service.Hooks); err != nil {
			return fmt.Errorf("hooks validation failed: %w", err)
		}
	}

	return nil
}

func (p *Parser) validateHooks(hooks *compose.Hooks) error {
	allHooks := [][]compose.Hook{
		hooks.PreStart,
		hooks.PostStart,
		hooks.PreStop,
		hooks.PostStop,
		hooks.PreBuild,
		hooks.PostBuild,
		hooks.PreDeploy,
		hooks.PostDeploy,
	}

	for _, hookList := range allHooks {
		for _, hook := range hookList {
			if hook.Name == "" {
				return fmt.Errorf("hook name is required")
			}
			if hook.Type == "" {
				return fmt.Errorf("hook %s: type is required", hook.Name)
			}
			switch hook.Type {
			case "command":
				if len(hook.Command) == 0 {
					return fmt.Errorf("hook %s: command is required for command type", hook.Name)
				}
			case "script":
				if hook.Script == "" {
					return fmt.Errorf("hook %s: script is required for script type", hook.Name)
				}
			case "http":
				if hook.HTTP == nil || hook.HTTP.URL == "" {
					return fmt.Errorf("hook %s: http configuration with URL is required for http type", hook.Name)
				}
			case "exec":
				if hook.Exec == nil || hook.Exec.Container == "" || len(hook.Exec.Command) == 0 {
					return fmt.Errorf("hook %s: exec configuration with container and command is required for exec type", hook.Name)
				}
			default:
				return fmt.Errorf("hook %s: invalid type %s", hook.Name, hook.Type)
			}
		}
	}

	return nil
}

func (p *Parser) SetEnvVar(key, value string) {
	p.envVars[key] = value
}

func (p *Parser) LoadEnvFile(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")
			p.envVars[key] = value
		}
	}

	return nil
}