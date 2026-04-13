# Version Management & Release Process

<br/>

## Version Locations

Version is tracked in the following files:

| File | Field | Format |
|------|-------|--------|
| `Makefile` | `IMG` | `somaz940/static-file-server:v0.4.0` |
| `helm/static-file-server/Chart.yaml` | `version` | `0.4.0` (without `v`) |
| `helm/static-file-server/Chart.yaml` | `appVersion` | `v0.4.0` |
| `helm/static-file-server/values.yaml` | `image.tag` | `v0.4.0` |
| `deploy/deployment.yaml` | `image` | `somaz940/static-file-server:v0.4.0` |
| `deploy/helmfile/helmfile.yaml` | `version` | `0.4.0` (without `v`) |
| `deploy/helmfile/values/mgmt.yaml` | `image.tag` | `v0.4.0` |

<br/>

## Check Current Version

```bash
make version
```

Output:
```
Current version: v0.4.0

Version in each file:
  Makefile:                           v0.4.0
  Chart.yaml (version):               0.4.0
  Chart.yaml (appVersion):            v0.4.0
  values.yaml (image.tag):            v0.4.0
  deployment.yaml (image):            v0.4.0
  helmfile.yaml (version):            0.4.0
  helmfile mgmt.yaml (image.tag):     v0.4.0
```

<br/>

## Bump Version

Update all files at once:

```bash
make bump-version VERSION=v0.4.0
```

This updates:
- `Makefile` IMG tag
- `Chart.yaml` version + appVersion
- `values.yaml` image.tag
- `deploy/deployment.yaml` image tag
- `deploy/helmfile/helmfile.yaml` version
- `deploy/helmfile/values/mgmt.yaml` image.tag
- `README.md` version references

<br/>

## Release Process

### 1. Bump version and commit

```bash
make bump-version VERSION=v0.4.0
git diff                                    # review changes
git commit -am "chore: bump version to v0.4.0"
git push origin main
```

### 2. Build and push Docker image

```bash
make docker-buildx                          # builds + pushes versioned + latest tags
```

### 3. Create git tag

```bash
git tag v0.4.0
git push origin v0.4.0
```

This triggers the following CI workflows:
- **release.yml**: Runs tests → Docker multi-arch build+push → GitHub Release
- **helm-release.yml**: Package Helm chart → publish to gh-pages

### 4. Verify

```bash
# Docker image
docker pull somaz940/static-file-server:v0.4.0

# Helm chart
helm repo update
helm search repo static-file-server
```

<br/>

## Development Workflow

### Feature branch

```bash
make branch name=search-filter              # creates feat/search-filter
# ... develop ...
make pr title="Add search filter"           # test + push + create PR
```

### Pre-flight checks

```bash
make test                                   # all tests pass
make test-helm                              # Helm chart lint + render
make lint                                   # golangci-lint
make version                                # versions consistent
```
