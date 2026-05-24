package gcp

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretManagerClient defines the interface for Secret Manager operations.
type SecretManagerClient interface {
	GetSecret(ctx context.Context, secretPath string) (string, error)
	Close() error
}

type secretsClient struct {
	client *secretmanager.Client
}

// NewSecretManagerClient initializes a SecretManagerClient.
func NewSecretManagerClient(ctx context.Context) (SecretManagerClient, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	return &secretsClient{client: client}, nil
}

// GetSecret fetches the secret payload from Secret Manager using the full version path.
// secretPath should be in format: "projects/{project}/secrets/{secret}/versions/{version}"
func (s *secretsClient) GetSecret(ctx context.Context, secretPath string) (string, error) {
	if secretPath == "" {
		return "", fmt.Errorf("secretPath cannot be empty")
	}

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	}

	result, err := s.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret version %s: %w", secretPath, err)
	}

	if result.Payload == nil || len(result.Payload.Data) == 0 {
		return "", fmt.Errorf("secret payload is empty")
	}

	return string(result.Payload.Data), nil
}

// Close closes the underlying Secret Manager client connection.
func (s *secretsClient) Close() error {
	return s.client.Close()
}
