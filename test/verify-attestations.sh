#!/usr/bin/env bash
# Verify a build-provenance attestation exists for every release asset in dist/
# (archives, SBOMs, checksums.txt). Needs GH_TOKEN; used by ci.yml (push to
# main rehearsal) and release.yml (before the draft release is published).
set -euo pipefail

readonly dist_dir="dist"
shopt -s nullglob
subjects=("$dist_dir"/*.tar.gz "$dist_dir"/*.sbom.json "$dist_dir"/checksums.txt)

# 4 archives + 4 SBOMs + checksums.txt
if [[ ${#subjects[@]} -ne 9 ]]; then
  echo "expected 9 attestation subjects in $dist_dir, found ${#subjects[@]}" >&2
  exit 1
fi

for subject in "${subjects[@]}"; do
  gh attestation verify "$subject" --repo "$GITHUB_REPOSITORY"
done
