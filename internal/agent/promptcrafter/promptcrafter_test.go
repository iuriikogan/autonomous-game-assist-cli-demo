package promptcrafter

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

func TestPromptCrafter_Instantiation(t *testing.T) {
	mockModel := &mockLLM{NameFunc: "gemini-3.1-flash"}
	agent, err := New(mockModel)
	if err != nil {
		t.Fatalf("failed to instantiate Prompt Crafter: %v", err)
	}

	if agent.Name() != "prompt_crafter" {
		t.Errorf("expected agent name 'prompt_crafter', got %q", agent.Name())
	}

	if agent.Description() == "" {
		t.Error("expected agent description to be non-empty")
	}
}
