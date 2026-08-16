# Overview
- This is my personal homelab configuration.

# Key rules
1. Dataloss is unacceptable. All data must be backed up to a remote location before any destructive operation is performed.
2. Always think about resiliency and redundancy. If a single point of failure exists, it must be mitigated.
3. Use tailscale to ensure security rather than traditional ingresses.

# Project structure
- `environments/`: Contains all the host configurations for the homelab for various environments. Includes secrets
- `charts/`: Contains all the self-developed helm-charts for the homelab.
- `base-install/`: Contains all the base helmfile, this installs device plugins and other onetime configuration
- `helmfile.yaml.gotmpl`: The main helmfile that is used to deploy the homelab.
- `.sops.yaml`: The configuration file for sops, which is used to encrypt secrets in the repository.
- `tests/`: Python pytest tests for chart integration testing (testcontainers + k3d).
- `Makefile`: Project task runner for tests and venv management.
- `flake.nix`: The flake configuration file for the homelab.
- `pyproject.toml`: The poetry configuration file for the tests for the homelab

# Coding standards
- **Makefiles**: Keep venv targets idempotent — check if the venv exists before creating, don't blindly run side-effects like `pip install --upgrade pip`.
- **General**: Don't use clever one-liners when straightforward code is clearer. Validate preconditions before running commands. Prefer explicit over implicit.

# Tooling
- **Python**: Used for testing and integration testing of the homelab.
- **Helmfile**: Used to manage the helm charts for the homelab.
- **Helm**: Used to deploy the helm charts for the homelab.
- **Sops**: Used to encrypt secrets in the repository.
- **Nix**: Used to manage the development environment for the homelab.
- **TestContainers**: Used to run integration tests for the homelab against a temporary k3d cluster.
