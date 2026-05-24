package vertex

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GenAIModels represents the subset of the Google Gen AI SDK Models service
// that we require, enabling unit testing via dependency injection.
type GenAIModels interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	EmbedContent(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
}

// Client defines the interface for interacting with Vertex AI Gemini models.
type Client interface {
	GenerateText(ctx context.Context, prompt string, usePro bool) (string, error)
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type vertexClient struct {
	models   GenAIModels
	project  string
	location string
}

// NewClient creates a new Vertex AI wrapper client.
func NewClient(ctx context.Context, project, location string) (Client, error) {
	if project == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if location == "" {
		location = "us-central1"
	}

	// Initialize the Google Gen AI Client configured for Vertex AI
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &vertexClient{
		models:   client.Models,
		project:  project,
		location: location,
	}, nil
}

// GenerateText sends a text prompt to either gemini-3.1-pro or gemini-3.1-flash.
func (vc *vertexClient) GenerateText(ctx context.Context, prompt string, usePro bool) (string, error) {
	model := "gemini-3.1-flash"
	if usePro {
		model = "gemini-3.1-pro"
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}

	// Make the GenerateContent call using the mockable models interface
	resp, err := vc.models.GenerateContent(ctx, model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content using %s: %w", model, err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned by model %s", model)
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("empty content or parts returned by model %s", model)
	}

	// Join all text parts
	var result string
	for _, part := range candidate.Content.Parts {
		if part != nil && part.Text != "" {
			result += part.Text
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text parts found in candidate content")
	}

	return result, nil
}

// EmbedText generates embedding vectors for a given text.
func (vc *vertexClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	model := "text-embedding-004"
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}

	resp, err := vc.models.EmbedContent(ctx, model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to embed content using %s: %w", model, err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned by model %s", model)
	}

	embedding := resp.Embeddings[0]
	if embedding == nil || len(embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding values returned by model %s", model)
	}

	return embedding.Values, nil
}
