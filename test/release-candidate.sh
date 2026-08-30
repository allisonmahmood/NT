#!/usr/bin/env bash
# Verify the complete snapshot artifact set without executing cross-compiled binaries.
set -euo pipefail

readonly dist_dir="${1:-dist}"
readonly expected_version="${2:-0.1.0-snapshot}"
readonly checksum_file="$dist_dir/checksums.txt"
readonly targets=(darwin_amd64 darwin_arm64 linux_amd64 linux_arm64)
readonly changelog_file="$dist_dir/CHANGELOG.md"

if [[ ! -f "$checksum_file" ]]; then
  echo "missing checksum manifest: $checksum_file" >&2
  exit 1
fi

if [[ ! -s "$changelog_file" ]]; then
  echo "missing generated changelog: $changelog_file" >&2
  exit 1
fi

(
  cd "$dist_dir"
  sha256sum --check "$(basename "$checksum_file")"
)

readonly extract_root="$(mktemp -d)"
trap 'rm -rf "$extract_root"' EXIT

for target in "${targets[@]}"; do
  archive="$dist_dir/nt_${expected_version}_${target}.tar.gz"
  if [[ ! -f "$archive" ]]; then
    echo "missing release archive: $archive" >&2
    exit 1
  fi

  target_dir="$extract_root/$target"
  mkdir "$target_dir"
  tar -xzf "$archive" -C "$target_dir"

  binary="$target_dir/nt"
  if [[ ! -x "$binary" ]]; then
    echo "archive does not contain an executable nt binary: $archive" >&2
    exit 1
  fi

  metadata="$(go version -m "$binary")"
  if [[ "$metadata" != *"-X main.version=$expected_version"* ]]; then
    echo "binary has unexpected version metadata: $archive" >&2
    exit 1
  fi
done

readonly reported_version="$("$extract_root/linux_amd64/nt" --version)"
if [[ "$reported_version" != "nt version $expected_version" ]]; then
  echo "binary reported unexpected version: $reported_version" >&2
  exit 1
fi
