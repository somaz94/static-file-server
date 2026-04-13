# Deploy

This directory contains Kubernetes deployment manifests and Helmfile configuration for `static-file-server`.

<br/>

## Directory Structure

```
deploy/
├── deployment.yaml              # Standalone K8s manifests (Deployment + Service)
└── helmfile/
    ├── helmfile.yaml            # Helmfile release configuration
    └── values/
        └── mgmt.yaml            # Values for mgmt environment
```

<br/>

## Standalone Kubernetes Deployment

Apply the standalone manifest directly with `kubectl`:

```bash
kubectl apply -f deploy/deployment.yaml -n default
```

This creates:
- **Deployment** — 1 replica, `somaz940/static-file-server:v0.4.1`, non-root user (UID 65532), health checks at `/healthz`
- **Service** — ClusterIP on port 80 → 8080

<br/>

## Helmfile Deployment

[Helmfile](https://github.com/helmfile/helmfile) manages Helm releases declaratively.

### Prerequisites

- [Helm](https://helm.sh/docs/intro/install/) v3+
- [Helmfile](https://github.com/helmfile/helmfile#installation)
- Kubernetes cluster access

### Deploy

```bash
cd deploy/helmfile

# Diff before applying
helmfile -e mgmt diff

# Apply
helmfile -e mgmt apply
```

### Environment Values

| File | Description |
|------|-------------|
| `values/mgmt.yaml` | Management environment — Ingress enabled, NFS persistent storage, CORS/debug on |

Create additional environment files (e.g., `values/prod.yaml`) and reference them in `helmfile.yaml` as needed.

<br/>

## Using the Helm Chart Directly

If you prefer Helm over Helmfile:

```bash
helm repo add static-file-server https://somaz94.github.io/static-file-server/helm-repo
helm repo update

# Install with custom values
helm install my-server static-file-server/static-file-server \
  -f deploy/helmfile/values/mgmt.yaml \
  -n static-file-server --create-namespace
```

See the [Helm chart README](../helm/static-file-server/values.yaml) for all configurable values.

<br/>

## Version Management

When bumping the project version, the deploy files are updated automatically:

```bash
make bump-version VERSION=v0.5.0
```

This updates the image tag in `deployment.yaml`, chart version in `helmfile.yaml`, and image tag in `values/mgmt.yaml`.
