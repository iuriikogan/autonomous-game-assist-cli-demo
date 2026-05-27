package pr

import (
	"context"
	"testing"
)

type mockSecretManager struct {
	GetSecretFunc func(ctx context.Context, secretPath string) (string, error)
}

func (m *mockSecretManager) GetSecret(ctx context.Context, secretPath string) (string, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(ctx, secretPath)
	}
	return "mock-github-token", nil
}

func (m *mockSecretManager) Close() error {
	return nil
}

func TestPRSubAgent_Instantiation_Success(t *testing.T) {
	mockSecrets := &mockSecretManager{}

	agent, err := New(Config{
		SecretManagerClient: mockSecrets,
		SecretPath:          "projects/my-project/secrets/github-token/versions/latest",
		TargetRepoOwner:     "ikogan",
		TargetRepoName:      "autonomous-game-assist-cli",
		BaseBranch:          "main",
	})
	if err != nil {
		t.Fatalf("unexpected error during agent construction: %v", err)
	}

	if agent.Name() != "pull_request_agent" {
		t.Errorf("expected agent name 'pull_request_agent', got %q", agent.Name())
	}

	if agent.Description() == "" {
		t.Error("expected agent description to be non-empty")
	}
}

func TestPRSubAgent_Instantiation_MissingParams(t *testing.T) {
	mockSecrets := &mockSecretManager{}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "missing secret manager",
			cfg: Config{
				SecretPath:      "projects/my-project/secrets/github-token/versions/latest",
				TargetRepoOwner: "ikogan",
				TargetRepoName:  "autonomous-game-assist-cli",
			},
			wantErr: "secret manager client is required",
		},
		{
			name: "missing secret path",
			cfg: Config{
				SecretManagerClient: mockSecrets,
				TargetRepoOwner:     "ikogan",
				TargetRepoName:      "autonomous-game-assist-cli",
			},
			wantErr: "secret path is required",
		},
		{
			name: "missing owner",
			cfg: Config{
				SecretManagerClient: mockSecrets,
				SecretPath:          "projects/my-project/secrets/github-token/versions/latest",
				TargetRepoName:      "autonomous-game-assist-cli",
			},
			wantErr: "target repo owner is required",
		},
		{
			name: "missing name",
			cfg: Config{
				SecretManagerClient: mockSecrets,
				SecretPath:          "projects/my-project/secrets/github-token/versions/latest",
				TargetRepoOwner:     "ikogan",
			},
			wantErr: "target repo name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGCSURIToHTTPS(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "standard gcs uri",
			uri:  "gs://my-bucket/audio/foley_123.wav",
			want: "https://storage.googleapis.com/my-bucket/audio/foley_123.wav",
		},
		{
			name: "nested gcs uri",
			uri:  "gs://my-bucket/scripts/unreal_assist_123.py",
			want: "https://storage.googleapis.com/my-bucket/scripts/unreal_assist_123.py",
		},
		{
			name: "non-gcs uri unchanged",
			uri:  "https://example.com/asset.wav",
			want: "https://example.com/asset.wav",
		},
		{
			name: "invalid gcs uri unchanged",
			uri:  "gs://bad-uri",
			want: "gs://bad-uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gcsURIToHTTPS(tt.uri)
			if got != tt.want {
				t.Errorf("gcsURIToHTTPS(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}
