package vector

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"github.com/googleapis/gax-go/v2"
)

// mockMatchClient is a mock of MatchClient.
type mockMatchClient struct {
	FindNeighborsFunc func(ctx context.Context, req *aiplatformpb.FindNeighborsRequest, opts ...gax.CallOption) (*aiplatformpb.FindNeighborsResponse, error)
	CloseFunc         func() error
}

func (m *mockMatchClient) FindNeighbors(ctx context.Context, req *aiplatformpb.FindNeighborsRequest, opts ...gax.CallOption) (*aiplatformpb.FindNeighborsResponse, error) {
	if m.FindNeighborsFunc != nil {
		return m.FindNeighborsFunc(ctx, req, opts...)
	}
	return nil, errors.New("FindNeighbors not implemented")
}

func (m *mockMatchClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestFindNeighbors_Success(t *testing.T) {
	ctx := context.Background()
	expectedVector := []float32{0.1, 0.2, 0.3}
	expectedIndexEndpoint := "projects/p/locations/l/indexEndpoints/ie"
	expectedDeployedIndexID := "di"

	mockClient := &mockMatchClient{
		FindNeighborsFunc: func(ctx context.Context, req *aiplatformpb.FindNeighborsRequest, opts ...gax.CallOption) (*aiplatformpb.FindNeighborsResponse, error) {
			if req.IndexEndpoint != expectedIndexEndpoint {
				t.Errorf("expected index endpoint %s, got %s", expectedIndexEndpoint, req.IndexEndpoint)
			}
			if req.DeployedIndexId != expectedDeployedIndexID {
				t.Errorf("expected deployed index ID %s, got %s", expectedDeployedIndexID, req.DeployedIndexId)
			}
			if len(req.Queries) != 1 || len(req.Queries[0].Datapoint.FeatureVector) != 3 {
				t.Errorf("unexpected queries count or vector length")
			}
			return &aiplatformpb.FindNeighborsResponse{
				NearestNeighbors: []*aiplatformpb.FindNeighborsResponse_NearestNeighbors{
					{
						Neighbors: []*aiplatformpb.FindNeighborsResponse_Neighbor{
							{
								Datapoint: &aiplatformpb.IndexDatapoint{
									DatapointId: "match-1",
								},
								Distance: 0.95,
							},
							{
								Datapoint: &aiplatformpb.IndexDatapoint{
									DatapointId: "match-2",
								},
								Distance: 0.88,
							},
						},
					},
				},
			}, nil
		},
	}

	client := &vectorClient{
		matchClient:     mockClient,
		indexEndpoint:   expectedIndexEndpoint,
		deployedIndexID: expectedDeployedIndexID,
	}

	results, err := client.FindNeighbors(ctx, expectedVector, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].ID != "match-1" || results[0].Distance != 0.95 {
		t.Errorf("unexpected result at index 0: %+v", results[0])
	}

	if results[1].ID != "match-2" || results[1].Distance != 0.88 {
		t.Errorf("unexpected result at index 1: %+v", results[1])
	}
}

func TestFindNeighbors_EmptyVector(t *testing.T) {
	ctx := context.Background()
	client := &vectorClient{
		matchClient:     &mockMatchClient{},
		indexEndpoint:   "endpoint",
		deployedIndexID: "deployed",
	}

	_, err := client.FindNeighbors(ctx, []float32{}, 5)
	if err == nil {
		t.Fatal("expected error for empty vector, got nil")
	}
}

func TestFindNeighbors_APIError(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockMatchClient{
		FindNeighborsFunc: func(ctx context.Context, req *aiplatformpb.FindNeighborsRequest, opts ...gax.CallOption) (*aiplatformpb.FindNeighborsResponse, error) {
			return nil, errors.New("network timeout")
		},
	}

	client := &vectorClient{
		matchClient:     mockClient,
		indexEndpoint:   "endpoint",
		deployedIndexID: "deployed",
	}

	_, err := client.FindNeighbors(ctx, []float32{0.1}, 5)
	if err == nil {
		t.Fatal("expected error from API call failure, got nil")
	}
}
