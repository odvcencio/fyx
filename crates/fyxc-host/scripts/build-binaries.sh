#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
crate_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${crate_dir}/../.." && pwd)"
bin_dir="${crate_dir}/bin"

build_one() {
  local goos="$1"
  local goarch="$2"
  local out_dir="$3"
  local name="$4"

  mkdir -p "${bin_dir}/${out_dir}"
  echo "building ${out_dir}/${name}"
  (
    cd "${repo_root}"
    CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
      go build -trimpath -ldflags='-s -w' -o "${bin_dir}/${out_dir}/${name}" ./cmd/fyxc
  )
}

build_one linux amd64 linux-amd64 fyxc
build_one linux arm64 linux-arm64 fyxc
build_one darwin amd64 darwin-amd64 fyxc
build_one darwin arm64 darwin-arm64 fyxc
build_one windows amd64 windows-amd64 fyxc.exe
build_one windows arm64 windows-arm64 fyxc.exe
