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

# expect_fail <error text> <helm template args...>: the render must fail, and for the expected reason.
expect_fail() {
    local want=$1 err
    shift
    if err=$(helm template test-release "${CHART_DIR}" "$@" 2>&1 >/dev/null); then
        echo "  [FAIL] rendered but should not: $*"; exit 1
    fi
    grep -qF -- "$want" <<<"$err" || { echo "  [FAIL] failed for the wrong reason ($*): $err"; exit 1; }
}

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
expect_fail 'existingSecret is required' --set s3Cache.enabled=true
expect_fail 'also served as content' \
    --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" \
    --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=cache-creds \
    --set s3Cache.existingClaim=web
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
expect_fail 'requires config.metrics' --set serviceMonitor.enabled=true
echo "  [OK] ServiceMonitor render and metrics guard"
echo ""

# 15. Regression guards for chart bugs
echo "--- Template (regression guards) ---"
# A chart-created content PVC reused as the cache claim must be rejected.
expect_fail 'also served as content' --set persistence.enabled=true \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c \
    --set s3Cache.existingClaim=test-release-static-file-server-data
# "./cache" is not a dot directory.
expect_fail 'also served as content' --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" --set config.hideDotFiles=true \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c --set s3Cache.existingClaim=web \
    --set s3Cache.subPath=./cache
# An existingClaim that persistence does not mount is not served content.
helm template test-release "${CHART_DIR}" --set persistence.existingClaim=foo \
    --set s3Cache.enabled=true --set s3Cache.existingSecret=c --set s3Cache.existingClaim=foo > /dev/null \
    || { echo "  [FAIL] unmounted claim rejected as served"; exit 1; }
# ReadWriteOnce cache claim shared by replicas.
expect_fail 'cannot be shared by replicaCount' --set replicaCount=2 --set s3Cache.enabled=true \
    --set s3Cache.existingSecret=c --set s3Cache.persistence.enabled=true
OUT=$(helm template test-release "${CHART_DIR}" --set persistentVolumeClaims.enabled=true \
    --set "persistentVolumeClaims.items[0].name=web" --set "persistentVolumeClaims.items[0].readOnly=false")
grep -q "readOnly: false" <<<"$OUT" || { echo "  [FAIL] readOnly: false ignored"; exit 1; }
grep -q "targetPort: http" <<<"$OUT" || { echo "  [FAIL] Service targetPort is not the named port"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" --set config.tlsCert=/tls/tls.crt --set config.tlsKey=/tls/tls.key)
[ "$(grep -c "scheme: HTTPS" <<<"$OUT")" -eq 2 ] || { echo "  [FAIL] TLS probes must use HTTPS"; exit 1; }
expect_fail 'parentRefs[].name is required' --set httproute.enabled=true
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
expect_fail 'requires s3Cache.enabled' --set mode=cache
expect_fail 'serves no files' -f "${CHART_DIR}/examples/runner-cache.yaml" \
    --set httproute.enabled=true --set "httproute.parentRefs[0].name=gw"
expect_fail 'mode must be' --set mode=bogus
echo "  [OK] Cache-only mode render and guards"
echo ""

# 17. Scheduling and hardening: PDB, topology spread, strategy, token and seccomp defaults
echo "--- Template (scheduling and hardening) ---"
OUT=$(helm template test-release "${CHART_DIR}" --skip-tests)
[ "$(grep -c 'automountServiceAccountToken: false' <<<"$OUT")" -eq 2 ] || { echo "  [FAIL] token must not be mounted by default"; exit 1; }
grep -q 'type: RuntimeDefault' <<<"$OUT" || { echo "  [FAIL] seccomp RuntimeDefault missing"; exit 1; }
if grep -q -E 'kind: PodDisruptionBudget|topologySpreadConstraints:|^  strategy:' <<<"$OUT"; then
    echo "  [FAIL] opt-in scheduling settings rendered by default"; exit 1
fi
OUT=$(helm template test-release "${CHART_DIR}" --set podDisruptionBudget.enabled=true \
    --set-json 'topologySpreadConstraints=[{"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"ScheduleAnyway"}]' \
    --set-json 'strategy={"rollingUpdate":{"maxUnavailable":0}}')
grep -q 'kind: PodDisruptionBudget' <<<"$OUT" && grep -q 'maxUnavailable: 1' <<<"$OUT" || { echo "  [FAIL] PDB missing"; exit 1; }
grep -q 'app.kubernetes.io/instance: test-release' <<<"$(grep -A4 'labelSelector:' <<<"$OUT")" || { echo "  [FAIL] spread must default to this release's pods"; exit 1; }
grep -q 'maxUnavailable: 0' <<<"$OUT" || { echo "  [FAIL] strategy missing"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" --set replicaCount=2 --set podDisruptionBudget.enabled=true --set podDisruptionBudget.minAvailable=1)
grep -q 'minAvailable: 1' <<<"$OUT" && ! grep -q 'maxUnavailable' <<<"$OUT" || { echo "  [FAIL] minAvailable must replace maxUnavailable"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" \
    --set-json 'topologySpreadConstraints=[{"maxSkew":1,"topologyKey":"zone","whenUnsatisfiable":"DoNotSchedule","labelSelector":{"matchLabels":{"custom":"x"}}}]')
if grep -q 'app.kubernetes.io/instance' <<<"$(grep -A4 'labelSelector:' <<<"$OUT")"; then
    echo "  [FAIL] an explicit labelSelector must be kept as given"; exit 1
fi
expect_fail 'blocks every node drain' --set podDisruptionBudget.enabled=true --set podDisruptionBudget.minAvailable=1
expect_fail 'blocks every node drain' --set podDisruptionBudget.enabled=true --set podDisruptionBudget.maxUnavailable=0
OUT=$(helm template test-release "${CHART_DIR}" --set s3Cache.enabled=true --set s3Cache.existingSecret=c --set s3Cache.persistence.enabled=true)
grep -q 'type: Recreate' <<<"$OUT" || { echo "  [FAIL] a ReadWriteOnce cache claim must default to Recreate"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" --set s3Cache.enabled=true --set s3Cache.existingSecret=c \
    --set s3Cache.persistence.enabled=true --set-json 'strategy={"type":"RollingUpdate"}')
grep -q 'type: RollingUpdate' <<<"$OUT" || { echo "  [FAIL] an explicit strategy must win"; exit 1; }
grep -q 'terminationGracePeriodSeconds: 0' <<<"$(helm template test-release "${CHART_DIR}" --set terminationGracePeriodSeconds=0)" \
    || { echo "  [FAIL] terminationGracePeriodSeconds 0 dropped"; exit 1; }
echo "  [OK] Scheduling and hardening"
echo ""

# 18. Optional pod knobs, the ingress/httproute guard and the test hook
echo "--- Template (pod knobs, route guard, test hook) ---"
OUT=$(helm template test-release "${CHART_DIR}" --set priorityClassName=high \
    --set terminationGracePeriodSeconds=60 --set probes.liveness.timeoutSeconds=2 \
    --set probes.readiness.failureThreshold=5)
for want in 'priorityClassName: high' 'terminationGracePeriodSeconds: 60' 'timeoutSeconds: 2' 'failureThreshold: 5'; do
    grep -q "$want" <<<"$OUT" || { echo "  [FAIL] missing $want"; exit 1; }
done
OUT=$(helm template test-release "${CHART_DIR}")
if grep -q -E 'priorityClassName|terminationGracePeriodSeconds|timeoutSeconds|failureThreshold' <<<"$OUT"; then
    echo "  [FAIL] optional pod knobs rendered by default"; exit 1
fi
TEST_POD=$(helm template test-release "${CHART_DIR}" -s templates/tests/test-connection.yaml)
grep -q 'hook-delete-policy' <<<"$TEST_POD" && grep -q '/healthz' <<<"$TEST_POD" || { echo "  [FAIL] test hook incomplete"; exit 1; }
if grep -q 'app.kubernetes.io/name:' <<<"$TEST_POD"; then
    echo "  [FAIL] test pod carries the Service selector labels"; exit 1
fi
expect_fail 'not both' --set ingress.enabled=true --set httproute.enabled=true \
    --set "httproute.parentRefs[0].name=gw"
TEST_POD=$(helm template test-release "${CHART_DIR}" -s templates/tests/test-connection.yaml \
    --set "imagePullSecrets[0].name=reg" --set nodeSelector.disk=ssd --set "tolerations[0].key=dedicated")
for want in 'name: reg' 'disk: ssd' 'key: dedicated'; do
    grep -q "$want" <<<"$TEST_POD" || { echo "  [FAIL] test pod missing $want"; exit 1; }
done
OUT=$(helm template test-release "${CHART_DIR}" -s templates/deployment.yaml)
grep -q 'enableServiceLinks: false' <<<"$OUT" || { echo "  [FAIL] service links not disabled by default"; exit 1; }
OUT=$(helm template test-release "${CHART_DIR}" -s templates/deployment.yaml \
    --set resources=null --set securityContext=null --set podSecurityContext=null)
if grep -q -E '^ *(resources|securityContext):' <<<"$OUT"; then
    echo "  [FAIL] null resources / securityContext still rendered"; exit 1
fi
echo "  [OK] Pod knobs, route guard and test hook"
echo ""

# 19. values.schema.json rejects what the server would refuse at startup.
# Match field paths only: the validator's wording changes between Helm versions.
echo "--- Values schema ---"
expect_fail 'bogus' --set bogus=1
expect_fail 'config.debug' --set config.debug=on
expect_fail 'config.port' --set config.port=abc
expect_fail 's3Cache.ttl' --set s3Cache.ttl=7d
expect_fail 's3Cache.maxTotalSize' --set s3Cache.maxTotalSize=50Gi
# Unquoted, Helm renders a large size as 5.36870912e+10, which the server rejects.
expect_fail 's3Cache.maxTotalSize' --set s3Cache.maxTotalSize=53687091200
expect_fail 'persistence.accessMode' --set persistence.accessMode=RWX
expect_fail 'config.port' --set-string config.port=99999
helm template test-release "${CHART_DIR}" --set config.debug=true --set config.port=8080 --set image.tag=20261020 \
    --set service.targetPort=8080 --set enabled=true \
    --set-string s3Cache.maxTotalSize=53687091200 --set s3Cache.ttl=336h > /dev/null
echo "  [OK] Schema accepts booleans, numeric ports and quoted sizes; rejects the rest"
echo ""

# 20. Template render (all example files)
echo "--- Template (example values) ---"
for f in "${CHART_DIR}"/examples/*.yaml; do
    name=$(basename "$f" .yaml)
    helm template test-release "${CHART_DIR}" -f "$f" > /dev/null
    echo "  [OK] ${name}"
done
echo ""

TOTAL=$((19 + $(ls "${CHART_DIR}"/examples/*.yaml 2>/dev/null | wc -l | tr -d ' ')))
echo "==> All Helm chart tests passed! (${TOTAL} scenarios)"
