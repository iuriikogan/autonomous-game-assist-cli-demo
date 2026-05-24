package tool

import (
	"context"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// SandboxArgs represents the input parameters for the Sandbox execution tool.
type SandboxArgs struct {
	Code     string `json:"code" jsonschema:"The full script or code content to execute and validate"`
	Language string `json:"language" jsonschema:"The language of the script (python/cpp)"`
}

// SandboxResponse represents the output of the script execution.
type SandboxResponse struct {
	Success  bool   `json:"success"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// RunSandbox runs the provided script in a safe subprocess, captures execution outputs,
// and maps exit status codes. Safe because runner runs in sandboxed environments.
func RunSandbox(ctx context.Context, args SandboxArgs) (SandboxResponse, error) {
	if args.Code == "" {
		return SandboxResponse{}, fmt.Errorf("code content cannot be empty")
	}

	// Create a temporary directory inside the workspace for execution
	tempDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return SandboxResponse{}, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	lang := strings.ToLower(strings.TrimSpace(args.Language))
	switch lang {
	case "python", "py", "python3":
		return runPython(ctx, tempDir, args.Code)
	case "cpp", "c++", "cplusplus":
		return runCpp(ctx, tempDir, args.Code)
	default:
		return SandboxResponse{}, fmt.Errorf("unsupported language %q", args.Language)
	}
}

func runPython(ctx context.Context, dir, code string) (SandboxResponse, error) {
	filePath := filepath.Join(dir, "script.py")
	if err := os.WriteFile(filePath, []byte(code), 0600); err != nil {
		return SandboxResponse{}, fmt.Errorf("failed to write Python script to disk: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "python3", filePath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return SandboxResponse{}, fmt.Errorf("subprocess execution error: %w", err)
		}
	}

	return SandboxResponse{
		Success:  err == nil,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

func runCpp(ctx context.Context, dir, code string) (SandboxResponse, error) {
	srcPath := filepath.Join(dir, "main.cpp")
	binPath := filepath.Join(dir, "app.out")
	if err := os.WriteFile(srcPath, []byte(code), 0600); err != nil {
		return SandboxResponse{}, fmt.Errorf("failed to write C++ source to disk: %w", err)
	}

	// Compile C++ source
	var compStdout, compStderr bytes.Buffer
	compCmd := exec.CommandContext(ctx, "g++", "-std=c++17", srcPath, "-o", binPath)
	compCmd.Stdout = &compStdout
	compCmd.Stderr = &compStderr

	if err := compCmd.Run(); err != nil {
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		return SandboxResponse{
			Success:  false,
			Stdout:   compStdout.String(),
			Stderr:   fmt.Sprintf("Compilation failed:\n%s", compStderr.String()),
			ExitCode: exitCode,
		}, nil
	}

	// Run C++ binary
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return SandboxResponse{}, fmt.Errorf("C++ subprocess execution error: %w", err)
		}
	}

	return SandboxResponse{
		Success:  err == nil,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// NewSandboxTool bootstraps the ADK function tool wrapping the subprocess sandbox logic.
func NewSandboxTool() (tool.Tool, error) {
	cfg := functiontool.Config{
		Name:        "sandbox_tool",
		Description: "Executes python or C++ scripts in a sandboxed local subprocess to dry-run and validate script behaviors.",
	}

	handler := func(tctx tool.Context, args SandboxArgs) (SandboxResponse, error) {
		return RunSandbox(tctx, args)
	}

	return functiontool.New(cfg, handler)
}
