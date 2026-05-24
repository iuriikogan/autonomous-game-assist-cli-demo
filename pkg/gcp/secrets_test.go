package gcp

import (
	"context"
	"errors"
	"testing"
)

// mockSecretManagerClient is a mock implementation of SecretManagerClient.
type mockSecretManagerClient struct {
	GetSecretFunc func(ctx context.Context, secretPath string) (string, error)
	CloseFunc     func() error
}

func (m *mockSecretManagerClient) GetSecret(ctx context.Context, secretPath string) (string, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(ctx, secretPath)
	}
	return "", errors.New("GetSecret not implemented")
}

func (m *mockSecretManagerClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestMockSecretManagerClient_GetSecret(t *testing.T) {
	ctx := context.Background()
	expectedPath := "projects/my-project/secrets/api-key/versions/latest"
	expectedValue := "my-super-secret-api-key"
	called := false

	client := &mockSecretManagerClient{
		GetSecretFunc: func(ctx context.Context, secretPath string) (string, error) {
			called = true
			if secretPath != expectedPath {
				t.Errorf("expected path %s, got %s", expectedPath, secretPath)
			}
			return expectedValue, nil
		},
	}

	val, err := client.GetSecret(ctx, expectedPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != expectedValue {
		t.Errorf("expected value %s, got %s", expectedValue, val)
	}
	if !called {
		t.Error("mock function was not called")
	}
}
