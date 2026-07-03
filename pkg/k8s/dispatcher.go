package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Dispatcher defines the interface for launching runner jobs on Kubernetes.
type Dispatcher interface {
	DispatchJob(ctx context.Context, jobName, imageName string, args []string) (*batchv1.Job, error)
	WaitForJob(ctx context.Context, jobName string) error
	StreamJobLogs(ctx context.Context, jobName string) (io.ReadCloser, error)
}

type k8sDispatcher struct {
	clientset kubernetes.Interface
}

// NewDispatcher initializes a Dispatcher using either local kubeconfig or InClusterConfig.
func NewDispatcher() (Dispatcher, error) {
	var config *rest.Config
	var err error

	// Try loading from local ~/.kube/config first
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	// If loading kubeconfig failed, fallback to InClusterConfig (e.g., if running inside the cluster)
	if err != nil || config == nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubernetes configuration (kubeconfig and InCluster failed): %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes clientset: %w", err)
	}

	return &k8sDispatcher{clientset: clientset}, nil
}

// NewDispatcherWithClientset is a helper for dependency injection in unit tests.
func NewDispatcherWithClientset(clientset kubernetes.Interface) Dispatcher {
	return &k8sDispatcher{clientset: clientset}
}

// DispatchJob submits a batchv1.Job executing the agent-runner under the gVisor sandbox in "game-assist" namespace.
func (kd *k8sDispatcher) DispatchJob(ctx context.Context, jobName, imageName string, args []string) (*batchv1.Job, error) {
	if jobName == "" {
		return nil, fmt.Errorf("jobName cannot be empty")
	}
	if imageName == "" {
		return nil, fmt.Errorf("imageName cannot be empty")
	}

	namespace := "game-assist"
	runtimeClassName := "gvisor"
	serviceAccountName := "game-assist-agent-runner"

	// Read and validate required environment variables
	gcsBucket := os.Getenv("GCS_BUCKET")
	if gcsBucket == "" {
		return nil, fmt.Errorf("missing required environment variable: GCS_BUCKET")
	}
	gcpProject := os.Getenv("GCP_PROJECT")
	if gcpProject == "" {
		return nil, fmt.Errorf("missing required environment variable: GCP_PROJECT")
	}
	githubTokenSecretPath := os.Getenv("GITHUB_TOKEN_SECRET_PATH")
	if githubTokenSecretPath == "" {
		return nil, fmt.Errorf("missing required environment variable: GITHUB_TOKEN_SECRET_PATH")
	}
	githubOwner := os.Getenv("GITHUB_OWNER")
	if githubOwner == "" {
		return nil, fmt.Errorf("missing required environment variable: GITHUB_OWNER")
	}
	githubRepo := os.Getenv("GITHUB_REPO")
	if githubRepo == "" {
		return nil, fmt.Errorf("missing required environment variable: GITHUB_REPO")
	}

	// Read optional environment variables with defaults
	gcpLocation := os.Getenv("GCP_LOCATION")
	if gcpLocation == "" {
		gcpLocation = "us-central1"
	}
	githubBaseBranch := os.Getenv("GITHUB_BASE_BRANCH")
	if githubBaseBranch == "" {
		githubBaseBranch = "main"
	}

	// Assemble the container's environment variables
	envVars := []corev1.EnvVar{
		{Name: "GCS_BUCKET", Value: gcsBucket},
		{Name: "GCP_PROJECT", Value: gcpProject},
		{Name: "GITHUB_TOKEN_SECRET_PATH", Value: githubTokenSecretPath},
		{Name: "GITHUB_OWNER", Value: githubOwner},
		{Name: "GITHUB_REPO", Value: githubRepo},
		{Name: "GCP_LOCATION", Value: gcpLocation},
		{Name: "GITHUB_BASE_BRANCH", Value: githubBaseBranch},
	}

	// Add optional variables if present
	if vectorCollectionID := os.Getenv("VECTOR_COLLECTION_ID"); vectorCollectionID != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "VECTOR_COLLECTION_ID", Value: vectorCollectionID})
	}
	if geminiAPIKeySecretPath := os.Getenv("GEMINI_API_KEY_SECRET_PATH"); geminiAPIKeySecretPath != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "GEMINI_API_KEY_SECRET_PATH", Value: geminiAPIKeySecretPath})
	}
	if mockAudio := os.Getenv("MOCK_AUDIO"); mockAudio != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "MOCK_AUDIO", Value: mockAudio})
	}

	// Define the secure gVisor Job spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"environment": "dev",
				"owner":       "ikogan",
				"cost-center": "gaming-assist-ai",
				"managed-by":  "game-assist-cli",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            new(int32), // 0 retries to fail fast
			TTLSecondsAfterFinished: new(int32), // Auto-cleanup standard (placeholder 300 sec below)
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RuntimeClassName:   &runtimeClassName,
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "agent-runner",
							Image:           imageName,
							Command:         []string{"/app/agent-runner"},
							Args:            args,
							Env:             envVars,
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: new(bool), // set to false
								ReadOnlyRootFilesystem:   new(bool), // set to false, or true depending on validation needs. Let's keep simple.
								RunAsNonRoot:             new(bool), // true is ideal
							},
						},
					},
				},
			},
		},
	}

	// Default backoff limit to 0 (fail fast)
	*job.Spec.BackoffLimit = 0
	// Auto clean up job 10 minutes after it finishes
	*job.Spec.TTLSecondsAfterFinished = 600

	// Disable privilege escalation for security posture
	*job.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = false

	createdJob, err := kd.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to submit kubernetes job %s: %w", jobName, err)
	}

	return createdJob, nil
}

// WaitForJob blocks until the specified Job completes (either succeeds or fails).
func (kd *k8sDispatcher) WaitForJob(ctx context.Context, jobName string) error {
	namespace := "game-assist"
	watcher, err := kd.clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("failed to watch job %s: %w", jobName, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("job watch channel closed unexpectedly for job %s", jobName)
			}
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}
			if job.Status.Succeeded > 0 {
				return nil
			}
			if job.Status.Failed > 0 {
				return fmt.Errorf("job %s failed", jobName)
			}
		}
	}
}

// StreamJobLogs returns a stream of the Job's Pod logs.
func (kd *k8sDispatcher) StreamJobLogs(ctx context.Context, jobName string) (io.ReadCloser, error) {
	namespace := "game-assist"

	// Watch for Pods belonging to this job
	watcher, err := kd.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch pods for job %s: %w", jobName, err)
	}
	defer watcher.Stop()

	var podName string
	// Wait for a Pod to be created and transition to Running, Succeeded or Failed
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil, fmt.Errorf("pod watch channel closed unexpectedly for job %s", jobName)
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				podName = pod.Name
				break
			}
		}
		if podName != "" {
			break
		}
	}

	// Now stream the logs
	req := kd.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream logs for pod %s: %w", podName, err)
	}
	return stream, nil
}
