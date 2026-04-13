# Version Management & Release Process

<br/>

## Version Locations

Version is tracked in the following files:

| File | Field | Format |
|------|-------|--------|
| `Makefile` | `IMG` | `somaz940/static-file-server:v0.1.0` |
| `helm/static-file-server/Chart.yaml` | `version` | `0.1.0` (without `v`) |
| `helm/static-file-server/Chart.yaml` | `appVersion` | `v0.1.0` |
| `helm/static-file-server/values.yaml` | `image.tag` | `v0.1.0` |

<br/>

## Check Current Version

```bash
make version
```

Output:
```
Current version: v0.1.0

Version in each file:
  Makefile:                 v0.1.0
  Chart.yaml (version):     0.1.0
  Chart.yaml (appVersion):  v0.1.0
  values.yaml (image.tag):  v0.1.0
```

<br/>

## Bump Version

Update all files at once:

```bash
make bump-version VERSION=v0.2.0
```

This updates:
- `Makefile` IMG tag
- `Chart.yaml` version + appVersion
- `values.yaml` image.tag
- `README.md` version references

<br/>

## Release Process

### 1. Bump version and commit

```bash
make bump-version VERSION=v0.2.0
git diff                                    # review changes
git commit -am "chore: bump version to v0.2.0"
git push origin main
```

### 2. Build and push Docker image

```bash
make docker-buildx                          # builds + pushes versioned + latest tags
```

### 3. Create git tag

```bash
git tag v0.2.0
git push origin v0.2.0
```

This triggers the following CI workflows:
- **release.yml**: Runs tests → Docker multi-arch build+push → GitHub Release
- **helm-release.yml**: Package Helm chart → publish to gh-pages

### 4. Verify

```bash
# Docker image
docker pull somaz940/static-file-server:v0.2.0

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
