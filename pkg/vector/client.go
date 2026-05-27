package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2/google"
)

// SearchResult represents a matched record from the Vector Search 2.0 Collection.
type SearchResult struct {
	ID          string
	Path        string
	Type        string
	Description string
	Distance    float64
}

// Client defines the interface for executing Vector Search 2.0 semantic queries.
type Client interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	Close() error
}

type vectorClient struct {
	httpClient   *http.Client
	projectID    string
	location     string
	collectionID string
	baseURL      string
}

// NewClient creates a new REST-based Vector Search 2.0 client using Google Application Default Credentials.
func NewClient(ctx context.Context, projectID, location, collectionID string) (Client, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if location == "" {
		return nil, fmt.Errorf("location cannot be empty")
	}
	if collectionID == "" {
		return nil, fmt.Errorf("collection ID cannot be empty")
	}

	// Retrieve OIDC authenticated HTTP Client automatically
	httpClient, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to create Google HTTP client: %w", err)
	}

	return &vectorClient{
		httpClient:   httpClient,
		projectID:    projectID,
		location:     location,
		collectionID: collectionID,
		baseURL:      "https://vectorsearch.googleapis.com",
	}, nil
}

// Request schemas for Search
type searchRequest struct {
	SemanticSearch semanticSearchQuery `json:"semantic_search"`
}

type semanticSearchQuery struct {
	SearchField string `json:"search_field"`
	Query       string `json:"query"`
	TopK        int    `json:"top_k"`
}

// Response schemas for Search
type searchResponse struct {
	Results []searchResultItem `json:"results"`
}

type searchResultItem struct {
	DataObject dataObjectItem `json:"dataObject"`
	Distance   float64        `json:"distance"`
}

type dataObjectItem struct {
	Name string         `json:"name"`
	Data dataObjectData `json:"data"`
}

type dataObjectData struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Search performs a semantic query against Vector Search 2.0 Collection.
func (vc *vectorClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}
	if limit <= 0 {
		limit = 5
	}

	reqBody := searchRequest{
		SemanticSearch: semanticSearchQuery{
			SearchField: "asset_embedding",
			Query:       query,
			TopK:        limit,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	// Vector Search 2.0 collections search REST endpoint:
	// POST https://vectorsearch.googleapis.com/v1beta/projects/{project}/locations/{location}/collections/{collection}/dataObjects:search
	apiURL := fmt.Sprintf("%s/v1beta/projects/%s/locations/%s/collections/%s/dataObjects:search",
		vc.baseURL, vc.projectID, vc.location, vc.collectionID)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP call failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var response searchResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse search JSON response: %w", err)
	}

	var results []SearchResult
	for _, r := range response.Results {
		// Extract standard ID from the resource name path (projects/*/locations/*/collections/*/dataObjects/ID)
		parts := bytes.Split([]byte(r.DataObject.Name), []byte("/dataObjects/"))
		id := r.DataObject.Name
		if len(parts) == 2 {
			id = string(parts[1])
		}

		results = append(results, SearchResult{
			ID:          id,
			Path:        r.DataObject.Data.Path,
			Type:        r.DataObject.Data.Type,
			Description: r.DataObject.Data.Description,
			Distance:    r.Distance,
		})
	}

	return results, nil
}

// Close satisfies the Client interface.
func (vc *vectorClient) Close() error {
	// The shared default HTTP client doesn't need manual connection closing.
	return nil
}
