#!/usr/bin/env bash
set -euo pipefail

# create-pr.sh - Create a pull request using gh CLI
#
# Usage:
#   ./scripts/create-pr.sh "PR title"
#   make pr title="PR title"

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 \"PR title\""
    exit 1
fi

TITLE="$1"
BRANCH="$(git branch --show-current)"
BASE="main"

if [[ "${BRANCH}" == "${BASE}" ]]; then
    echo "Error: Cannot create PR from ${BASE} branch"
    exit 1
fi

gh pr create \
    --title "${TITLE}" \
    --body "## Summary

- Branch: \`${BRANCH}\`

---
*Created with \`make pr\`*" \
    --base "${BASE}"
