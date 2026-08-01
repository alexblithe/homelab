// package internal

// import (
// 	"context"
// 	"testing"

// 	"github.com/testcontainers/testcontainers-go"
// 	"github.com/testcontainers/testcontainers-go/modules/k3s"
// 	"k8s.io/client-go/kubernetes"
// 	"k8s.io/client-go/rest"
// 	"k8s.io/client-go/tools/clientcmd"
// )

// // K3sCluster provides a K3s cluster and Kubernetes client for tests.
// type K3sCluster struct {
// 	Container  testcontainers.Container
// 	KubeConfig string
// 	Client     *kubernetes.Clientset
// 	RestCfg    *rest.Config
// }

// // SetupK3s starts a K3s container and returns a K3sCluster with a Kubernetes client.
// // It registers cleanup so the container is removed even if the test fails.
// func SetupK3s(t *testing.T) *K3sCluster {
// 	t.Helper()

// 	ctx := context.Background()

// 	container, err := k3s.Run(ctx, "rancher/k3s:v1.27.1-k3s1")
// 	if err != nil {
// 		t.Fatalf("failed to start K3s container: %v", err)
// 	}

// 	t.Cleanup(func() {
// 		testcontainers.CleanupContainer(t, container)
// 	})

// 	kubeConfig, err := container.GetKubeConfig(ctx)
// 	if err != nil {
// 		t.Fatalf("failed to get K3s kubeconfig: %v", err)
// 	}

// 	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
// 	if err != nil {
// 		t.Fatalf("failed to build REST config from kubeconfig: %v", err)
// 	}

// 	client, err := kubernetes.NewForConfig(restCfg)
// 	if err != nil {
// 		t.Fatalf("failed to create Kubernetes client: %v", err)
// 	}

// 	return &K3sCluster{
// 		Container:  container,
// 		KubeConfig: kubeConfig,
// 		Client:     client,
// 		RestCfg:    restCfg,
// 	}
// }
