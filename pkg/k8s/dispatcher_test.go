package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDispatchJob_Success(t *testing.T) {
	ctx := context.Background()
	fakeClientset := fake.NewSimpleClientset()
	dispatcher := NewDispatcherWithClientset(fakeClientset)

	setupTestEnv(t)

	jobName := "test-agent-job"
	imageName := "gcr.io/project/agent-runner:latest"
	args := []string{"--task", "generate-foley", "--prompt", "footsteps in gravel"}

	createdJob, err := dispatcher.DispatchJob(ctx, jobName, imageName, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdJob == nil {
		t.Fatal("expected created job to be non-nil")
	}

	if createdJob.Name != jobName {
		t.Errorf("expected job name %s, got %s", jobName, createdJob.Name)
	}

	if createdJob.Namespace != "game-assist" {
		t.Errorf("expected namespace game-assist, got %s", createdJob.Namespace)
	}

	// Verify container spec
	podSpec := createdJob.Spec.Template.Spec
	if podSpec.RuntimeClassName == nil || *podSpec.RuntimeClassName != "gvisor" {
		t.Errorf("expected runtimeClassName gvisor, got %v", podSpec.RuntimeClassName)
	}

	if podSpec.ServiceAccountName != "game-assist-agent-runner" {
		t.Errorf("expected service account game-assist-agent-runner, got %s", podSpec.ServiceAccountName)
	}

	if len(podSpec.Containers) != 1 {
		t.Fatalf("expected exactly 1 container, got %d", len(podSpec.Containers))
	}

	container := podSpec.Containers[0]
	if container.Image != imageName {
		t.Errorf("expected image %s, got %s", imageName, container.Image)
	}

	if len(container.Args) != 4 || container.Args[0] != "--task" {
		t.Errorf("unexpected container args: %v", container.Args)
	}

	// Verify environment variables
	expectedEnv := map[string]string{
		"GCS_BUCKET":               "test-bucket",
		"GCP_PROJECT":              "test-project",
		"GITHUB_TOKEN_SECRET_PATH": "/secrets/github-token",
		"GITHUB_OWNER":             "test-owner",
		"GITHUB_REPO":              "test-repo",
		"GCP_LOCATION":             "us-central1", // default value
		"GITHUB_BASE_BRANCH":       "main",        // default value
	}

	for _, env := range container.Env {
		val, exists := expectedEnv[env.Name]
		if !exists {
			t.Errorf("unexpected environment variable in container: %s", env.Name)
			continue
		}
		if env.Value != val {
			t.Errorf("expected env %s to have value %s, got %s", env.Name, val, env.Value)
		}
		delete(expectedEnv, env.Name)
	}

	if len(expectedEnv) > 0 {
		t.Errorf("missing expected environment variables in container: %v", expectedEnv)
	}

	// Check the mock cluster directly
	jobsList, err := fakeClientset.BatchV1().Jobs("game-assist").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list jobs in mock cluster: %v", err)
	}

	if len(jobsList.Items) != 1 {
		t.Errorf("expected exactly 1 job in cluster state, got %d", len(jobsList.Items))
	}
}

func TestDispatchJob_InvalidParams(t *testing.T) {
	ctx := context.Background()
	fakeClientset := fake.NewSimpleClientset()
	dispatcher := NewDispatcherWithClientset(fakeClientset)

	setupTestEnv(t)

	// Test empty jobName
	_, err := dispatcher.DispatchJob(ctx, "", "image", []string{})
	if err == nil {
		t.Error("expected error for empty jobName, got nil")
	}

	// Test empty imageName
	_, err = dispatcher.DispatchJob(ctx, "job", "", []string{})
	if err == nil {
		t.Error("expected error for empty imageName, got nil")
	}
}

func TestWaitForJob_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeClientset := fake.NewSimpleClientset()
	dispatcher := NewDispatcherWithClientset(fakeClientset)

	setupTestEnv(t)

	jobName := "test-job"
	namespace := "game-assist"

	// Submit job
	_, err := dispatcher.DispatchJob(ctx, jobName, "runner-image", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate Job completion in a goroutine
	go func() {
		// Wait a bit then update job status
		// Retrieve job
		job, err := fakeClientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return
		}
		job.Status.Succeeded = 1
		_, _ = fakeClientset.BatchV1().Jobs(namespace).UpdateStatus(ctx, job, metav1.UpdateOptions{})
	}()

	err = dispatcher.WaitForJob(ctx, jobName)
	if err != nil {
		t.Errorf("expected no error waiting for job, got: %v", err)
	}
}

func TestStreamJobLogs_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeClientset := fake.NewSimpleClientset()
	dispatcher := NewDispatcherWithClientset(fakeClientset)

	setupTestEnv(t)

	jobName := "test-job"
	namespace := "game-assist"
	podName := "test-job-pod"

	// Submit job
	_, err := dispatcher.DispatchJob(ctx, jobName, "runner-image", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate Pod creation and running state
	go func() {
		// Create pod with matching labels
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels: map[string]string{
					"job-name": jobName,
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
		_, _ = fakeClientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	}()

	// Verify that StreamJobLogs returns successfully (or with the fake's dummy stream)
	stream, err := dispatcher.StreamJobLogs(ctx, jobName)
	if err != nil {
		t.Fatalf("expected no error streaming logs in fake, got: %v", err)
	}
	if stream != nil {
		stream.Close()
	}
}

func setupTestEnv(t *testing.T) {
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("GCP_PROJECT", "test-project")
	t.Setenv("GITHUB_TOKEN_SECRET_PATH", "/secrets/github-token")
	t.Setenv("GITHUB_OWNER", "test-owner")
	t.Setenv("GITHUB_REPO", "test-repo")
	// Clear optional/default env vars to isolate from host environment
	t.Setenv("GCP_LOCATION", "")
	t.Setenv("GITHUB_BASE_BRANCH", "")
	t.Setenv("VECTOR_COLLECTION_ID", "")
	t.Setenv("GEMINI_API_KEY_SECRET_PATH", "")
}

func TestDispatchJob_EnvPropagation(t *testing.T) {
	ctx := context.Background()
	fakeClientset := fake.NewSimpleClientset()
	dispatcher := NewDispatcherWithClientset(fakeClientset)

	setupTestEnv(t)
	t.Setenv("GCP_LOCATION", "europe-west1")
	t.Setenv("GITHUB_BASE_BRANCH", "dev")
	t.Setenv("VECTOR_COLLECTION_ID", "my-collection")
	t.Setenv("GEMINI_API_KEY_SECRET_PATH", "/secrets/gemini-api-key")

	createdJob, err := dispatcher.DispatchJob(ctx, "test-job", "runner-image", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := createdJob.Spec.Template.Spec.Containers[0]
	expectedEnv := map[string]string{
		"GCS_BUCKET":                 "test-bucket",
		"GCP_PROJECT":                "test-project",
		"GITHUB_TOKEN_SECRET_PATH":   "/secrets/github-token",
		"GITHUB_OWNER":               "test-owner",
		"GITHUB_REPO":                "test-repo",
		"GCP_LOCATION":               "europe-west1",
		"GITHUB_BASE_BRANCH":         "dev",
		"VECTOR_COLLECTION_ID":       "my-collection",
		"GEMINI_API_KEY_SECRET_PATH": "/secrets/gemini-api-key",
	}

	for _, env := range container.Env {
		val, exists := expectedEnv[env.Name]
		if !exists {
			t.Errorf("unexpected environment variable in container: %s", env.Name)
			continue
		}
		if env.Value != val {
			t.Errorf("expected env %s to have value %s, got %s", env.Name, val, env.Value)
		}
		delete(expectedEnv, env.Name)
	}

	if len(expectedEnv) > 0 {
		t.Errorf("missing expected environment variables in container: %v", expectedEnv)
	}
}

func TestDispatchJob_MissingRequiredEnv(t *testing.T) {
	requiredEnvVars := []string{
		"GCS_BUCKET",
		"GCP_PROJECT",
		"GITHUB_TOKEN_SECRET_PATH",
		"GITHUB_OWNER",
		"GITHUB_REPO",
	}

	for _, envVar := range requiredEnvVars {
		t.Run(envVar, func(t *testing.T) {
			ctx := context.Background()
			fakeClientset := fake.NewSimpleClientset()
			dispatcher := NewDispatcherWithClientset(fakeClientset)

			// Explicitly clear the target env var to override host environment
			t.Setenv(envVar, "")
			// Set all other required ones
			for _, other := range requiredEnvVars {
				if other != envVar {
					t.Setenv(other, "some-val")
				}
			}

			_, err := dispatcher.DispatchJob(ctx, "test-job", "runner-image", []string{})
			if err == nil {
				t.Errorf("expected error when %s is missing, got nil", envVar)
			}
		})
	}
}
