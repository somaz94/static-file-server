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

# 11. Template render (S3 cache endpoint on a hidden subPath of the served claim)
echo "--- Template (s3 cache, shared claim) ---"
helm template test-release "${CHART_DIR}" \
    --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" \
    --set "persistentVolumeClaims.items[0].storage=5Gi" \
    --set config.hideDotFiles=true \
    --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=cache-creds \
    --set s3Cache.existingClaim=web \
    --set s3Cache.subPath=.runner-cache > /dev/null
echo "  [OK] S3 cache (shared claim, hidden subPath) render successfully"
echo ""

# 12. Template render (S3 cache on a chart-created PVC)
echo "--- Template (s3 cache, chart-created PVC) ---"
OUT=$(helm template test-release "${CHART_DIR}" \
    --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=cache-creds \
    --set s3Cache.persistence.enabled=true \
    --set s3Cache.persistence.storageClass=nfs-client)
grep -q "name: test-release-static-file-server-s3-cache" <<<"$OUT" || { echo "  [FAIL] s3 cache PVC not rendered"; exit 1; }
grep -q "claimName: test-release-static-file-server-s3-cache" <<<"$OUT" || { echo "  [FAIL] s3 cache PVC not mounted"; exit 1; }
echo "  [OK] S3 cache (chart-created PVC) render successfully"
echo ""

# 13. S3 cache guards must fail the render
echo "--- Template (s3 cache guards) ---"
if helm template test-release "${CHART_DIR}" --set s3Cache.enabled=true > /dev/null 2>&1; then
    echo "  [FAIL] s3Cache without existingSecret rendered"; exit 1
fi
if helm template test-release "${CHART_DIR}" \
    --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" \
    --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=cache-creds \
    --set s3Cache.existingClaim=web > /dev/null 2>&1; then
    echo "  [FAIL] s3Cache on the served claim without a hidden subPath rendered"; exit 1
fi
echo "  [OK] S3 cache guards reject unsafe values"
echo ""

# 14. ServiceMonitor: one endpoint per metrics port, and refused without metrics
echo "--- Template (servicemonitor) ---"
OUT=$(helm template test-release "${CHART_DIR}" \
    --set config.metrics=true \
    --set serviceMonitor.enabled=true \
    --set serviceMonitor.interval=30s \
    --set serviceMonitor.labels.release=kube-prometheus-stack \
    --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=cache-creds)
grep -q "kind: ServiceMonitor" <<<"$OUT" || { echo "  [FAIL] ServiceMonitor not rendered"; exit 1; }
grep -q -- "- port: s3" <<<"$OUT" || { echo "  [FAIL] s3 endpoint missing"; exit 1; }
grep -q "release: kube-prometheus-stack" <<<"$OUT" || { echo "  [FAIL] extra labels missing"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" --set config.metrics=true --set serviceMonitor.enabled=true)
if grep -q -- "- port: s3" <<<"$OUT"; then
    echo "  [FAIL] s3 endpoint rendered without s3Cache"; exit 1
fi
if helm template test-release "${CHART_DIR}" --set serviceMonitor.enabled=true > /dev/null 2>&1; then
    echo "  [FAIL] ServiceMonitor rendered without config.metrics"; exit 1
fi
echo "  [OK] ServiceMonitor render and metrics guard"
echo ""

# 15. Regression guards for chart bugs
echo "--- Template (regression guards) ---"
# A chart-created content PVC reused as the cache claim must be rejected.
if helm template test-release "${CHART_DIR}" --set persistence.enabled=true \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c \
    --set s3Cache.existingClaim=test-release-static-file-server-data > /dev/null 2>&1; then
    echo "  [FAIL] cache on the chart-created content claim rendered"; exit 1
fi
# "./cache" is not a dot directory.
if helm template test-release "${CHART_DIR}" --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" --set config.hideDotFiles=true \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c --set s3Cache.existingClaim=web \
    --set s3Cache.subPath=./cache > /dev/null 2>&1; then
    echo "  [FAIL] subPath ./cache accepted as hidden"; exit 1
fi
# An existingClaim that persistence does not mount is not served content.
helm template test-release "${CHART_DIR}" --set persistence.existingClaim=foo \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c --set s3Cache.existingClaim=foo > /dev/null \
    || { echo "  [FAIL] unmounted claim rejected as served"; exit 1; }
# ReadWriteOnce cache claim shared by replicas.
if helm template test-release "${CHART_DIR}" --set replicaCount=2 --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=c --set s3Cache.persistence.enabled=true > /dev/null 2>&1; then
    echo "  [FAIL] RWO cache claim with 2 replicas rendered"; exit 1
fi
OUT=$(helm template test-release "${CHART_DIR}" --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" --set "persistentVolumeClaims.items[0].readOnly=false")
grep -q "readOnly: false" <<<"$OUT" || { echo "  [FAIL] readOnly: false ignored"; exit 1; }
grep -q "targetPort: http" <<<"$OUT" || { echo "  [FAIL] Service targetPort is not the named port"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" --set config.tlsCert=/tls/tls.crt --set config.tlsKey=/tls/tls.key)
[ "$(grep -c "scheme: HTTPS" <<<"$OUT")" -eq 2 ] || { echo "  [FAIL] TLS probes must use HTTPS"; exit 1; }
if helm template test-release "${CHART_DIR}" --set httproute.enabled=true > /dev/null 2>&1; then
    echo "  [FAIL] HTTPRoute without a parentRef name rendered"; exit 1
fi
echo "  [OK] Regression guards hold"
echo ""

# 16. Cache-only mode: no file listener, Service, content volume or route
echo "--- Template (mode: cache) ---"
OUT=$(helm template test-release "${CHART_DIR}" -f "${CHART_DIR}/examples/runner-cache.yaml")
grep -q 'args: \["cache"\]' <<<"$OUT" || { echo "  [FAIL] cache args missing"; exit 1; }
[ "$(grep -c '^kind: Service$' <<<"$OUT")" -eq 1 ] || { echo "  [FAIL] expected only the s3 Service"; exit 1; }
if grep -q -E -- '- name: http$|name: web-content|- port: http' <<<"$OUT"; then
    echo "  [FAIL] file-server pieces rendered in cache mode"; exit 1
fi
[ "$(grep -c -E '^ +port: s3$' <<<"$OUT")" -eq 2 ] || { echo "  [FAIL] probes must target the s3 port"; exit 1; }
if helm template test-release "${CHART_DIR}" --set mode=cache > /dev/null 2>&1; then
    echo "  [FAIL] mode: cache without s3Cache rendered"; exit 1
fi
if helm template test-release "${CHART_DIR}" -f "${CHART_DIR}/examples/runner-cache.yaml" \
    --set httproute.enabled=true --set "httproute.parentRefs[0].name=gw" > /dev/null 2>&1; then
    echo "  [FAIL] mode: cache with an HTTPRoute rendered"; exit 1
fi
if helm template test-release "${CHART_DIR}" --set mode=bogus > /dev/null 2>&1; then
    echo "  [FAIL] unknown mode rendered"; exit 1
fi
echo "  [OK] Cache-only mode render and guards"
echo ""

# 17. Template render (all example files)
echo "--- Template (example values) ---"
for f in "${CHART_DIR}"/examples/*.yaml; do
    name=$(basename "$f" .yaml)
    helm template test-release "${CHART_DIR}" -f "$f" > /dev/null
    echo "  [OK] ${name}"
done
echo ""

TOTAL=$((16 + $(ls "${CHART_DIR}"/examples/*.yaml 2>/dev/null | wc -l | tr -d ' ')))
echo "==> All Helm chart tests passed! (${TOTAL} scenarios)"
