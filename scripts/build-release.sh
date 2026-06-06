#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:-dist}"
go_cmd="${GO_EXE:-go}"
mkdir -p "$output_dir"
rm -f "${output_dir}"/vulnsky-* "${output_dir}"/SHA256SUMS

version="dev"
commit="none"
module_path="$("$go_cmd" list -m)"
if command -v git >/dev/null 2>&1; then
  version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
  commit="$(git rev-parse HEAD 2>/dev/null || echo none)"
fi
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w -X ${module_path}/internal/version.Version=${version} -X ${module_path}/internal/version.Commit=${commit} -X ${module_path}/internal/version.BuildDate=${build_date}"

write_checksums() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum vulnsky-*.zip vulnsky-*.tar.gz > SHA256SUMS
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 vulnsky-*.zip vulnsky-*.tar.gz > SHA256SUMS
    return
  fi
  echo "sha256sum or shasum is required to write SHA256SUMS" >&2
  exit 1
}

"$go_cmd" test ./...
"$go_cmd" vet ./...

targets=(
  "windows amd64 vulnsky-windows-amd64.exe"
  "windows arm64 vulnsky-windows-arm64.exe"
  "linux amd64 vulnsky-linux-amd64"
  "linux arm64 vulnsky-linux-arm64"
  "darwin amd64 vulnsky-darwin-amd64"
  "darwin arm64 vulnsky-darwin-arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch output <<< "$target"
  echo "building ${goos}/${goarch} -> ${output_dir}/${output}"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    "$go_cmd" build -trimpath -buildvcs=false -ldflags="${ldflags}" -o "${output_dir}/${output}" ./cmd/vulnsky
  if [[ "$goos" == "windows" ]]; then
    archive="${output%.exe}.zip"
    if command -v zip >/dev/null 2>&1; then
      (cd "$output_dir" && zip -9 "$archive" "$output" >/dev/null)
    else
      python3 - "$output_dir" "$archive" "$output" <<'PY'
import pathlib
import sys
import zipfile

out_dir, archive, binary = sys.argv[1:]
with zipfile.ZipFile(pathlib.Path(out_dir) / archive, "w", zipfile.ZIP_DEFLATED) as zf:
    zf.write(pathlib.Path(out_dir) / binary, binary)
PY
    fi
  else
    archive="${output}.tar.gz"
    tar -czf "${output_dir}/${archive}" -C "$output_dir" "$output"
  fi
  echo "packaged ${output_dir}/${archive}"
done

(
  cd "$output_dir"
  write_checksums
)
