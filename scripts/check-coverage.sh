#!/usr/bin/env bash
# Fails if total statement coverage in the given profile is below the threshold.
set -euo pipefail
profile="${1:-output/coverage.out}"
threshold="${2:-85}"
total=$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')
echo "total coverage: ${total}% (threshold ${threshold}%)"
awk -v t="$total" -v th="$threshold" 'BEGIN { exit (t+0 >= th+0) ? 0 : 1 }'
