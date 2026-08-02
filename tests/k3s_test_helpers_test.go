package tests

import (
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const k3sImage = "rancher/k3s:v1.27.1-k3s1"

type k3sTestEnv struct {
	container *k3s.K3sContainer
	client    *kubernetes.Clientset
}

func newK3sTestEnv(t *testing.T) *k3sTestEnv {
	t.Helper()

	ctx := t.Context()
	container, err := k3s.Run(ctx, k3sImage)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("failed to start K3s container: %v", err)
	}

	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("failed to get K3s container state: %v", err)
	}
	if state == nil || !state.Running {
		t.Fatalf("K3s container is not running")
	}

	kubeConfig, err := container.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("failed to get K3s kubeconfig: %v", err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		t.Fatalf("failed to build REST config from kubeconfig: %v", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}

	return &k3sTestEnv{
		container: container,
		client:    client,
	}
}
