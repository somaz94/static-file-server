# Deployment Guide

<br/>

## Binary

### Build from source

```bash
git clone https://github.com/somaz94/static-file-server.git
cd static-file-server
make build
./bin/static-file-server
```

### Install to system

```bash
make install    # copies to /usr/local/bin
make uninstall  # removes from /usr/local/bin
```

### Cross-compile

```bash
make cross-build  # builds for linux/darwin amd64/arm64
ls dist/
```

<br/>

## Docker

### Quick start

```bash
docker run -d \
  --name static-file-server \
  -p 8080:8080 \
  -v /path/to/files:/web:ro \
  somaz940/static-file-server:latest
```

### With environment variables

```bash
docker run -d \
  --name static-file-server \
  -p 3000:3000 \
  -v /path/to/files:/web:ro \
  -e PORT=3000 \
  -e CORS=true \
  -e SHOW_LISTING=true \
  somaz940/static-file-server:latest
```

### Using Makefile

```bash
# Deploy locally (builds image + runs container)
make deploy
make deploy DEPLOY_PORT=3000 DEPLOY_VOLUME=/path/to/files

# Smoke test
make test-deploy

# Stop and remove
make undeploy
```

<br/>

## Kubernetes (kubectl)

### Deploy with raw manifests

```bash
# Deploy to default namespace
make deploy-k8s

# Deploy to specific namespace
make deploy-k8s K8S_NAMESPACE=web

# Remove
make undeploy-k8s
```

The manifests are in `deploy/deployment.yaml` and create:
- Deployment (1 replica, non-root, resource limits, liveness/readiness probes)
- Service (ClusterIP, port 80 → 8080)

To provide content, modify the volume in `deploy/deployment.yaml` to use a PVC, ConfigMap, or hostPath.

<br/>

## Kubernetes (Helm)

### Install from local chart

```bash
helm install my-server ./helm/static-file-server

# With custom values
helm install my-server ./helm/static-file-server \
  --set config.cors=true \
  --set config.showListing=true \
  --set service.type=LoadBalancer
```

### Install from Helm repository

```bash
helm repo add static-file-server https://somaz94.github.io/static-file-server/helm-repo
helm repo update
helm install my-server static-file-server/static-file-server
```

### Common configurations

#### Serve files from PVC

```yaml
# values-pvc.yaml
persistence:
  enabled: true
  accessMode: ReadOnlyMany
  size: 5Gi
  storageClass: nfs
```

```bash
helm install my-server ./helm/static-file-server -f values-pvc.yaml
```

#### Serve files from ConfigMap (small sites)

```yaml
# values-configmap.yaml
content:
  enabled: true
  files:
    index.html: |
      <!DOCTYPE html>
      <html><body><h1>Hello World!</h1></body></html>
    style.css: |
      body { font-family: sans-serif; }
```

```bash
helm install my-server ./helm/static-file-server -f values-configmap.yaml
```

#### With Ingress

```yaml
# values-ingress.yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: files.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: files-tls
      hosts:
        - files.example.com
```

```bash
helm install my-server ./helm/static-file-server -f values-ingress.yaml
```

### Upgrade and uninstall

```bash
helm upgrade my-server ./helm/static-file-server --set image.tag=v0.2.0
helm uninstall my-server
```

<br/>

## Kubernetes (Helmfile)

Example helmfile configuration is provided in `deploy/helmfile/`.

### Prerequisites

```bash
# Install helmfile
brew install helmfile    # macOS
# or download from https://github.com/helmfile/helmfile/releases
```

### Deploy with helmfile

```bash
# Apply to mgmt environment
helmfile -f deploy/helmfile/helmfile.yaml -e mgmt apply

# Preview changes
helmfile -f deploy/helmfile/helmfile.yaml -e mgmt diff

# Destroy
helmfile -f deploy/helmfile/helmfile.yaml -e mgmt destroy
```

### Customize

Copy `deploy/helmfile/values/mgmt.yaml` and adjust for your environment:

```bash
cp deploy/helmfile/values/mgmt.yaml deploy/helmfile/values/prod.yaml
# Edit prod.yaml with your domain, storage, and resource values
```

Then add the new environment to `deploy/helmfile/helmfile.yaml`:

```yaml
environments:
  mgmt:
    values:
      - values/mgmt.yaml
  prod:
    values:
      - values/prod.yaml
```
