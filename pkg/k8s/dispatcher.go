package k8s

import (
	"context"
	"fmt"
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
