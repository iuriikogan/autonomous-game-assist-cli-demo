package gcp

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
)

// StorageClient defines the interface for Cloud Storage operations.
type StorageClient interface {
	UploadObject(ctx context.Context, bucketName, objectName string, data io.Reader) error
	DownloadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)
	Close() error
}

type gcsClient struct {
	client *storage.Client
}

// NewStorageClient initializes a GCS StorageClient.
func NewStorageClient(ctx context.Context) (StorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud Storage client: %w", err)
	}
	return &gcsClient{client: client}, nil
}

// UploadObject uploads a stream of data into a specific GCS bucket.
func (g *gcsClient) UploadObject(ctx context.Context, bucketName, objectName string, data io.Reader) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}
	if objectName == "" {
		return fmt.Errorf("objectName cannot be empty")
	}
	if data == nil {
		return fmt.Errorf("data reader cannot be nil")
	}

	bucket := g.client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	wc := obj.NewWriter(ctx)

	if _, err := io.Copy(wc, data); err != nil {
		_ = wc.Close() // Close it anyway to clean up
		return fmt.Errorf("failed to copy data to GCS writer: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return nil
}

// DownloadObject downloads a stream of data from a GCS object.
func (g *gcsClient) DownloadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("bucketName cannot be empty")
	}
	if objectName == "" {
		return nil, fmt.Errorf("objectName cannot be empty")
	}

	bucket := g.client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	rc, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS reader: %w", err)
	}

	return rc, nil
}

// Close closes the underlying GCS client.
func (g *gcsClient) Close() error {
	return g.client.Close()
}
