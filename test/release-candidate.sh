#!/usr/bin/env bash
# Verify cross-compiled metadata, per-archive SBOMs, and execute the native release candidate.
set -euo pipefail

readonly dist_dir="${2:-dist}"
readonly expected_version="${1:-0.1.0-snapshot}"
readonly checksum_file="$dist_dir/checksums.txt"
readonly targets=(linux_amd64 linux_arm64)
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

  sbom="$archive.sbom.json"
  if [[ ! -f "$sbom" ]]; then
    echo "missing SBOM: $sbom" >&2
    exit 1
  fi
  if ! grep -q '"spdxVersion"' "$sbom"; then
    echo "SBOM is not SPDX JSON: $sbom" >&2
    exit 1
  fi
  if ! grep -q "  $(basename "$sbom")\$" "$checksum_file"; then
    echo "SBOM not listed in checksum manifest: $sbom" >&2
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

case "$(uname -m)" in
  x86_64) native_target=linux_amd64 ;;
  aarch64|arm64) native_target=linux_arm64 ;;
  *) echo "unsupported verification host architecture" >&2; exit 1 ;;
esac
reported_version="$("$extract_root/$native_target/nt" --version)"
if [[ "$reported_version" != "nt version $expected_version" ]]; then
  echo "binary reported unexpected version: $reported_version" >&2
  exit 1
fi
