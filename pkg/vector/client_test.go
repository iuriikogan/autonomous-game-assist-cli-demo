package vector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch_Success(t *testing.T) {
	ctx := context.Background()
	expectedProject := "my-project"
	expectedLocation := "us-central1"
	expectedCollection := "my-collection"
	expectedQuery := "find character blueprints"
	expectedLimit := 3

	// Spin up mock HTTP Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP Parameters
		expectedPath := "/v1beta/projects/my-project/locations/us-central1/collections/my-collection/dataObjects:search"
		if r.URL.Path != expectedPath {
			t.Errorf("expected REST path %q, got %q", expectedPath, r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("expected method POST, got %s", r.Method)
		}

		// Decode and verify query payload
		var reqBody searchRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode search request: %v", err)
		}

		if reqBody.SemanticSearch.SearchField != "asset_embedding" {
			t.Errorf("expected search field 'asset_embedding', got %q", reqBody.SemanticSearch.SearchField)
		}
		if reqBody.SemanticSearch.Query != expectedQuery {
			t.Errorf("expected query %q, got %q", expectedQuery, reqBody.SemanticSearch.Query)
		}
		if reqBody.SemanticSearch.TopK != expectedLimit {
			t.Errorf("expected limit %d, got %d", expectedLimit, reqBody.SemanticSearch.TopK)
		}

		// Mock Response Payload
		resp := searchResponse{
			Results: []searchResultItem{
				{
					DataObject: dataObjectItem{
						Name: "projects/my-project/locations/us-central1/collections/my-collection/dataObjects/bp-character",
						Data: dataObjectData{
							Path:        "Content/Blueprints/BP_RPGCharacter.uasset",
							Type:        "Blueprint Asset",
							Description: "Player Character Blueprint",
						},
					},
					Distance: 0.98,
				},
				{
					DataObject: dataObjectItem{
						Name: "projects/my-project/locations/us-central1/collections/my-collection/dataObjects/bp-enemy",
						Data: dataObjectData{
							Path:        "Content/Blueprints/Enemy/BP_Enemy.uasset",
							Type:        "Blueprint Asset",
							Description: "Enemy AI Base Blueprint",
						},
					},
					Distance: 0.85,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Instantiate client with mock server base URL
	client := &vectorClient{
		httpClient:   http.DefaultClient,
		projectID:    expectedProject,
		location:     expectedLocation,
		collectionID: expectedCollection,
		baseURL:      server.URL,
	}

	results, err := client.Search(ctx, expectedQuery, expectedLimit)
	if err != nil {
		t.Fatalf("unexpected error during Search: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify Results Mapping
	r1 := results[0]
	if r1.ID != "bp-character" || r1.Path != "Content/Blueprints/BP_RPGCharacter.uasset" || r1.Type != "Blueprint Asset" || r1.Description != "Player Character Blueprint" || r1.Distance != 0.98 {
		t.Errorf("unexpected result at index 0: %+v", r1)
	}

	r2 := results[1]
	if r2.ID != "bp-enemy" || r2.Path != "Content/Blueprints/Enemy/BP_Enemy.uasset" || r2.Type != "Blueprint Asset" || r2.Description != "Enemy AI Base Blueprint" || r2.Distance != 0.85 {
		t.Errorf("unexpected result at index 1: %+v", r2)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	client := &vectorClient{
		httpClient: http.DefaultClient,
	}

	_, err := client.Search(context.Background(), "", 5)
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

func TestSearch_APIError(t *testing.T) {
	// Spin up mock failing server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal database connection failure"))
	}))
	defer server.Close()

	client := &vectorClient{
		httpClient:   http.DefaultClient,
		projectID:    "p",
		location:     "l",
		collectionID: "c",
		baseURL:      server.URL,
	}

	_, err := client.Search(context.Background(), "query", 5)
	if err == nil {
		t.Fatal("expected error from API 500 failure, got nil")
	}
}
