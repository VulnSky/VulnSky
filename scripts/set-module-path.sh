#!/usr/bin/env bash
set -euo pipefail

module_path="${1:-}"
if [[ -z "$module_path" || "$module_path" =~ [[:space:]] || "$module_path" != */* ]]; then
  echo "usage: $0 github.com/<owner>/vulnsky" >&2
  exit 2
fi

go_cmd="${GO_EXE:-go}"
"$go_cmd" mod edit -module="$module_path"

while IFS= read -r file; do
  perl -0pi -e "s#\"vulnsky/([A-Za-z0-9_./-]+)\"#\"${module_path}/\${1}\"#g" "$file"
done < <(find cmd internal -name '*.go' -type f)

"$go_cmd" mod tidy
"$go_cmd" test ./...

echo "module path updated to ${module_path}"
