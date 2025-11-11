package executor

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecutionResult represents command execution result
type ExecutionResult struct {
	ExitCode int
	Error    error
}

// Executor executes shell commands
type Executor struct {
	timeout time.Duration
}

// NewExecutor creates a new command executor
func NewExecutor(defaultTimeoutSec int) *Executor {
	return &Executor{
		timeout: time.Duration(defaultTimeoutSec) * time.Second,
	}
}

// Execute executes a command with optional timeout
func (e *Executor) Execute(ctx context.Context, cmd string, args []string, timeoutSec int) (*ExecutionResult, error) {
	timeout := e.timeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(execCtx, cmd, args...)

	err := command.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Context timeout or other error
			return &ExecutionResult{
				ExitCode: -1,
				Error:    fmt.Errorf("execution failed: %w", err),
			}, nil
		}
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Error:    nil,
	}, nil
}

// ExecuteShell executes a shell command (sh -c)
func (e *Executor) ExecuteShell(ctx context.Context, shellCmd string, timeoutSec int) (*ExecutionResult, error) {
	return e.Execute(ctx, "sh", []string{"-c", shellCmd}, timeoutSec)
}
