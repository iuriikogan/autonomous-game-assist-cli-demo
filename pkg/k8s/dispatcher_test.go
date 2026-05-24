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
