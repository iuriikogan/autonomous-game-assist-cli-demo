package tool

import (
	"context"
	"fmt"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vertex"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// VectorSearchArgs represents the input parameters for the Vector Search tool.
type VectorSearchArgs struct {
	Query string `json:"query" jsonschema:"The text query to search for in the vector database (e.g. 'footstep audio metadata')"`
	Limit int    `json:"limit,omitempty" jsonschema:"The maximum number of nearest neighbor results to return (default: 5)"`
}

// VectorSearchResultItem represents a single matched result.
type VectorSearchResultItem struct {
	ID       string  `json:"id"`
	Distance float64 `json:"distance"`
}

// VectorSearchResponse represents the response payload of the Vector Search tool.
type VectorSearchResponse struct {
	Results []VectorSearchResultItem `json:"results"`
}

// VectorSearch contains the core domain logic for executing the Vector Search,
// allowing for direct unit testing without mocking the ADK framework.
func VectorSearch(ctx context.Context, vertexClient vertex.Client, vectorClient vector.Client, args VectorSearchArgs) (VectorSearchResponse, error) {
	if args.Query == "" {
		return VectorSearchResponse{}, fmt.Errorf("query text cannot be empty")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}

	// 1. Embed the text query using Vertex AI
	embedding, err := vertexClient.EmbedText(ctx, args.Query)
	if err != nil {
		return VectorSearchResponse{}, fmt.Errorf("failed to embed query text: %w", err)
	}

	// 2. Query nearest neighbors from the Vector Search database
	neighbors, err := vectorClient.FindNeighbors(ctx, embedding, limit)
	if err != nil {
		return VectorSearchResponse{}, fmt.Errorf("failed to find nearest neighbors: %w", err)
	}

	// 3. Map results to response format
	results := make([]VectorSearchResultItem, len(neighbors))
	for i, n := range neighbors {
		results[i] = VectorSearchResultItem{
			ID:       n.ID,
			Distance: n.Distance,
		}
	}

	return VectorSearchResponse{Results: results}, nil
}

// NewVectorSearchTool bootstraps the ADK function tool wrapping our vector search logic.
func NewVectorSearchTool(vertexClient vertex.Client, vectorClient vector.Client) (tool.Tool, error) {
	cfg := functiontool.Config{
		Name:        "vector_search",
		Description: "Queries the Vertex AI Vector Search database to find nearest neighbor Blueprint asset metadata based on a semantic text query.",
	}

	handler := func(tctx tool.Context, args VectorSearchArgs) (VectorSearchResponse, error) {
		return VectorSearch(tctx, vertexClient, vectorClient, args)
	}

	return functiontool.New(cfg, handler)
}
