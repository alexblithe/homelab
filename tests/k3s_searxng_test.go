package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	logger := log.New(os.Stdout, "", log.LstdFlags)
	namespaceLogger := NewNamespaceLogger(env.client, "test-searxng", logger)
	go namespaceLogger.Start(t.Context())

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

	// --- Port-forward to the SearXNG service ---
	serviceName := "test-searxng"
	localPort := 8080
	targetPort := 80

	pf := NewPortForwarder(env.restConfig, env.client, namespace, serviceName, localPort, targetPort)
	err = pf.Start(t.Context())
	if err != nil {
		t.Fatalf("failed to start port-forward: %v", err)
	}
	defer pf.Stop()

	t.Logf("Port-forward established: localhost:%d -> %s:%d", localPort, serviceName, targetPort)

	// --- Hit the SearXNG search API ---
	searchURL := fmt.Sprintf("http://localhost:%d/search?q=test&format=json", localPort)
	t.Logf("Sending search request to %s", searchURL)

	resp, err := http.Get(searchURL)
	if err != nil {
		t.Fatalf("search API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if !json.Valid(body) {
		t.Fatalf("response body is not valid JSON: %s", string(body))
	}

	t.Log("SearXNG search API returned valid JSON response")
}
