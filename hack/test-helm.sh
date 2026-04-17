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

# 4. Template render (dynamic provisioning)
echo "--- Template (dynamic provisioning) ---"
helm template test-release "${CHART_DIR}" \
    --set persistence.enabled=true \
    --set persistence.storageClass=standard \
    --set persistence.size=5Gi > /dev/null
echo "  [OK] Dynamic provisioning render successfully"
echo ""

# 5. Template render (static provisioning - NFS PV + PVC items)
echo "--- Template (static provisioning - NFS) ---"
helm template test-release "${CHART_DIR}" \
    --set persistentVolumes.enabled=true \
    --set "persistentVolumes.items[0].name=test-pv" \
    --set "persistentVolumes.items[0].storage=5Gi" \
    --set "persistentVolumes.items[0].accessModes[0]=ReadWriteMany" \
    --set "persistentVolumes.items[0].reclaimPolicy=Retain" \
    --set "persistentVolumes.items[0].storageClassName=nfs-client" \
    --set "persistentVolumes.items[0].nfs.server=10.0.0.1" \
    --set "persistentVolumes.items[0].nfs.path=/exports/web" \
    --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=test-pvc" \
    --set "persistentVolumeClaims.items[0].accessModes[0]=ReadWriteMany" \
    --set "persistentVolumeClaims.items[0].storageClassName=nfs-client" \
    --set "persistentVolumeClaims.items[0].storage=5Gi" \
    --set "persistentVolumeClaims.items[0].mountPath=/web" \
    --set "persistentVolumeClaims.items[0].volumeName=test-pv" > /dev/null
echo "  [OK] Static provisioning NFS render successfully"
echo ""

# 6. Template render (static provisioning - hostPath)
echo "--- Template (static provisioning - hostPath) ---"
helm template test-release "${CHART_DIR}" \
    --set persistentVolumes.enabled=true \
    --set "persistentVolumes.items[0].name=local-pv" \
    --set "persistentVolumes.items[0].storage=10Gi" \
    --set "persistentVolumes.items[0].accessModes[0]=ReadWriteOnce" \
    --set "persistentVolumes.items[0].hostPath.path=/data/web" \
    --set "persistentVolumes.items[0].hostPath.type=DirectoryOrCreate" > /dev/null
echo "  [OK] Static provisioning hostPath render successfully"
echo ""

# 7. Template render (configmap content)
echo "--- Template (configmap content) ---"
helm template test-release "${CHART_DIR}" \
    --set content.enabled=true \
    --set 'content.files.index\.html=<html>test</html>' > /dev/null
echo "  [OK] ConfigMap content render successfully"
echo ""

# 8. Template render (full options with extraVolumes)
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

# 9. Template render (httproute - basic HTTP)
echo "--- Template (httproute - HTTP) ---"
helm template test-release "${CHART_DIR}" \
    --set httproute.enabled=true \
    --set "httproute.parentRefs[0].name=test-gateway" \
    --set "httproute.parentRefs[0].namespace=gateway-system" \
    --set-json 'httproute.hostnames=["test.example.com"]' \
    --set "httproute.rules[0].matches[0].path.type=PathPrefix" \
    --set "httproute.rules[0].matches[0].path.value=/" \
    --set "httproute.rules[0].backendRefs[0].port=80" > /dev/null
echo "  [OK] HTTPRoute (HTTP) render successfully"
echo ""

# 10. Template render (httproute - HTTPS with HTTP→HTTPS redirect sibling)
echo "--- Template (httproute - HTTPS + redirect) ---"
helm template test-release "${CHART_DIR}" \
    --set httproute.enabled=true \
    --set "httproute.parentRefs[0].name=test-gateway" \
    --set "httproute.parentRefs[0].namespace=gateway-system" \
    --set "httproute.parentRefs[0].sectionName=https" \
    --set-json 'httproute.hostnames=["test.example.com"]' \
    --set "httproute.rules[0].matches[0].path.type=PathPrefix" \
    --set "httproute.rules[0].matches[0].path.value=/" \
    --set "httproute.rules[0].backendRefs[0].port=80" \
    --set httproute.httpsRedirect.enabled=true \
    --set httproute.httpsRedirect.statusCode=301 \
    --set "httproute.httpsRedirect.parentRefs[0].name=test-gateway" \
    --set "httproute.httpsRedirect.parentRefs[0].namespace=gateway-system" \
    --set "httproute.httpsRedirect.parentRefs[0].sectionName=http" > /dev/null
echo "  [OK] HTTPRoute (HTTPS + redirect) render successfully"
echo ""

# 11. Template render (all example files)
echo "--- Template (example values) ---"
for f in "${CHART_DIR}"/examples/*.yaml; do
    name=$(basename "$f" .yaml)
    helm template test-release "${CHART_DIR}" -f "$f" > /dev/null
    echo "  [OK] ${name}"
done
echo ""

TOTAL=$((10 + $(ls "${CHART_DIR}"/examples/*.yaml 2>/dev/null | wc -l | tr -d ' ')))
echo "==> All Helm chart tests passed! (${TOTAL} scenarios)"
