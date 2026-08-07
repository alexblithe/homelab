package tests

import (
	"testing"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestInstallK3sSearxNG(t *testing.T) {

	actionsConfig := new(action.Configuration)
	env := newK3sTestEnv(t)
	env.client.CoreV1().Namespaces().Create(t.Context(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-searxng",
		},
	}, metav1.CreateOptions{})
	go InstallLogger(t.Context(), t, env.client, "test-searxng")

	namespace := "test-searxng"
	cfgFlags := &genericclioptions.ConfigFlags{
		KubeConfig: env.kubeConfigPath,
		Namespace:  &namespace,
	}

	if err := actionsConfig.Init(cfgFlags, "test-searxng", "secret"); err != nil {
		t.Fatalf("failed to initialize Helm action configuration: %v", err)
	}

	client := action.NewInstall(actionsConfig)
	client.ReleaseName = "test-searxng"
	client.Namespace = "test-searxng"
	client.WaitStrategy = "watcher"

	chartPath := "../charts/searxng"

	chartLoader, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("failed to load SearxNG chart: %v", err)
	}

	_, err = client.Run(chartLoader, nil)
	if err != nil {
		t.Fatalf("failed to install SearxNG chart: %v", err)
	}

	t.Log("SearxNG chart installed successfully in K3s test environment")
}
