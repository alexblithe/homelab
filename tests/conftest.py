"""Common fixtures for pytest tests."""
from collections.abc import Generator

import pytest
import yaml
from kubernetes import client, config
from testcontainers.k3s import K3SContainer


@pytest.fixture(scope="session")
def kube_client() -> Generator[client.CoreV1Api, None, None]:
    """Fixture to provide a Kubernetes client for tests."""
    k3s_container = K3SContainer()
    k3s_container.start()

    try:
        kubeconfig = k3s_container.config_yaml()
        config.load_kube_config_from_dict(yaml.safe_load(kubeconfig))

        v1 = client.CoreV1Api()
        yield v1
    finally:
        k3s_container.stop()
