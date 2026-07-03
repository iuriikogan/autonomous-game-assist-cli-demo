package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Indexer coordinates scanning the repository and uploading data objects to Vector Search 2.0.
type Indexer struct {
	genaiClient  *genai.Client
	httpClient   *http.Client
	projectID    string
	location     string
	collectionID string
	srcDir       string
}

// DataObject represents the JSON body structured for Vector Search 2.0.
type DataObject struct {
	Data DataFields `json:"data"`
}

// DataFields maps exact keys defined in our Collection data schema.
type DataFields struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// NewIndexer initializes the OpenWorldRPG codebase indexer.
func NewIndexer(ctx context.Context, genaiClient *genai.Client, httpClient *http.Client, projectID, location, collectionID, srcDir string) *Indexer {
	return &Indexer{
		genaiClient:  genaiClient,
		httpClient:   httpClient,
		projectID:    projectID,
		location:     location,
		collectionID: collectionID,
		srcDir:       srcDir,
	}
}

// Run walks the directory, analyzes each target file, and inserts data objects into the collection.
func (idx *Indexer) Run(ctx context.Context) error {
	sourcePath := filepath.Join(idx.srcDir, "Source")
	contentPath := filepath.Join(idx.srcDir, "Content")

	fmt.Printf("Starting walk of source directory: %s\n", sourcePath)
	if err := idx.walkAndIndex(ctx, sourcePath, []string{".h", ".cpp"}, "C++ Source"); err != nil {
		return fmt.Errorf("error walking C++ sources: %w", err)
	}

	fmt.Printf("Starting walk of blueprint directory: %s\n", contentPath)
	if err := idx.walkAndIndex(ctx, contentPath, []string{".uasset"}, "Blueprint Asset"); err != nil {
		return fmt.Errorf("error walking blueprint assets: %w", err)
	}

	return nil
}

func (idx *Indexer) walkAndIndex(ctx context.Context, rootDir string, extensions []string, assetType string) error {
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		fmt.Printf("Directory %s does not exist, skipping.\n", rootDir)
		return nil
	}

	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Match extensions
		match := false
		for _, ext := range extensions {
			if strings.ToLower(filepath.Ext(path)) == ext {
				match = true
				break
			}
		}
		if !match {
			return nil
		}

		// Compute relative path from source root for clear identification
		relPath, err := filepath.Rel(idx.srcDir, path)
		if err != nil {
			relPath = path
		}

		fmt.Printf("Analyzing [%s]: %s...\n", assetType, relPath)

		var summary string
		if assetType == "C++ Source" {
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read C++ file %s: %w", path, err)
			}
			summary, err = idx.summarizeCode(ctx, relPath, string(contentBytes))
			if err != nil {
				fmt.Printf("Warning: failed to summarize code for %s: %v\n", relPath, err)
				return nil
			}
		} else {
			// For blueprints, summarize based on name and context
			summary, err = idx.summarizeBlueprint(ctx, relPath)
			if err != nil {
				fmt.Printf("Warning: failed to summarize blueprint for %s: %v\n", relPath, err)
				return nil
			}
		}

		// Upload to Vector Search 2.0
		if err := idx.uploadDataObject(ctx, relPath, assetType, summary); err != nil {
			return fmt.Errorf("failed to upload data object for %s: %w", relPath, err)
		}

		fmt.Printf("Successfully uploaded data object for %s\n", relPath)

		// Introduce a small delay to avoid rate limits (429 Resource Exhausted)
		time.Sleep(1 * time.Second)

		return nil
	})
}

// summarizeCode calls Gemini 3.1 Flash to generate code descriptions.
func (idx *Indexer) summarizeCode(ctx context.Context, filepath string, content string) (string, error) {
	prompt := fmt.Sprintf(`You are an expert systems analyst. Your task is to analyze this Unreal Engine 5 C++ file (%s) and write a concise, one-paragraph technical summary for semantic code retrieval.
Focus on:
1. The main purpose of the class (e.g. player character base, inventory controller, weapon item).
2. Primary properties and state tracking attributes.
3. Key public methods, interfaces, and events that other classes or scripts can interact with.

Do not use introductory phrases like "Here is the summary". Output ONLY the concise technical paragraph.

Source Code:
%s`, filepath, content)

	resp, err := idx.genaiClient.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text()), nil
}

// summarizeBlueprint utilizes Gemini to guess and document the Blueprint's role based on naming conventions.
func (idx *Indexer) summarizeBlueprint(ctx context.Context, assetPath string) (string, error) {
	prompt := fmt.Sprintf(`You are an expert Unreal Engine 5 level architect. Analyze the following Blueprint asset file path in the game directory:
Path: %s

Write a concise, one-paragraph description of what this Blueprint asset represents in the game structure, what components it likely manages, and what triggers or variables it typically configures.
Use standard Unreal Engine 5 conventions (e.g. ABP_ represents Animation Blueprint, BP_ represents Blueprint Actor, etc.).

Do not use introductory phrases. Output ONLY the concise paragraph description.`, assetPath)

	resp, err := idx.genaiClient.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text()), nil
}

// uploadDataObject sends a POST request to the Vector Search 2.0 REST endpoint to upsert the Data Object.
func (idx *Indexer) uploadDataObject(ctx context.Context, path, assetType, description string) error {
	// Use MD5 of path as a unique, fixed-length ID (32 characters) to prevent 64 character limit errors
	hash := md5.Sum([]byte(path))
	id := hex.EncodeToString(hash[:])

	obj := DataObject{
		Data: DataFields{
			Path:        path,
			Type:        assetType,
			Description: description,
		},
	}

	bodyBytes, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal data object: %w", err)
	}

	// REST URL format for Vector Search 2.0:
	// POST https://vectorsearch.googleapis.com/v1beta/projects/{project}/locations/{location}/collections/{collection}/dataObjects?dataObjectId={dataObjectId}
	apiURL := fmt.Sprintf("https://vectorsearch.googleapis.com/v1beta/projects/%s/locations/%s/collections/%s/dataObjects?dataObjectId=%s",
		idx.projectID, idx.location, idx.collectionID, id)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create REST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := idx.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("REST request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("DataObject %s already exists, attempting to update via PATCH...\n", id)
		return idx.updateDataObject(ctx, id, bodyBytes)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

func (idx *Indexer) updateDataObject(ctx context.Context, id string, bodyBytes []byte) error {
	apiURL := fmt.Sprintf("https://vectorsearch.googleapis.com/v1beta/projects/%s/locations/%s/collections/%s/dataObjects/%s",
		idx.projectID, idx.location, idx.collectionID, id)

	req, err := http.NewRequestWithContext(ctx, "PATCH", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create PATCH request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := idx.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PATCH request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read PATCH response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("PATCH API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}
