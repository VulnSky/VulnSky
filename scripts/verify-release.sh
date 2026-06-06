#!/usr/bin/env bash
set -euo pipefail

go_cmd="${GO_EXE:-go}"

"$go_cmd" test ./...
"$go_cmd" vet ./...
module_path="$("$go_cmd" list -m)"
"$go_cmd" build \
  -trimpath \
  -buildvcs=false \
  -ldflags="-s -w -X ${module_path}/internal/version.Version=verify -X ${module_path}/internal/version.Commit=local -X ${module_path}/internal/version.BuildDate=local" \
  -o dist/vulnsky ./cmd/vulnsky
dist/vulnsky version | grep -F "Version=verify" >/dev/null

if [[ -d .git ]]; then
  paths=(
    ".env"
    ".env.local"
    "profiles/default.env"
    "vulnsky.db"
    "dist/vulnsky"
    "backend/README.md"
    "frontend/README.md"
  )
  for path in "${paths[@]}"; do
    git check-ignore --quiet --no-index "$path"
  done
fi

echo "release verification passed"
