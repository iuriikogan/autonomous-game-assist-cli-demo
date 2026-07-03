package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/coordinator"
	"github.com/iuriikogan/autonomous-game-assist-cli/internal/tool"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/gcp"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/trace"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/vector"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	// Parse inputs and flags
	promptFlag := flag.String("prompt", "", "The high-level sound & asset linkage request (e.g. 'footsteps on metal floor')")
	audioFlag := flag.String("audio", "", "Path to existing WAV audio asset file (optional)")
	userIDFlag := flag.String("user", "game_dev_1", "The user ID starting the request")
	sessIDFlag := flag.String("session", fmt.Sprintf("session_%d", time.Now().Unix()), "The session ID")
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

	// Initialize OpenTelemetry tracing linked to Google Cloud Trace
	shutdown, err := trace.InitTracerProvider(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to initialize distributed tracing exporter: %v", err)
	}
	defer shutdown(ctx)

	location := os.Getenv("GCP_LOCATION")
	if location == "" {
		location = "us-central1" // standard default region for Vertex AI
	}

	secretPath := os.Getenv("GEMINI_API_KEY_SECRET_PATH") // optional, in format: projects/{project}/secrets/{secret}/versions/{version}

	githubTokenSecretPath := os.Getenv("GITHUB_TOKEN_SECRET_PATH")
	githubOwner := os.Getenv("GITHUB_OWNER")
	githubRepo := os.Getenv("GITHUB_REPO")
	githubBaseBranch := os.Getenv("GITHUB_BASE_BRANCH")
	if githubBaseBranch == "" {
		githubBaseBranch = "main"
	}

	if githubTokenSecretPath == "" || githubOwner == "" || githubRepo == "" {
		log.Fatalf("Missing required GitHub environment configurations. Require: GITHUB_TOKEN_SECRET_PATH, GITHUB_OWNER, GITHUB_REPO")
	}

	vectorCollectionID := os.Getenv("VECTOR_COLLECTION_ID")
	if vectorCollectionID == "" {
		vectorCollectionID = fmt.Sprintf("%s-dev-%s-gameassist-collection", projectID, location)
	}

	// 2. Initialize Secret Manager client if needed to resolve secrets securely
	var smClient gcp.SecretManagerClient
	if secretPath != "" || githubTokenSecretPath != "" {
		client, err := gcp.NewSecretManagerClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize Secret Manager client: %v", err)
		}
		smClient = client
		defer smClient.Close()
	}

	// Resolve Vertex API Key if secret path is supplied, otherwise rely on Workload Identity / ADC
	apiKey := ""
	if secretPath != "" && smClient != nil {
		resolvedKey, err := smClient.GetSecret(ctx, secretPath)
		if err != nil {
			log.Fatalf("Failed to resolve Vertex API Key from Secret Manager: %v", err)
		}
		apiKey = resolvedKey
	}


	// 3. Initialize GCP storage client
	storageClient, err := gcp.NewStorageClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize GCP Storage client: %v", err)
	}
	defer storageClient.Close()

	// 4. Initialize Google Vector search database client (Vector Search 2.0 Collections)
	vectorClient, err := vector.NewClient(ctx, projectID, location, vectorCollectionID)
	if err != nil {
		log.Fatalf("Failed to initialize Vector Search 2.0 client: %v", err)
	}
	defer vectorClient.Close()

	// 5. Initialize ADK Gemini LLM Backend
	genaiCfg := &genai.ClientConfig{}
	if apiKey != "" {
		genaiCfg.APIKey = apiKey
		genaiCfg.Backend = genai.BackendGeminiAPI
	} else {
		genaiCfg.Project = projectID
		genaiCfg.Location = location
		genaiCfg.Backend = genai.BackendVertexAI
	}

	// Strict adherence to Gemini 3.1 Pro for heavy reasoning sub-agents
	modelBackend, err := gemini.NewModel(ctx, "gemini-3.1-pro-preview", genaiCfg)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini 3.1 Pro backend: %v", err)
	}

	// 6. Initialize custom tools (No local embedding client required for Vector Search 2.0)
	vectorSearchTool, err := tool.NewVectorSearchTool(vectorClient)
	if err != nil {
		log.Fatalf("Failed to create Vector Search tool: %v", err)
	}

	sandboxTool, err := tool.NewSandboxTool()
	if err != nil {
		log.Fatalf("Failed to create Sandbox Validation tool: %v", err)
	}

	// 8. Construct sequential coordinator workflow
	coordinatorAgent, err := coordinator.New(coordinator.Config{
		Model:               modelBackend,
		VectorSearchTool:    vectorSearchTool,
		SandboxTool:         sandboxTool,
		StorageClient:       storageClient,
		BucketName:          bucketName,
		SecretManagerClient: smClient,
		GitHubTokenSecret:   githubTokenSecretPath,
		GitHubOwner:         githubOwner,
		GitHubRepo:          githubRepo,
		BaseBranch:          githubBaseBranch,
	})
	if err != nil {
		log.Fatalf("Failed to construct Central Coordinator sequential agent workflow: %v", err)
	}

	// 9. Initialize ADK runner and pre-populate session state with audio asset
	// 9. Initialize ADK runner and pre-populate session state with audio asset
	var audioBytes []byte
	if *audioFlag != "" {
		b, err := os.ReadFile(*audioFlag)
		if err != nil {
			log.Fatalf("Failed to read input audio file %s: %v", *audioFlag, err)
		}
		audioBytes = b
	} else {
		// Default sample WAV binary header
		audioBytes = []byte("RIFF4\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00@\x1f\x00\x00@\x1f\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00")
	}

	sessSvc := session.InMemoryService()
	_, err = sessSvc.Create(ctx, &session.CreateRequest{
		AppName:   "autonomous-game-assist-cli",
		UserID:    *userIDFlag,
		SessionID: *sessIDFlag,
		State: map[string]any{
			"audio_binary": audioBytes,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	adkRunner, err := runner.New(runner.Config{
		AppName:           "autonomous-game-assist-cli",
		Agent:             coordinatorAgent,
		SessionService:    sessSvc,
		AutoCreateSession: false,
	})
	if err != nil {
		log.Fatalf("Failed to bootstrap ADK Runner: %v", err)
	}

	log.Printf("Starting central coordinator workflow execution...")
	log.Printf("User Prompt: %q\n\n", prompt)

	// 10. Run sequential agent tree under a root span
	tr := otel.Tracer("agent-runner")
	ctx, rootSpan := tr.Start(ctx, "game-assist-workflow")

	events := adkRunner.Run(ctx, *userIDFlag, *sessIDFlag, &genai.Content{
		Parts: []*genai.Part{
			{Text: prompt},
		},
	}, agent.RunConfig{})

	var runErr error
	for event, err := range events {
		if err != nil {
			runErr = err
			break
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					fmt.Printf("[%s] %s\n", event.Author, part.Text)
				}
			}
		}
	}

	if runErr != nil {
		rootSpan.RecordError(runErr)
		rootSpan.SetStatus(codes.Error, runErr.Error())
		rootSpan.End()
		log.Printf("Execution failed with error: %v", runErr)
		os.Exit(1)
	}

	rootSpan.SetStatus(codes.Ok, "Workflow completed successfully")
	rootSpan.End()

	log.Println("\nAutonomous Link Process completed successfully.")
}
