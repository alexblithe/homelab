package tests

import (
	"context"
	"fmt"
	"log"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/testcontainers/testcontainers-go/modules/k3s"
)

func TestK3sSmoke(t *testing.T) {
	ctx := context.Background()
	k3sContainer, err := k3s.Run(ctx, "rancher/k3s:v1.27.1-k3s1")

	defer func() {
		if err := k3sContainer.Terminate(ctx); err != nil {
			log.Printf("failed to terminate K3s container: %v", err)
		}
	}()

	if err != nil {
		log.Printf("failed to start K3s container: %v", err)
		return
	}

	state, err := k3sContainer.State(ctx)
	if err != nil {
		log.Printf("failed to get K3s container state: %v", err)
		return
	}

	fmt.Println(state.Running)
	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		log.Printf("failed to get kubeconfig: %s", err)
		return
	}

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		log.Printf("failed to create rest config: %s", err)
		return
	}

	k8s, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		log.Printf("failed to create k8s client: %s", err)
		return
	}

	nodes, err := k8s.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		log.Printf("failed to list nodes: %s", err)
		return
	}

	fmt.Println(len(nodes.Items))

}
