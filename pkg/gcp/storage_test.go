package gcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// mockStorageClient is a mock implementation of StorageClient for other packages to use.
type mockStorageClient struct {
	UploadObjectFunc   func(ctx context.Context, bucketName, objectName string, data io.Reader) error
	DownloadObjectFunc func(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)
	CloseFunc          func() error
}

func (m *mockStorageClient) UploadObject(ctx context.Context, bucketName, objectName string, data io.Reader) error {
	if m.UploadObjectFunc != nil {
		return m.UploadObjectFunc(ctx, bucketName, objectName, data)
	}
	return errors.New("UploadObject not implemented")
}

func (m *mockStorageClient) DownloadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	if m.DownloadObjectFunc != nil {
		return m.DownloadObjectFunc(ctx, bucketName, objectName)
	}
	return nil, errors.New("DownloadObject not implemented")
}

func (m *mockStorageClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestMockStorageClient_Upload(t *testing.T) {
	ctx := context.Background()
	uploadedData := "test audio wav data"
	called := false

	client := &mockStorageClient{
		UploadObjectFunc: func(ctx context.Context, bucketName, objectName string, data io.Reader) error {
			called = true
			if bucketName != "test-bucket" {
				t.Errorf("expected bucketName test-bucket, got %s", bucketName)
			}
			if objectName != "sound.wav" {
				t.Errorf("expected objectName sound.wav, got %s", objectName)
			}
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, data)
			if buf.String() != uploadedData {
				t.Errorf("expected data %s, got %s", uploadedData, buf.String())
			}
			return nil
		},
	}

	err := client.UploadObject(ctx, "test-bucket", "sound.wav", strings.NewReader(uploadedData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("mock function was not called")
	}
}

func TestMockStorageClient_Download(t *testing.T) {
	ctx := context.Background()
	mockData := "mocked foley sound"
	called := false

	client := &mockStorageClient{
		DownloadObjectFunc: func(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
			called = true
			return io.NopCloser(strings.NewReader(mockData)), nil
		},
	}

	rc, err := client.DownloadObject(ctx, "test-bucket", "foley.wav")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, rc)
	if buf.String() != mockData {
		t.Errorf("expected data %q, got %q", mockData, buf.String())
	}
	if !called {
		t.Error("mock function was not called")
	}
}
