package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

func main() {
	// 1. Parse CLI inputs
	srcDirFlag := flag.String("src", "scratch/OpenWorldRPG", "The relative or absolute path to the cloned OpenWorldRPG repository")
	projectIDFlag := flag.String("project", "", "The GCP Project ID")
	locationFlag := flag.String("location", "us-central1", "The GCP location/region")
	collectionIDFlag := flag.String("collection", "", "The Vector Search 2.0 Collection ID")
	flag.Parse()

	// Fallback to environment variables if flags are omitted
	projectID := *projectIDFlag
	if projectID == "" {
		projectID = os.Getenv("GCP_PROJECT")
		if projectID == "" {
			log.Fatalf("Error: --project flag or GCP_PROJECT env variable is required.")
		}
	}

	location := *locationFlag
	if location == "" {
		location = os.Getenv("GCP_LOCATION")
		if location == "" {
			location = "us-central1"
		}
	}

	collectionID := *collectionIDFlag
	if collectionID == "" {
		collectionID = fmt.Sprintf("%s-dev-%s-gameassist-collection", projectID, location)
	}

	srcDir := *srcDirFlag
	if srcDir == "" {
		log.Fatalf("Error: --src directory cannot be empty.")
	}

	ctx := context.Background()

	// 2. Initialize authenticated HTTP client utilizing Application Default Credentials (ADC)
	httpClient, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("Failed to initialize authenticated Google HTTP Client: %v", err)
	}

	// 3. Initialize GenAI Client pointing to Vertex AI Backend
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Google GenAI client: %v", err)
	}

	fmt.Println("==================================================")
	fmt.Printf("Starting OpenWorldRPG Vector Search 2.0 Indexer\n")
	fmt.Printf("Source Code Directory: %s\n", srcDir)
	fmt.Printf("GCP Project ID:        %s\n", projectID)
	fmt.Printf("GCP Region:            %s\n", location)
	fmt.Printf("Target Collection ID:  %s\n", collectionID)
	fmt.Println("==================================================")

	// 4. Construct and run indexer
	indexer := NewIndexer(ctx, genaiClient, httpClient, projectID, location, collectionID, srcDir)
	if err := indexer.Run(ctx); err != nil {
		log.Fatalf("Indexer execution failed: %v", err)
	}

	fmt.Println("\nIndexer completed successfully! All elements processed.")
}
