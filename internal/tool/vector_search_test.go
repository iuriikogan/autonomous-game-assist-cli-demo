package tool

import (
	"context"
	"errors"
	"testing"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
)

// mockVertexClient is a mock implementation of vertex.Client
type mockVertexClient struct {
	GenerateTextFunc func(ctx context.Context, prompt string, usePro bool) (string, error)
	EmbedTextFunc    func(ctx context.Context, text string) ([]float32, error)
}

func (m *mockVertexClient) GenerateText(ctx context.Context, prompt string, usePro bool) (string, error) {
	if m.GenerateTextFunc != nil {
		return m.GenerateTextFunc(ctx, prompt, usePro)
	}
	return "", errors.New("GenerateText not implemented")
}

func (m *mockVertexClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedTextFunc != nil {
		return m.EmbedTextFunc(ctx, text)
	}
	return nil, errors.New("EmbedText not implemented")
}

// mockVectorClient is a mock implementation of vector.Client
type mockVectorClient struct {
	FindNeighborsFunc func(ctx context.Context, vector []float32, neighborCount int) ([]vector.SearchResult, error)
	CloseFunc         func() error
}

func (m *mockVectorClient) FindNeighbors(ctx context.Context, vec []float32, neighborCount int) ([]vector.SearchResult, error) {
	if m.FindNeighborsFunc != nil {
		return m.FindNeighborsFunc(ctx, vec, neighborCount)
	}
	return nil, errors.New("FindNeighbors not implemented")
}

func (m *mockVectorClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestVectorSearchTool_Success(t *testing.T) {
	ctx := context.Background()
	queryText := "test footstep sound"
	expectedEmbedding := []float32{0.5, -0.5, 0.1}
	expectedNeighbors := []vector.SearchResult{
		{ID: "asset_footstep_gravel", Distance: 0.95},
		{ID: "asset_footstep_wood", Distance: 0.85},
	}

	vtxMock := &mockVertexClient{
		EmbedTextFunc: func(ctx context.Context, text string) ([]float32, error) {
			if text != queryText {
				t.Errorf("expected query text %q, got %q", queryText, text)
			}
			return expectedEmbedding, nil
		},
	}

	vecMock := &mockVectorClient{
		FindNeighborsFunc: func(ctx context.Context, vec []float32, count int) ([]vector.SearchResult, error) {
			if len(vec) != len(expectedEmbedding) || vec[0] != expectedEmbedding[0] {
				t.Errorf("unexpected vector passed to FindNeighbors")
			}
			if count != 2 {
				t.Errorf("expected neighbor count 2, got %d", count)
			}
			return expectedNeighbors, nil
		},
	}

	vectorSearchTool, err := NewVectorSearchTool(vtxMock, vecMock)
	if err != nil {
		t.Fatalf("failed to create VectorSearchTool: %v", err)
	}

	if vectorSearchTool.Name() != "vector_search" {
		t.Errorf("expected tool name 'vector_search', got %q", vectorSearchTool.Name())
	}

	// Invoke and assert the handler directly
	resp, err := VectorSearch(ctx, vtxMock, vecMock, VectorSearchArgs{
		Query: queryText,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}

	if resp.Results[0].ID != "asset_footstep_gravel" || resp.Results[0].Distance != 0.95 {
		t.Errorf("unexpected result[0]: %+v", resp.Results[0])
	}

	if resp.Results[1].ID != "asset_footstep_wood" || resp.Results[1].Distance != 0.85 {
		t.Errorf("unexpected result[1]: %+v", resp.Results[1])
	}
}
