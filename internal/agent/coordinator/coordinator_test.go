package coordinator

import (
	"context"
	"io"
	"iter"
	"testing"

	"google.golang.org/adk/model"
)

type mockLLM struct {
	NameFunc            string
	GenerateContentFunc func(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error]
}

func (m *mockLLM) Name() string {
	if m.NameFunc != "" {
		return m.NameFunc
	}
	return "mock-gemini"
}

func (m *mockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, req, stream)
	}
	return func(yield func(*model.LLMResponse, error) bool) {}
}

type mockTool struct {
	NameFunc        string
	DescriptionFunc string
}

func (m *mockTool) Name() string        { return m.NameFunc }
func (m *mockTool) Description() string { return m.DescriptionFunc }
func (m *mockTool) IsLongRunning() bool { return false }

type mockStorage struct {
	UploadFunc func(ctx context.Context, bucketName, objectName string, data io.Reader) error
}

func (m *mockStorage) UploadObject(ctx context.Context, bucketName, objectName string, data io.Reader) error {
	if m.UploadFunc != nil {
		return m.UploadFunc(ctx, bucketName, objectName, data)
	}
	return nil
}

func (m *mockStorage) DownloadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockStorage) Close() error {
	return nil
}

type mockSecretManager struct {
	GetSecretFunc func(ctx context.Context, secretPath string) (string, error)
}

func (m *mockSecretManager) GetSecret(ctx context.Context, secretPath string) (string, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(ctx, secretPath)
	}
	return "dummy-token", nil
}

func (m *mockSecretManager) Close() error {
	return nil
}

func TestCoordinator_Instantiation(t *testing.T) {
	mockModel := &mockLLM{NameFunc: "gemini-3.1-pro"}
	mockVecTool := &mockTool{NameFunc: "vector_search", DescriptionFunc: "Mock vector search"}
	mockSandboxTool := &mockTool{NameFunc: "sandbox_tool", DescriptionFunc: "Mock sandbox"}
	mockStore := &mockStorage{}
	mockSecrets := &mockSecretManager{}

	coord, err := New(Config{
		Model:               mockModel,
		VectorSearchTool:    mockVecTool,
		SandboxTool:         mockSandboxTool,
		StorageClient:       mockStore,
		BucketName:          "my-bucket",
		SecretManagerClient: mockSecrets,
		GitHubTokenSecret:   "projects/123/secrets/git/versions/latest",
		GitHubOwner:         "ikogan",
		GitHubRepo:          "autonomous-game-assist-cli",
		BaseBranch:          "main",
	})
	if err != nil {
		t.Fatalf("failed to instantiate coordinator workflow: %v", err)
	}

	if coord.Name() != "central_coordinator" {
		t.Errorf("expected coordinator name 'central_coordinator', got %q", coord.Name())
	}

	subAgents := coord.SubAgents()
	if len(subAgents) != 4 {
		t.Fatalf("expected 4 subagents in coordinator, got %d", len(subAgents))
	}

	expectedNames := []string{"unreal_agent", "validation_agent", "gcs_uploader", "pull_request_agent"}
	for i, name := range expectedNames {
		if subAgents[i].Name() != name {
			t.Errorf("expected subagent %d to be %q, got %q", i, name, subAgents[i].Name())
		}
	}
}
