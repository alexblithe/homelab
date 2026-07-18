"""Basic smoke test for K3s cluster using pytest and Kubernetes Python client."""
import typing

from kubernetes import client

if typing.TYPE_CHECKING:
    from kubernetes.client import V1NamespaceList


def test_create_and_get_namespaces(kube_client: client.CoreV1Api) -> None:
    """Test creating a namespace and retrieving the list of namespaces."""
    namespace = "test-namespace"
    kube_client.create_namespace(
        client.V1Namespace(metadata=client.V1ObjectMeta(name=namespace)),
    )
    namespaces = typing.cast("V1NamespaceList", kube_client.list_namespace())
    assert namespaces is not None
    if namespaces.items is not None:
        assert len(namespaces.items) > 0, "No namespaces found in the cluster"
        assert any(ns.metadata.name == namespace for ns in namespaces.items), (
            f"Namespace '{namespace}' not found in the cluster"
        )
