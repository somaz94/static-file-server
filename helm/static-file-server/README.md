# static-file-server Helm Chart

A Helm chart for deploying a lightweight static file server with a modern directory listing UI on Kubernetes.

<br/>

## Prerequisites

- Kubernetes >= 1.16
- Helm >= 3.0

<br/>

## Installation

```bash
# Add the Helm repository
helm repo add static-file-server https://somaz94.github.io/static-file-server/helm-repo
helm repo update

# Install with default values
helm install my-server static-file-server/static-file-server

# Install with custom values
helm install my-server static-file-server/static-file-server -f my-values.yaml
```

<br/>

## Uninstall

```bash
helm uninstall my-server
```

<br/>

## Configuration

### Image

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `somaz940/static-file-server` |
| `image.tag` | Container image tag | `v0.4.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

<br/>

### Application

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.folder` | Root folder to serve | `/web` |
| `config.port` | Server port | `8080` |
| `config.cors` | Enable CORS headers | `false` |
| `config.debug` | Enable debug logging | `false` |
| `config.allowIndex` | Serve `index.html` for directories | `true` |
| `config.showListing` | Show directory listing | `true` |
| `config.urlPrefix` | URL path prefix | `""` |
| `config.spa` | SPA mode (fallback to `index.html`) | `false` |
| `config.compression` | Enable gzip compression | `false` |
| `config.hideDotFiles` | Hide dot files (`.env`, `.git`) | `false` |
| `config.logFormat` | Log format (`text` or `json`) | `text` |
| `config.metrics` | Enable Prometheus metrics at `/metrics` | `false` |
| `config.customHeaders` | Custom response headers (comma-separated `Key:Value`) | `""` |
| `config.tlsCert` | TLS certificate file path | `""` |
| `config.tlsKey` | TLS private key file path | `""` |
| `config.tlsMinVers` | Minimum TLS version (`TLS12`/`TLS13`) | `""` |
| `config.referrers` | Allowed referrer prefixes | `""` |
| `config.accessKey` | URL parameter access key | `""` |

<br/>

### Deployment

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `revisionHistoryLimit` | Revision history limit | `3` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full release name | `""` |
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Extra pod labels | `{}` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |
| `extraEnv` | Extra environment variables | `[]` |

<br/>

### Security

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `""` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `podSecurityContext.runAsNonRoot` | Run as non-root | `true` |
| `podSecurityContext.runAsUser` | Run as user ID | `65532` |
| `podSecurityContext.fsGroup` | FS group ID | `65532` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |

<br/>

### Service & Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `80` |
| `service.annotations` | Service annotations | `{}` |
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts configuration | see `values.yaml` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

<br/>

### Resources & Probes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory` | Memory request | `32Mi` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `probes.liveness.enabled` | Enable liveness probe | `true` |
| `probes.liveness.path` | Liveness probe path | `/healthz` |
| `probes.readiness.enabled` | Enable readiness probe | `true` |
| `probes.readiness.path` | Readiness probe path | `/healthz` |

<br/>

### Storage

Three storage modes are available (choose one):

#### Mode 1: Dynamic Provisioning (Simple PVC)

```yaml
persistence:
  enabled: true
  accessMode: ReadOnlyMany
  size: 1Gi
  storageClass: "nfs-client"
```

#### Mode 2: Static Provisioning (PV + PVC)

```yaml
persistentVolumes:
  enabled: true
  items:
    - name: static-pv
      storage: 5Gi
      accessModes: [ReadWriteMany]
      storageClassName: nfs-client
      nfs:
        server: 203.0.113.5
        path: /volume1/nfs

persistentVolumeClaims:
  enabled: true
  items:
    - name: static-pvc
      accessModes: [ReadWriteMany]
      storageClassName: nfs-client
      storage: 5Gi
      mountPath: /web
      volumeName: static-pv
```

#### Mode 3: ConfigMap Content (Small Sites)

```yaml
content:
  enabled: true
  files:
    index.html: |
      <html><body><h1>Hello!</h1></body></html>
```

<br/>

### Extra Volumes

```yaml
extraVolumes:
  - name: extra-data
    configMap:
      name: my-configmap

extraVolumeMounts:
  - name: extra-data
    mountPath: /extra
    readOnly: true
```

<br/>

### GitLab Runner Cache Endpoint

Serves the runner's distributed cache (presigned S3 GET/HEAD/PUT) on a separate port and `<release>-s3` Service. See [docs/s3-cache.md](../../docs/s3-cache.md).

| Parameter | Description | Default |
|-----------|-------------|---------|
| `s3Cache.enabled` | Enable the cache endpoint | `false` |
| `s3Cache.port` | Container port | `9000` |
| `s3Cache.folder` | Writable object directory in the container | `/cache` |
| `s3Cache.existingSecret` | Secret holding the credentials (required) | `""` |
| `s3Cache.accessKeyKey` / `s3Cache.secretKeyKey` | Keys inside that Secret | `accessKey` / `secretKey` |
| `s3Cache.buckets` | Comma-separated bucket allowlist | `""` (any) |
| `s3Cache.ttl` | Expire objects neither uploaded nor read for this long (Go duration) | `""` (never) |
| `s3Cache.maxObjectSize` | Max upload size in bytes | `""` (unlimited) |
| `s3Cache.maxTotalSize` | Total size cap in bytes; least recently used objects are evicted above it | `""` (unlimited) |
| `s3Cache.existingClaim` | Existing PVC mounted read-write at `folder` (takes precedence over `persistence`) | `""` |
| `s3Cache.persistence.enabled` | Create a `<release>-s3-cache` PVC for the cache | `false` (emptyDir) |
| `s3Cache.persistence.storageClass` / `.accessMode` / `.size` | That PVC's class, access mode and size | `""` / `ReadWriteOnce` / `10Gi` |
| `s3Cache.subPath` | Sub-path of that PVC | `""` |
| `s3Cache.service.type` / `.port` / `.annotations` | Cache Service | `ClusterIP` / `9000` / `{}` |

If `existingClaim` is also a served content claim, the render fails unless `subPath` is a dot directory and `config.hideDotFiles` is `"true"`, so cache archives cannot leak through the file server.

<br/>

### Prometheus ServiceMonitor

Requires the Prometheus Operator CRDs and `config.metrics: "true"`; the render fails otherwise. Scrapes `/metrics` on the `http` port, and on the `s3` port when `s3Cache.enabled`.

| Parameter | Description | Default |
|---|---|---|
| `serviceMonitor.enabled` | Create a ServiceMonitor | `false` |
| `serviceMonitor.labels` | Extra labels, e.g. the label your Prometheus `serviceMonitorSelector` matches | `{}` |
| `serviceMonitor.interval` / `.scrapeTimeout` | Scrape interval and timeout | `""` (Prometheus defaults) |
| `serviceMonitor.tlsConfig` | Endpoint `tlsConfig` when `config.tlsCert` is set (scheme switches to `https`) | `{}` |

<br/>

## Examples

See the [examples/](examples/) directory for ready-to-use value files:

| Example | File | Description |
|---------|------|-------------|
| Dynamic provisioning | [dynamic-provisioning.yaml](examples/dynamic-provisioning.yaml) | StorageClass auto-creates PV |
| NFS static | [nfs-static.yaml](examples/nfs-static.yaml) | Manual NFS PV + PVC |
| HostPath | [hostpath-static.yaml](examples/hostpath-static.yaml) | Single-node / development |
| AWS EBS CSI | [csi-ebs.yaml](examples/csi-ebs.yaml) | AWS EBS volume |
| ConfigMap site | [configmap-site.yaml](examples/configmap-site.yaml) | Small static site from ConfigMap |
| Ingress + TLS | [ingress-tls.yaml](examples/ingress-tls.yaml) | cert-manager TLS termination |
| HTTPRoute (HTTP) | [httproute.yaml](examples/httproute.yaml) | Gateway API, HTTP listener only |
| HTTPRoute (HTTPS + redirect) | [httproute-https.yaml](examples/httproute-https.yaml) | Gateway API, HTTPS + sibling HTTP→HTTPS 301 |
| Multi-volume | [multi-volume.yaml](examples/multi-volume.yaml) | NFS + extra ConfigMap volume |
| Runner cache | [s3-cache.yaml](examples/s3-cache.yaml) | GitLab Runner cache endpoint on a chart-created claim |

<br/>

## Directory Listing UI Features

When `config.showListing` is enabled, the server provides a rich directory listing UI:

- Dark/light mode with system preference detection
- Grid/list view toggle with image thumbnail cards
- File type filter chips with counts
- Search with highlight and URL hash state
- Inline preview for images, video, audio, PDF, and text/code files
- Preview gallery navigation, image zoom, and line numbers
- Multi-file selection with batch ZIP download
- Column sorting with persistence
- Keyboard shortcuts (`/` search, `g` grid, `?` help, arrow keys navigation)
- Full accessibility support (ARIA attributes, focus trap)
