package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/neomody77/fake-compose/pkg/compose"
)

type Executor struct {
	logger     *logrus.Logger
	httpClient *http.Client
}

func NewExecutor(logger *logrus.Logger) *Executor {
	return &Executor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *Executor) ExecuteHooks(ctx context.Context, hooks []compose.Hook) error {
	for _, hook := range hooks {
		if err := e.ExecuteHook(ctx, &hook); err != nil {
			if hook.Retries > 0 {
				for i := 0; i < hook.Retries; i++ {
					e.logger.Warnf("Hook %s failed, retrying (%d/%d): %v", hook.Name, i+1, hook.Retries, err)
					time.Sleep(time.Second * time.Duration(i+1))
					if err = e.ExecuteHook(ctx, &hook); err == nil {
						break
					}
				}
			}
			if err != nil {
				return fmt.Errorf("hook %s failed: %w", hook.Name, err)
			}
		}
	}
	return nil
}

func (e *Executor) ExecuteHook(ctx context.Context, hook *compose.Hook) error {
	e.logger.Infof("Executing hook: %s (type: %s)", hook.Name, hook.Type)

	if hook.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, hook.Timeout)
		defer cancel()
	}

	switch hook.Type {
	case "command":
		return e.executeCommandHook(ctx, hook)
	case "script":
		return e.executeScriptHook(ctx, hook)
	case "http":
		return e.executeHTTPHook(ctx, hook)
	case "exec":
		return e.executeExecHook(ctx, hook)
	default:
		return fmt.Errorf("unknown hook type: %s", hook.Type)
	}
}

func (e *Executor) executeCommandHook(ctx context.Context, hook *compose.Hook) error {
	if len(hook.Command) == 0 {
		return fmt.Errorf("command hook requires command")
	}

	cmd := exec.CommandContext(ctx, hook.Command[0], hook.Command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	e.logger.Debugf("Executing command: %v", hook.Command)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

func (e *Executor) executeScriptHook(ctx context.Context, hook *compose.Hook) error {
	if hook.Script == "" {
		return fmt.Errorf("script hook requires script content")
	}

	tmpfile, err := ioutil.TempFile("", "hook-script-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create temp script file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString("#!/bin/bash\n" + hook.Script); err != nil {
		return fmt.Errorf("failed to write script: %w", err)
	}
	tmpfile.Close()

	if err := os.Chmod(tmpfile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, tmpfile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	e.logger.Debugf("Executing script for hook: %s", hook.Name)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

func (e *Executor) executeHTTPHook(ctx context.Context, hook *compose.Hook) error {
	if hook.HTTP == nil || hook.HTTP.URL == "" {
		return fmt.Errorf("HTTP hook requires URL")
	}

	method := hook.HTTP.Method
	if method == "" {
		method = "GET"
	}

	var body *bytes.Buffer
	if hook.HTTP.Body != "" {
		body = bytes.NewBufferString(hook.HTTP.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, hook.HTTP.URL, body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range hook.HTTP.Headers {
		req.Header.Set(key, value)
	}

	e.logger.Debugf("Making HTTP request: %s %s", method, hook.HTTP.URL)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (e *Executor) executeExecHook(ctx context.Context, hook *compose.Hook) error {
	if hook.Exec == nil || hook.Exec.Container == "" || len(hook.Exec.Command) == 0 {
		return fmt.Errorf("exec hook requires container and command")
	}

	e.logger.Debugf("Executing command in container %s: %v", hook.Exec.Container, hook.Exec.Command)

	return nil
}

type HookResult struct {
	HookName  string
	Success   bool
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Output    string
}

func (e *Executor) ExecuteHooksWithResults(ctx context.Context, hooks []compose.Hook) []HookResult {
	results := make([]HookResult, 0, len(hooks))

	for _, hook := range hooks {
		result := HookResult{
			HookName:  hook.Name,
			StartTime: time.Now(),
		}

		err := e.ExecuteHook(ctx, &hook)
		result.EndTime = time.Now()
		result.Success = err == nil
		result.Error = err

		results = append(results, result)

		if err != nil && hook.Retries == 0 {
			break
		}
	}

	return results
}