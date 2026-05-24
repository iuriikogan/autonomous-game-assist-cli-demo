package vector

import (
	"context"
	"fmt"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
)

// MatchClient interface wraps the MatchClient from the GCloud AI Platform SDK
// to make it unit-testable.
type MatchClient interface {
	FindNeighbors(ctx context.Context, req *aiplatformpb.FindNeighborsRequest, opts ...gax.CallOption) (*aiplatformpb.FindNeighborsResponse, error)
	Close() error
}

// SearchResult represents a neighbor match from the Vector database.
type SearchResult struct {
	ID       string
	Distance float64
}

// Client defines the interface for executing Vector Search operations.
type Client interface {
	FindNeighbors(ctx context.Context, vector []float32, neighborCount int) ([]SearchResult, error)
	Close() error
}

type vectorClient struct {
	matchClient     MatchClient
	indexEndpoint   string
	deployedIndexID string
}

// NewClient creates a new Vector Search client.
func NewClient(ctx context.Context, apiEndpoint, indexEndpoint, deployedIndexID string) (Client, error) {
	if apiEndpoint == "" {
		return nil, fmt.Errorf("API endpoint cannot be empty")
	}
	if indexEndpoint == "" {
		return nil, fmt.Errorf("Index Endpoint resource name cannot be empty")
	}
	if deployedIndexID == "" {
		return nil, fmt.Errorf("Deployed Index ID cannot be empty")
	}

	// Initialize the official AI Platform MatchClient
	gClient, err := aiplatform.NewMatchClient(ctx, option.WithEndpoint(apiEndpoint))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google MatchClient: %w", err)
	}

	return &vectorClient{
		matchClient:     gClient,
		indexEndpoint:   indexEndpoint,
		deployedIndexID: deployedIndexID,
	}, nil
}

// FindNeighbors searches for nearest neighbors of a given feature vector.
func (vc *vectorClient) FindNeighbors(ctx context.Context, vector []float32, neighborCount int) ([]SearchResult, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("feature vector cannot be empty")
	}
	if neighborCount <= 0 {
		neighborCount = 10
	}

	req := &aiplatformpb.FindNeighborsRequest{
		IndexEndpoint:   vc.indexEndpoint,
		DeployedIndexId: vc.deployedIndexID,
		Queries: []*aiplatformpb.FindNeighborsRequest_Query{
			{
				Datapoint: &aiplatformpb.IndexDatapoint{
					FeatureVector: vector,
				},
				NeighborCount: int32(neighborCount),
			},
		},
	}

	resp, err := vc.matchClient.FindNeighbors(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query nearest neighbors: %w", err)
	}

	var results []SearchResult
	for _, nearest := range resp.NearestNeighbors {
		for _, neighbor := range nearest.Neighbors {
			if neighbor != nil && neighbor.Datapoint != nil {
				results = append(results, SearchResult{
					ID:       neighbor.Datapoint.DatapointId,
					Distance: neighbor.Distance,
				})
			}
		}
	}

	return results, nil
}

// Close closes the underlying MatchClient connection.
func (vc *vectorClient) Close() error {
	return vc.matchClient.Close()
}
