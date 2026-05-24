package vertex

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/genai"
)

// mockGenAIModels is a mock implementation of the GenAIModels interface.
type mockGenAIModels struct {
	GenerateContentFunc func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	EmbedContentFunc    func(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
}

func (m *mockGenAIModels) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, model, contents, config)
	}
	return nil, errors.New("GenerateContentFunc not implemented")
}

func (m *mockGenAIModels) EmbedContent(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	if m.EmbedContentFunc != nil {
		return m.EmbedContentFunc(ctx, model, contents, config)
	}
	return nil, errors.New("EmbedContentFunc not implemented")
}

func TestGenerateText_Success_Flash(t *testing.T) {
	ctx := context.Background()
	expectedPrompt := "Hello Flash"
	expectedResponseText := "Hi! I am Flash."

	mockModels := &mockGenAIModels{
		GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			if model != "gemini-3.1-flash" {
				t.Errorf("expected model gemini-3.1-flash, got %s", model)
			}
			if len(contents) != 1 || len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != expectedPrompt {
				t.Errorf("unexpected contents: %v", contents)
			}
			return &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: expectedResponseText},
							},
						},
					},
				},
			}, nil
		},
	}

	client := &vertexClient{
		models:   mockModels,
		project:  "test-project",
		location: "us-central1",
	}

	resp, err := client.GenerateText(ctx, expectedPrompt, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != expectedResponseText {
		t.Errorf("expected response %q, got %q", expectedResponseText, resp)
	}
}

func TestGenerateText_Success_Pro(t *testing.T) {
	ctx := context.Background()
	expectedPrompt := "Hello Pro"
	expectedResponseText := "Hi! I am Pro."

	mockModels := &mockGenAIModels{
		GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			if model != "gemini-3.1-pro" {
				t.Errorf("expected model gemini-3.1-pro, got %s", model)
			}
			return &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: expectedResponseText},
							},
						},
					},
				},
			}, nil
		},
	}

	client := &vertexClient{
		models:   mockModels,
		project:  "test-project",
		location: "us-central1",
	}

	resp, err := client.GenerateText(ctx, expectedPrompt, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != expectedResponseText {
		t.Errorf("expected response %q, got %q", expectedResponseText, resp)
	}
}

func TestGenerateText_Error_API(t *testing.T) {
	ctx := context.Background()
	mockModels := &mockGenAIModels{
		GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			return nil, errors.New("api rate limit exceeded")
		},
	}

	client := &vertexClient{
		models:   mockModels,
		project:  "test-project",
		location: "us-central1",
	}

	_, err := client.GenerateText(ctx, "test prompt", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, err) { // checking wrapping
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestGenerateText_Error_EmptyCandidates(t *testing.T) {
	ctx := context.Background()
	mockModels := &mockGenAIModels{
		GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			return &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			}, nil
		},
	}

	client := &vertexClient{
		models:   mockModels,
		project:  "test-project",
		location: "us-central1",
	}

	_, err := client.GenerateText(ctx, "test prompt", false)
	if err == nil {
		t.Fatal("expected error for empty candidates, got nil")
	}
}

func TestEmbedText_Success(t *testing.T) {
	ctx := context.Background()
	inputText := "Generate audio footsteps"
	expectedVector := []float32{0.1, 0.2, 0.3}

	mockModels := &mockGenAIModels{
		EmbedContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
			if model != "text-embedding-004" {
				t.Errorf("expected model text-embedding-004, got %s", model)
			}
			if len(contents) != 1 || len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != inputText {
				t.Errorf("unexpected contents: %v", contents)
			}
			return &genai.EmbedContentResponse{
				Embeddings: []*genai.ContentEmbedding{
					{
						Values: expectedVector,
					},
				},
			}, nil
		},
	}

	client := &vertexClient{
		models:   mockModels,
		project:  "test-project",
		location: "us-central1",
	}

	resp, err := client.EmbedText(ctx, inputText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp) != len(expectedVector) {
		t.Fatalf("expected vector length %d, got %d", len(expectedVector), len(resp))
	}

	for i, val := range resp {
		if val != expectedVector[i] {
			t.Errorf("expected value at %d to be %f, got %f", i, expectedVector[i], val)
		}
	}
}
