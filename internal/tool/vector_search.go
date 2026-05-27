package tool

import (
	"context"
	"fmt"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// VectorSearchArgs represents the input parameters for the Vector Search tool.
type VectorSearchArgs struct {
	Query string `json:"query" jsonschema:"The text query to search for in the vector database (e.g. 'footstep audio metadata')"`
	Limit int    `json:"limit,omitempty" jsonschema:"The maximum number of nearest neighbor results to return (default: 5)"`
}

// VectorSearchResultItem represents a single matched result with rich metadata.
type VectorSearchResultItem struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Distance    float64 `json:"distance"`
}

// VectorSearchResponse represents the response payload of the Vector Search tool.
type VectorSearchResponse struct {
	Results []VectorSearchResultItem `json:"results"`
}

// VectorSearch contains the core domain logic for executing the Vector Search 2.0 query.
func VectorSearch(ctx context.Context, vectorClient vector.Client, args VectorSearchArgs) (VectorSearchResponse, error) {
	tr := otel.Tracer("game-assist-tools")
	ctx, span := tr.Start(ctx, "vector_search_tool")
	defer span.End()

	if args.Query == "" {
		err := fmt.Errorf("query text cannot be empty")
		span.RecordError(err)
		return VectorSearchResponse{}, err
	}

	span.SetAttributes(
		attribute.String("search.query", args.Query),
		attribute.Int("search.limit", args.Limit),
	)

	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}

	// Query nearest neighbors directly using Vector Search 2.0 Auto-Embeddings
	neighbors, err := vectorClient.Search(ctx, args.Query, limit)
	if err != nil {
		span.RecordError(err)
		return VectorSearchResponse{}, fmt.Errorf("failed to query semantic collections: %w", err)
	}

	// Map results containing rich metadata fields
	results := make([]VectorSearchResultItem, len(neighbors))
	for i, n := range neighbors {
		results[i] = VectorSearchResultItem{
			ID:          n.ID,
			Path:        n.Path,
			Type:        n.Type,
			Description: n.Description,
			Distance:    n.Distance,
		}
	}

	return VectorSearchResponse{Results: results}, nil
}

// NewVectorSearchTool bootstraps the ADK function tool wrapping our Vector Search 2.0 logic.
func NewVectorSearchTool(vectorClient vector.Client) (tool.Tool, error) {
	cfg := functiontool.Config{
		Name:        "vector_search",
		Description: "Queries the Vertex AI Vector Search 2.0 database to find nearest neighbor Blueprint asset/code metadata based on a semantic text query.",
	}

	handler := func(tctx tool.Context, args VectorSearchArgs) (VectorSearchResponse, error) {
		return VectorSearch(tctx, vectorClient, args)
	}

	return functiontool.New(cfg, handler)
}
