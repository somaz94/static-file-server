#!/usr/bin/env bash
set -euo pipefail

# test-helm.sh - Lint and template-test the Helm chart
#
# Usage:
#   ./hack/test-helm.sh
#   make test-helm

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHART_DIR="${ROOT_DIR}/helm/static-file-server"

echo "==> Helm chart tests"
echo ""

# 1. Lint
echo "--- Lint ---"
helm lint "${CHART_DIR}"
echo ""

# 2. Template render (default values)
echo "--- Template (default values) ---"
helm template test-release "${CHART_DIR}" > /dev/null
echo "  [OK] Default values render successfully"
echo ""

# 3. Template render (with ingress)
echo "--- Template (ingress enabled) ---"
helm template test-release "${CHART_DIR}" \
    --set ingress.enabled=true \
    --set "ingress.hosts[0].host=test.example.com" \
    --set "ingress.hosts[0].paths[0].path=/" \
    --set "ingress.hosts[0].paths[0].pathType=Prefix" > /dev/null
echo "  [OK] Ingress values render successfully"
echo ""

# 4. Template render (with persistence)
echo "--- Template (persistence enabled) ---"
helm template test-release "${CHART_DIR}" \
    --set persistence.enabled=true \
    --set persistence.size=5Gi > /dev/null
echo "  [OK] Persistence values render successfully"
echo ""

# 5. Template render (with configmap content)
echo "--- Template (configmap content) ---"
helm template test-release "${CHART_DIR}" \
    --set content.enabled=true \
    --set 'content.files.index\.html=<html>test</html>' > /dev/null
echo "  [OK] ConfigMap content values render successfully"
echo ""

# 6. Template render (static PV + PVC with selector)
echo "--- Template (static PV + NFS) ---"
helm template test-release "${CHART_DIR}" \
    --set persistentVolume.enabled=true \
    --set persistentVolume.storage=5Gi \
    --set persistentVolume.storageClass=nfs-client \
    --set persistentVolume.nfs.server=10.0.0.1 \
    --set persistentVolume.nfs.path=/exports/web \
    --set persistence.enabled=true \
    --set persistence.accessMode=ReadWriteMany \
    --set persistence.size=5Gi \
    --set persistence.storageClass=nfs-client > /dev/null
echo "  [OK] Static PV + NFS values render successfully"
echo ""

# 7. Template render (all options)
echo "--- Template (full options) ---"
helm template test-release "${CHART_DIR}" \
    --set replicaCount=3 \
    --set revisionHistoryLimit=1 \
    --set config.cors=true \
    --set config.debug=true \
    --set config.urlPrefix="/static" \
    --set service.type=NodePort \
    --set "extraEnv[0].name=CUSTOM" \
    --set "extraEnv[0].value=test" \
    --set "extraVolumes[0].name=extra" \
    --set "extraVolumes[0].emptyDir={}" \
    --set "extraVolumeMounts[0].name=extra" \
    --set "extraVolumeMounts[0].mountPath=/extra" > /dev/null
echo "  [OK] Full options render successfully"
echo ""

echo "==> All Helm chart tests passed!"
