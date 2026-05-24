package validation

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/model"
)

type mockLLM struct {
	NameFunc            string
	GenerateContentFunc func(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error]
}

func (m *mockLLM) Name() string {
	if m.NameFunc != "" {
		return m.NameFunc
	}
	return "mock-gemini"
}

func (m *mockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, req, stream)
	}
	return func(yield func(*model.LLMResponse, error) bool) {}
}

type mockTool struct {
	NameFunc        string
	DescriptionFunc string
}

func (m *mockTool) Name() string        { return m.NameFunc }
func (m *mockTool) Description() string { return m.DescriptionFunc }
func (m *mockTool) IsLongRunning() bool { return false }

func TestValidationAgent_Instantiation(t *testing.T) {
	mockModel := &mockLLM{NameFunc: "gemini-3.1-pro"}
	mockSandboxTool := &mockTool{NameFunc: "sandbox_tool", DescriptionFunc: "Mock sandbox"}

	agent, err := New(mockModel, mockSandboxTool)
	if err != nil {
		t.Fatalf("failed to instantiate Validation Agent: %v", err)
	}

	if agent.Name() != "validation_agent" {
		t.Errorf("expected agent name 'validation_agent', got %q", agent.Name())
	}

	if agent.Description() == "" {
		t.Error("expected agent description to be non-empty")
	}
}
