package tool

import (
	"context"
	"errors"
	"testing"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
)

// mockVectorClient is a mock implementation of vector.Client mapping to Vector Search 2.0 Search interface.
type mockVectorClient struct {
	SearchFunc func(ctx context.Context, query string, limit int) ([]vector.SearchResult, error)
	CloseFunc  func() error
}

func (m *mockVectorClient) Search(ctx context.Context, query string, limit int) ([]vector.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, limit)
	}
	return nil, errors.New("Search not implemented")
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
	expectedLimit := 2
	expectedNeighbors := []vector.SearchResult{
		{
			ID:          "asset-footstep-gravel",
			Path:        "Content/Blueprints/ABP_Echo_IK.uasset",
			Type:        "Blueprint Asset",
			Description: "Echo Character Animation Blueprint",
			Distance:    0.95,
		},
		{
			ID:          "asset-footstep-wood",
			Path:        "Source/OpenWorldRPG/Characters/RPGCharacter.h",
			Type:        "C++ Source Header",
			Description: "Main playable character base class",
			Distance:    0.85,
		},
	}

	vecMock := &mockVectorClient{
		SearchFunc: func(ctx context.Context, query string, limit int) ([]vector.SearchResult, error) {
			if query != queryText {
				t.Errorf("expected search query %q, got %q", queryText, query)
			}
			if limit != expectedLimit {
				t.Errorf("expected limit %d, got %d", expectedLimit, limit)
			}
			return expectedNeighbors, nil
		},
	}

	vectorSearchTool, err := NewVectorSearchTool(vecMock)
	if err != nil {
		t.Fatalf("failed to create VectorSearchTool: %v", err)
	}

	if vectorSearchTool.Name() != "vector_search" {
		t.Errorf("expected tool name 'vector_search', got %q", vectorSearchTool.Name())
	}

	// Invoke and assert the handler directly
	resp, err := VectorSearch(ctx, vecMock, VectorSearchArgs{
		Query: queryText,
		Limit: expectedLimit,
	})
	if err != nil {
		t.Fatalf("unexpected error during VectorSearch call: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}

	r1 := resp.Results[0]
	if r1.ID != "asset-footstep-gravel" || r1.Path != "Content/Blueprints/ABP_Echo_IK.uasset" || r1.Type != "Blueprint Asset" || r1.Description != "Echo Character Animation Blueprint" || r1.Distance != 0.95 {
		t.Errorf("unexpected result[0]: %+v", r1)
	}

	r2 := resp.Results[1]
	if r2.ID != "asset-footstep-wood" || r2.Path != "Source/OpenWorldRPG/Characters/RPGCharacter.h" || r2.Type != "C++ Source Header" || r2.Description != "Main playable character base class" || r2.Distance != 0.85 {
		t.Errorf("unexpected result[1]: %+v", r2)
	}
}
