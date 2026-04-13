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

# 6. Template render (all options)
echo "--- Template (full options) ---"
helm template test-release "${CHART_DIR}" \
    --set replicaCount=3 \
    --set config.cors=true \
    --set config.debug=true \
    --set config.urlPrefix="/static" \
    --set service.type=NodePort \
    --set "extraEnv[0].name=CUSTOM" \
    --set "extraEnv[0].value=test" > /dev/null
echo "  [OK] Full options render successfully"
echo ""

echo "==> All Helm chart tests passed!"
