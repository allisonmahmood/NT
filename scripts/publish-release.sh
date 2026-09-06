#!/usr/bin/env bash
# Run from a trusted checkout with an authenticated maintainer gh login.
# Actions cannot read repository Administration settings, so publication lives here.
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 || ! $1 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || ! $2 =~ ^[0-9a-f]{40}$ ]]; then
  echo "usage: $0 vX.Y.Z EXPECTED_MAIN_SHA [--verify-only]" >&2
  exit 1
fi
if [[ $# -eq 3 && $3 != --verify-only ]]; then
  echo "unknown option: $3" >&2
  exit 1
fi
readonly tag="$1" expected_sha="$2" repo=allisonmahmood/NT
script_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

# Check the live setting with the maintainer credential, never a cached snapshot.
gh api "repos/$repo/immutable-releases" --jq .enabled | grep -qx true
ref_type="$(gh api "repos/$repo/git/ref/tags/$tag" --jq .object.type)"
[[ $ref_type == tag ]] || { echo 'release requires an annotated, signed tag' >&2; exit 1; }
tag_sha="$(gh api "repos/$repo/git/ref/tags/$tag" --jq .object.sha)"
gh api "repos/$repo/git/tags/$tag_sha" --jq '.verification.verified' | grep -qx true
tag_commit="$(gh api "repos/$repo/git/tags/$tag_sha" --jq '.object | select(.type == "commit") | .sha')"
[[ $tag_commit == "$expected_sha" ]] || { echo 'tag does not match the selected commit' >&2; exit 1; }
gh api "repos/$repo/compare/$expected_sha...main" --jq .status | grep -Eq '^(identical|ahead)$'

for workflow in ci.yml release.yml; do
  gh run list --repo "$repo" --workflow "$workflow" --commit "$expected_sha" \
    --event push --branch "$([[ $workflow == ci.yml ]] && echo main || echo "$tag")" \
    --limit 1 --json conclusion --jq '.[0].conclusion' | grep -qx success
done
gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft | grep -qx true
gh release view "$tag" --repo "$repo" --json assets --jq '.assets | length' | grep -qx 5

release_dir="$(mktemp -d)"
trap 'rm -rf "$release_dir"' EXIT
gh release download "$tag" --repo "$repo" --dir "$release_dir"
gh release view "$tag" --repo "$repo" --json body --jq .body > "$release_dir/CHANGELOG.md"
GITHUB_REPOSITORY="$repo" GITHUB_WORKFLOW_PATH=.github/workflows/release.yml \
  GITHUB_REF="refs/tags/$tag" GITHUB_SHA="$expected_sha" \
  "$script_root/test/verify-attestations.sh" "$release_dir"
"$script_root/test/release-candidate.sh" "${tag#v}" "$release_dir"

if [[ ${3:-} == --verify-only ]]; then
  echo "Verified draft $tag at $expected_sha; nothing published."
  exit 0
fi
# Recheck immediately before the irreversible transition.
gh api "repos/$repo/immutable-releases" --jq .enabled | grep -qx true
gh release edit "$tag" --repo "$repo" --draft=false
gh release verify "$tag" --repo "$repo"
for asset in "$release_dir"/*.tar.gz "$release_dir"/*.sbom.json "$release_dir/checksums.txt"; do
  gh release verify-asset "$tag" "$asset" --repo "$repo"
done
echo "Published and verified $tag at $expected_sha."
