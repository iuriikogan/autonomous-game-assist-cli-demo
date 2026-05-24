package coordinator

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/audio"
	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/promptcrafter"
	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/unreal"
	"github.com/iuriikogan/autonomous-game-assist-cli/internal/agent/validation"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/gcp"
)

// Config holds all dependencies and configurations for bootstrapping the coordinator workflow.
type Config struct {
	Model            model.LLM
	VectorSearchTool tool.Tool
	SandboxTool      tool.Tool
	StorageClient    gcp.StorageClient
	BucketName       string
}

// New creates the Central Coordinator sequential agent workflow.
func New(cfg Config) (agent.Agent, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("model is required")
	}
	if cfg.VectorSearchTool == nil {
		return nil, fmt.Errorf("vector search tool is required")
	}
	if cfg.SandboxTool == nil {
		return nil, fmt.Errorf("sandbox tool is required")
	}
	if cfg.StorageClient == nil {
		return nil, fmt.Errorf("storage client is required")
	}
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	// 1. Construct sub-agents
	promptCrafter, err := promptcrafter.New(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Prompt Crafter: %w", err)
	}

	creativeAudio, err := audio.New(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Creative Audio: %w", err)
	}

	unrealAgent, err := unreal.New(cfg.Model, cfg.VectorSearchTool)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Unreal Agent: %w", err)
	}

	validationAgent, err := validation.New(cfg.Model, cfg.SandboxTool)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Validation Agent: %w", err)
	}

	gcsUploader, err := newGCSUploader(cfg.StorageClient, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to construct GCS Uploader: %w", err)
	}

	// 2. Combine sub-agents in strict sequential execution order
	seqConfig := sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "central_coordinator",
			Description: "Orchestrates Prompt Crafter, Creative Audio, Unreal Agent, Validation Agent, and uploads to GCS.",
			SubAgents: []agent.Agent{
				promptCrafter,
				creativeAudio,
				unrealAgent,
				validationAgent,
				gcsUploader,
			},
		},
	}

	return sequentialagent.New(seqConfig)
}

type gcsUploader struct {
	agent.Agent
	storageClient gcp.StorageClient
	bucketName    string
}

func newGCSUploader(storageClient gcp.StorageClient, bucketName string) (agent.Agent, error) {
	u := &gcsUploader{
		storageClient: storageClient,
		bucketName:    bucketName,
	}

	base, err := agent.New(agent.Config{
		Name:        "gcs_uploader",
		Description: "Uploads validated assets (synthesized audio, automation scripts) to Cloud Storage.",
		Run:         u.run,
	})
	if err != nil {
		return nil, err
	}
	u.Agent = base
	return u, nil
}

func (u *gcsUploader) run(ictx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		sess := ictx.Session()

		// 1. Retrieve generated foley description and synthesized audio binary from previous states
		audioVal, err := sess.State().Get("audio_binary")
		if err != nil {
			yield(nil, fmt.Errorf("state lacks required variable 'audio_binary': %w", err))
			return
		}

		// 2. Retrieve final validated UE5 script from state
		scriptVal, err := sess.State().Get("validated_script")
		if err != nil {
			yield(nil, fmt.Errorf("state lacks required variable 'validated_script': %w", err))
			return
		}

		// 3. Extract audio data
		var audioBytes []byte
		switch val := audioVal.(type) {
		case []byte:
			audioBytes = val
		case string:
			audioBytes = []byte(val)
		default:
			yield(nil, fmt.Errorf("unexpected type for 'audio_binary': %T", audioVal))
			return
		}

		// 4. Extract script code
		scriptStr, ok := scriptVal.(string)
		if !ok {
			yield(nil, fmt.Errorf("unexpected type for 'validated_script': %T", scriptVal))
			return
		}

		sessID := sess.ID()
		audioObj := fmt.Sprintf("audio/foley_%s.wav", sessID)
		scriptObj := fmt.Sprintf("scripts/unreal_assist_%s.py", sessID)

		// Upload audio artifact
		if err := u.storageClient.UploadObject(ictx, u.bucketName, audioObj, bytes.NewReader(audioBytes)); err != nil {
			yield(nil, fmt.Errorf("failed GCS upload for foley audio %q: %w", audioObj, err))
			return
		}

		// Upload script artifact
		if err := u.storageClient.UploadObject(ictx, u.bucketName, scriptObj, io.NopCloser(strings.NewReader(scriptStr))); err != nil {
			yield(nil, fmt.Errorf("failed GCS upload for UE5 script %q: %w", scriptObj, err))
			return
		}

		// Return final sequential delivery success event
		evt := session.NewEvent(ictx.InvocationID())
		evt.Content = &genai.Content{
			Parts: []*genai.Part{
				{Text: fmt.Sprintf("SUCCESS: Autonomous Sound Link Delivery Completed.\nAudio Asset URI: gs://%s/%s\nUnreal Automation Script URI: gs://%s/%s", u.bucketName, audioObj, u.bucketName, scriptObj)},
			},
		}
		yield(evt, nil)
	}
}
