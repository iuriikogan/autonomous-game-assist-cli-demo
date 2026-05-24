package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/coordinator"
	"github.com/iuriikogan/autonomous-game-assist-cli/internal/tool"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/gcp"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vertex"
)

func main() {
	// Parse inputs and flags
	promptFlag := flag.String("prompt", "", "The high-level sound & asset linkage request (e.g. 'footsteps on metal floor')")
	userIDFlag := flag.String("user", "game_dev_1", "The user ID starting the request")
	sessIDFlag := flag.String("session", "session_123", "The session ID")
	flag.Parse()

	prompt := *promptFlag
	if prompt == "" {
		log.Fatalf("Error: -prompt flag is required.")
	}

	ctx := context.Background()

	// 1. Read Environment configurations
	bucketName := os.Getenv("GCS_BUCKET")
	if bucketName == "" {
		log.Fatalf("Missing environment variable: GCS_BUCKET")
	}

	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		log.Fatalf("Missing environment variable: GCP_PROJECT")
	}

	location := os.Getenv("GCP_LOCATION")
	if location == "" {
		location = "us-central1" // standard default region for Vertex AI
	}

	secretPath := os.Getenv("GEMINI_API_KEY_SECRET_PATH") // optional, in format: projects/{project}/secrets/{secret}/versions/{version}

	vectorIndexEndpoint := os.Getenv("VECTOR_INDEX_ENDPOINT") // e.g. "projects/123456/locations/us-central1/indexEndpoints/7890"
	vectorAPIEndpoint := os.Getenv("VECTOR_API_ENDPOINT")     // e.g. "us-central1-aiplatform.googleapis.com"
	deployedIndexID := os.Getenv("DEPLOYED_INDEX_ID")

	if vectorIndexEndpoint == "" || vectorAPIEndpoint == "" || deployedIndexID == "" {
		log.Fatalf("Missing required vector environment configurations. Require: VECTOR_INDEX_ENDPOINT, VECTOR_API_ENDPOINT, DEPLOYED_INDEX_ID")
	}

	// 2. Resolve Vertex API Key if secret path is supplied, otherwise rely on Workload Identity / ADC
	apiKey := ""
	if secretPath != "" {
		smClient, err := gcp.NewSecretManagerClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize Secret Manager client: %v", err)
		}
		resolvedKey, err := smClient.GetSecret(ctx, secretPath)
		if err != nil {
			log.Fatalf("Failed to resolve Vertex API Key from Secret Manager: %v", err)
		}
		apiKey = resolvedKey
		smClient.Close()
	}

	// 3. Initialize GCP storage client
	storageClient, err := gcp.NewStorageClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize GCP Storage client: %v", err)
	}
	defer storageClient.Close()

	// 4. Initialize Google Vector search database client
	vectorClient, err := vector.NewClient(ctx, vectorAPIEndpoint, vectorIndexEndpoint, deployedIndexID)
	if err != nil {
		log.Fatalf("Failed to initialize Vector search client: %v", err)
	}
	defer vectorClient.Close()

	// 5. Initialize Vertex AI model wrapper (used by the Vector Search tool for query embeddings)
	vertexWrapper, err := vertex.NewClient(ctx, projectID, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex AI wrapper: %v", err)
	}

	// 6. Initialize ADK Gemini LLM Backend
	genaiCfg := &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	}
	if apiKey != "" {
		genaiCfg.APIKey = apiKey
	}

	// Strict adherence to Gemini 3.1 Pro for heavy reasoning sub-agents
	modelBackend, err := gemini.NewModel(ctx, "gemini-3.1-pro", genaiCfg)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini 3.1 Pro backend: %v", err)
	}

	// 7. Initialize custom tools
	vectorSearchTool, err := tool.NewVectorSearchTool(vertexWrapper, vectorClient)
	if err != nil {
		log.Fatalf("Failed to create Vector Search tool: %v", err)
	}

	sandboxTool, err := tool.NewSandboxTool()
	if err != nil {
		log.Fatalf("Failed to create Sandbox Validation tool: %v", err)
	}

	// 8. Construct sequential coordinator workflow
	coordinatorAgent, err := coordinator.New(coordinator.Config{
		Model:            modelBackend,
		VectorSearchTool: vectorSearchTool,
		SandboxTool:      sandboxTool,
		StorageClient:    storageClient,
		BucketName:       bucketName,
	})
	if err != nil {
		log.Fatalf("Failed to construct Central Coordinator sequential agent workflow: %v", err)
	}

	// 9. Initialize ADK runner
	adkRunner, err := runner.New(runner.Config{
		AppName:           "autonomous-game-assist-cli",
		Agent:             coordinatorAgent,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Fatalf("Failed to bootstrap ADK Runner: %v", err)
	}

	log.Printf("Starting central coordinator workflow execution...")
	log.Printf("User Prompt: %q\n\n", prompt)

	// 10. Run sequential agent tree and stream output events
	events := adkRunner.Run(ctx, *userIDFlag, *sessIDFlag, &genai.Content{
		Parts: []*genai.Part{
			{Text: prompt},
		},
	}, agent.RunConfig{})

	for event, err := range events {
		if err != nil {
			log.Fatalf("Execution failed with error: %v", err)
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					fmt.Printf("[%s] %s\n", event.Author, part.Text)
				}
			}
		}
	}

	log.Println("\nAutonomous Link Process completed successfully.")
}
