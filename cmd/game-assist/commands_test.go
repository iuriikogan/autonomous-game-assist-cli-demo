package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/gcp"
	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/k8s"
	batchv1 "k8s.io/api/batch/v1"
)

type mockDispatcher struct {
	dispatchFunc func(ctx context.Context, jobName, imageName string, args []string) (*batchv1.Job, error)
	waitFunc     func(ctx context.Context, jobName string) error
	streamFunc   func(ctx context.Context, jobName string) (io.ReadCloser, error)
}

func (m *mockDispatcher) DispatchJob(ctx context.Context, jobName, imageName string, args []string) (*batchv1.Job, error) {
	if m.dispatchFunc != nil {
		return m.dispatchFunc(ctx, jobName, imageName, args)
	}
	return &batchv1.Job{}, nil
}

func (m *mockDispatcher) WaitForJob(ctx context.Context, jobName string) error {
	if m.waitFunc != nil {
		return m.waitFunc(ctx, jobName)
	}
	return nil
}

func (m *mockDispatcher) StreamJobLogs(ctx context.Context, jobName string) (io.ReadCloser, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, jobName)
	}
	return io.NopCloser(strings.NewReader("mock logs")), nil
}

type mockStorage struct {
	uploadFunc   func(ctx context.Context, bucketName, objectName string, data io.Reader) error
	downloadFunc func(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)
	closeFunc    func() error
}

func (m *mockStorage) UploadObject(ctx context.Context, bucketName, objectName string, data io.Reader) error {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, bucketName, objectName, data)
	}
	return nil
}

func (m *mockStorage) DownloadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, bucketName, objectName)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockStorage) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestGenerateCommand_Success(t *testing.T) {
	// Swap out dispatcher
	var dispatchedArgs []string
	var dispatchedImage string

	newDispatcher = func() (k8s.Dispatcher, error) {
		return &mockDispatcher{
			dispatchFunc: func(ctx context.Context, jobName, imageName string, args []string) (*batchv1.Job, error) {
				dispatchedArgs = args
				dispatchedImage = imageName
				return &batchv1.Job{}, nil
			},
		}, nil
	}
	defer func() { newDispatcher = k8s.NewDispatcher }()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs([]string{"generate", "creaky floor sound", "--session", "testsession", "--image", "test-image:latest"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error executing command, got: %v", err)
	}

	// Verify arguments passed to dispatcher
	if dispatchedImage != "test-image:latest" {
		t.Errorf("expected image test-image:latest, got: %s", dispatchedImage)
	}

	hasPrompt := false
	hasSession := false
	for i, arg := range dispatchedArgs {
		if arg == "--prompt" && dispatchedArgs[i+1] == "creaky floor sound" {
			hasPrompt = true
		}
		if arg == "--session" && dispatchedArgs[i+1] == "testsession" {
			hasSession = true
		}
	}

	if !hasPrompt {
		t.Error("dispatched args did not contain prompt")
	}
	if !hasSession {
		t.Error("dispatched args did not contain correct session ID")
	}
}

func TestDownloadCommand_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "game-assist-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	downloadedObjects := make(map[string]bool)

	newStorageClient = func(ctx context.Context) (gcp.StorageClient, error) {
		return &mockStorage{
			downloadFunc: func(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
				downloadedObjects[objectName] = true
				return io.NopCloser(strings.NewReader("mocked asset content")), nil
			},
		}, nil
	}
	defer func() { newStorageClient = gcp.NewStorageClient }()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs([]string{"download", "--session", "testsession", "--bucket", "testbucket", "--dir", tempDir})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error executing download, got: %v", err)
	}

	// We expect foley_.wav and unreal_assist_.py to be downloaded
	expectedObj1 := "audio/foley_testsession.wav"
	expectedObj2 := "scripts/unreal_assist_testsession.py"

	if !downloadedObjects[expectedObj1] {
		t.Errorf("expected GCS object %s to be downloaded", expectedObj1)
	}
	if !downloadedObjects[expectedObj2] {
		t.Errorf("expected GCS object %s to be downloaded", expectedObj2)
	}

	// Check files written to tempDir
	localWav := fmt.Sprintf("%s/foley_testsession.wav", tempDir)
	if _, err := os.Stat(localWav); os.IsNotExist(err) {
		t.Errorf("expected local WAV file to be written at %s", localWav)
	}

	localPy := fmt.Sprintf("%s/unreal_assist_testsession.py", tempDir)
	if _, err := os.Stat(localPy); os.IsNotExist(err) {
		t.Errorf("expected local Py file to be written at %s", localPy)
	}
}
