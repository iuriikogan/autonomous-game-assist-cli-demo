package tool

import (
	"context"
	"strings"
	"testing"
)

func TestSandboxTool_Python_Success(t *testing.T) {
	ctx := context.Background()
	code := "print('Hello from python sandbox!')"
	
	resp, err := RunSandbox(ctx, SandboxArgs{
		Code:     code,
		Language: "python",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got failure. Stderr: %s", resp.Stderr)
	}

	trimmedStdout := strings.TrimSpace(resp.Stdout)
	if trimmedStdout != "Hello from python sandbox!" {
		t.Errorf("expected stdout 'Hello from python sandbox!', got %q", trimmedStdout)
	}

	if resp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", resp.ExitCode)
	}
}

func TestSandboxTool_Python_Failure(t *testing.T) {
	ctx := context.Background()
	code := "raise ValueError('something went wrong')"

	resp, err := RunSandbox(ctx, SandboxArgs{
		Code:     code,
		Language: "python",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Success {
		t.Error("expected failure, got success")
	}

	if !strings.Contains(resp.Stderr, "ValueError: something went wrong") {
		t.Errorf("expected stderr to contain 'ValueError', got %q", resp.Stderr)
	}

	if resp.ExitCode == 0 {
		t.Error("expected non-zero exit code, got 0")
	}
}

func TestSandboxTool_Cpp_Success(t *testing.T) {
	ctx := context.Background()
	code := `#include <iostream>
int main() {
    std::cout << "Hello from cpp sandbox!" << std::endl;
    return 0;
}`

	resp, err := RunSandbox(ctx, SandboxArgs{
		Code:     code,
		Language: "cpp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got failure. Stderr: %s", resp.Stderr)
	}

	trimmedStdout := strings.TrimSpace(resp.Stdout)
	if trimmedStdout != "Hello from cpp sandbox!" {
		t.Errorf("expected stdout 'Hello from cpp sandbox!', got %q", trimmedStdout)
	}

	if resp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", resp.ExitCode)
	}
}

func TestSandboxTool_UnsupportedLanguage(t *testing.T) {
	ctx := context.Background()
	_, err := RunSandbox(ctx, SandboxArgs{
		Code:     "some code",
		Language: "rust",
	})
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}
