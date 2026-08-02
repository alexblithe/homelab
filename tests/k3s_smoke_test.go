package tests

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestK3sSmoke(t *testing.T) {
	env := newK3sTestEnv(t)
	nodes, err := env.client.CoreV1().Nodes().List(t.Context(), v1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}
	if len(nodes.Items) == 0 {
		t.Fatal("Kubernetes API returned no nodes")
	}
}
