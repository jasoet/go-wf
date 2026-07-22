#!/usr/bin/env bash
# Ensures go-wf releases only pin tagged releases of github.com/jasoet/pkg/v2.
set -euo pipefail
version=$(go list -m -f '{{.Version}}' github.com/jasoet/pkg/v2)
# Pseudo-versions look like v2.12.1-0.20260511023026-f8ae822218ab (timestamp+sha suffix).
if [[ "$version" =~ \.[0-9]{14}-[0-9a-f]{12}$ ]]; then
  echo "ERROR: github.com/jasoet/pkg/v2 is pinned to pseudo-version $version"
  echo "Tag a pkg/v2 release and 'go get github.com/jasoet/pkg/v2@latest' before releasing go-wf."
  exit 1
fi
echo "OK: github.com/jasoet/pkg/v2 pinned to tagged release $version"
