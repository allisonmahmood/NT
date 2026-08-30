# Security Policy

## Supported versions

Until NT has a tagged release, security fixes are made against the latest commit on `main`. After the first tagged release, only the latest release will receive security fixes; older releases will not be supported.

## Reporting a vulnerability

Do not report suspected vulnerabilities in a public issue, discussion, pull request, or other public channel.

Report them privately through [GitHub's private vulnerability reporting form](https://github.com/allisonmahmood/NT/security/advisories/new). Include:

- a description of the vulnerability and its impact;
- affected versions or commits;
- reproduction steps or a proof of concept; and
- any known mitigations or workarounds.

You can expect an acknowledgement within three business days. The maintainer will provide an initial assessment or request more information within ten business days, then send updates at least every ten business days while the report remains open. Resolution time depends on severity and complexity. The maintainer will coordinate disclosure and release details with the reporter; please keep the report private until a fix and disclosure are agreed.

## Deliberate Scorecard exceptions

- `main` requires pull requests, up-to-date status checks, and blocks deletion and force pushes. Review-count, stale-review, last-push, and code-owner requirements stay disabled because this repository has one maintainer; requiring that maintainer to approve their own change would not add an independent control. The explicit administrator bypass in `.github/rulesets/main.json` is the emergency path.
- Only the release job receives `contents: write`, because it must create the draft release, upload its assets, and publish it. `.github/workflows/release.yml` limits that permission to the job that needs it and verifies checksums, SBOMs, and build attestations before the immutable release becomes public.
- Scorecard's `Signed-Releases` check does not recognize provenance stored in GitHub's Attestations API. NT deliberately uses GitHub build attestations instead of separate signature files attached to each release: `.github/immutable-releases.json` records the enabled immutability policy, and the release workflow attests every archive, SBOM, and checksum file before publishing.
