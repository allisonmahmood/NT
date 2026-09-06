# Linux releases

Supported release archives: Linux amd64 and arm64. macOS is deferred in
[#23](https://github.com/allisonmahmood/NT/issues/23); its former packaging and CI
remain in Git history at `89e436deed75d4ded3df9573ee6b6529c9491a33`. AUR submission
and Homebrew are not prerequisites for Linux releases.

## Prepare

1. Merge the release changes and wait for the complete `ci` and CodeQL runs on
   the selected `main` commit. The main-only `attest-snapshot` job rehearses
   provenance; PR builds have read-only permissions.
2. Complete the maintainer confirmations in #18 before the first release.
3. Select the full commit SHA. Using the maintainer's configured signing identity,
   create `git tag -s v0.1.0 FULL_MAIN_SHA -m 'nt v0.1.0'`, inspect it with
   `git show v0.1.0` and `git verify-tag v0.1.0`, then push
   `git push origin refs/tags/v0.1.0`. GitHub must recognize the tag signature.
   Tags are protected against rewriting and deletion: do not push a rehearsal tag.

## Verify and publish

The tag workflow builds Linux archives, checks their checksums, SPDX SBOMs and
version metadata, then attests every asset and verifies its provenance. It leaves
an unpublished draft. A failed run must be investigated before proceeding.

From a trusted checkout on a Linux machine with Go, Git, and an authenticated
maintainer `gh` login:

```sh
scripts/publish-release.sh v0.1.0 FULL_MAIN_SHA --verify-only
scripts/publish-release.sh v0.1.0 FULL_MAIN_SHA
```

The command checks the live repository immutability setting, GitHub's validation
of the signed tag, its exact commit, main ancestry, successful CI and release
runs, the draft asset set, and the downloaded binaries and attestations. Only
then does it publish and verify GitHub's immutable release and asset attestations.
It runs the binary for the local architecture; exercise the other architecture
on native hardware when available.

This boundary deliberately uses the maintainer's existing CLI authentication.
The [immutability setting API](https://docs.github.com/en/rest/repos/repos#check-if-immutable-releases-are-enabled-for-a-repository)
requires Administration read, which `GITHUB_TOKEN` cannot obtain. No personal
access token is stored in Actions. Immutability is checked again immediately
before publication; do not change release settings or edit draft assets while
publication is running.

## Finish installation validation

- Follow the README's archive installation and shell setup in a clean directory.
- For the first release, finish #42: replace all `SKIP` hashes in both Arch recipes
  with hashes of the verified published inputs; regenerate both `.SRCINFO` files.
  Build both recipes from fresh downloads with `makepkg`, inspect with `namcap`,
  and confirm `nt --version`. Keep #34 open until these checks pass.
- Arch CI exercises HEAD with a temporary recipe and its own computed checksum;
  separately test the unmodified release recipes against published downloads.
- Exercise create, cd, home, completion, dirty-worktree refusal, and removal
  through the installed shell hook. Confirm update and removal instructions.
- Verify GitHub has no unresolved dependency, code-scanning or secret-scanning
  alerts. Classify advisory Scorecard findings against `SECURITY.md`.
- Remove the README's pre-release notice and close #26 only after publication
  and installation verification are complete.
