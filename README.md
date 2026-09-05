# nt — navigate tree

Git worktrees, minus the ceremony. `nt` (short for **navigate tree**) is a tiny
cross-shell command for hopping around worktrees: it spins one up — or jumps to
it if it already exists — `cd`s you in, and gets out of your way.

```sh
nt fix-login   # worktree up, cd'd in, go
```

A single Go binary plus a one-line shell hook. Works in **zsh, bash, and fish**.

## Where the trees live

Worktrees sit right next to the main checkout in **`<repo>.worktrees/<branch>`** —
no scattering them across `/tmp`, no losing track of where they went:

```
~/Developer/
  acme/                       <- the main checkout
  acme.worktrees/
    fix-login/                <- nt fix-login
    team/issue-123-thing/     <- nt team/issue-123-thing
```

## Install

The first release will be **v0.1.0**. It is not published yet, so the pinned
release commands below will work only after it is published.

### Go toolchain

With Go 1.25 or newer:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/allisonmahmood/nt@v0.1.0
go version -m "$HOME/.local/bin/nt" | grep 'mod.*github.com/allisonmahmood/nt.*v0.1.0'
```

Until v0.1.0 is published, build the current source instead:

```sh
mkdir -p "$HOME/.local/bin" && git clone https://github.com/allisonmahmood/NT nt && cd nt && go build -o "$HOME/.local/bin/nt" .
```

Go-toolchain builds report `nt version dev`; `go version -m` verifies the tagged
module version. GitHub build attestations cover release archives, not local
builds.

### Arch Linux

The checked-in [`nt`](packaging/arch/nt/PKGBUILD) package builds the tagged
source, while [`nt-bin`](packaging/arch/nt-bin/PKGBUILD) installs the prebuilt
Linux archive and verifies it against the release's `checksums.txt`.

```sh
git clone https://github.com/allisonmahmood/NT
cd NT/packaging/arch/nt       # or NT/packaging/arch/nt-bin for the prebuilt archive
makepkg -si
nt --version
```

These commands require the v0.1.0 release. Publishing both packages to the AUR
is pending; until then, build them from these repository paths.

### Release archive

These commands require an authenticated [`gh`](https://cli.github.com/). Paste
the complete block into bash or zsh. It stops on any failed check without changing
your shell's error-handling settings.

#### Linux

```sh
(
set -euo pipefail
version=v0.1.0
platform="linux_$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
archive="nt_${version#v}_${platform}.tar.gz"
mkdir -p "nt-$version-$platform"
cd "nt-$version-$platform"
gh release verify "$version" --repo allisonmahmood/NT
gh release download "$version" --repo allisonmahmood/NT \
  --pattern "$archive" --pattern checksums.txt
gh release verify-asset "$version" "$archive" --repo allisonmahmood/NT
awk -v archive="$archive" '$2 == archive' checksums.txt | sha256sum --check
gh attestation verify "$archive" --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
gh attestation verify checksums.txt --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
tar -xzf "$archive"
reported_version="$(./nt --version)"
test "$reported_version" = "nt version ${version#v}"
mkdir -p "$HOME/.local/bin"
install -m 0755 nt "$HOME/.local/bin/nt"
)
```

#### macOS

```sh
(
set -euo pipefail
version=v0.1.0
platform="darwin_$(uname -m | sed 's/x86_64/amd64/')"
archive="nt_${version#v}_${platform}.tar.gz"
mkdir -p "nt-$version-$platform"
cd "nt-$version-$platform"
gh release verify "$version" --repo allisonmahmood/NT
gh release download "$version" --repo allisonmahmood/NT \
  --pattern "$archive" --pattern checksums.txt
gh release verify-asset "$version" "$archive" --repo allisonmahmood/NT
awk -v archive="$archive" '$2 == archive' checksums.txt | shasum -a 256 --check
gh attestation verify "$archive" --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
gh attestation verify checksums.txt --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
tar -xzf "$archive"
reported_version="$(./nt --version)"
test "$reported_version" = "nt version ${version#v}"
mkdir -p "$HOME/.local/bin"
install -m 0755 nt "$HOME/.local/bin/nt"
)
```

A v0.1.0 archive must report `nt version 0.1.0`. Archives support Linux and
macOS on x86-64 (`amd64`) and ARM64 (`arm64`); Windows is not supported.

`gh release verify` confirms GitHub's signed, immutable release;
`verify-asset` ties the download to its exact release asset; and the checksum
manifest verifies its bytes. `gh attestation verify` requires provenance from
this repository's release workflow and the selected version tag. Each archive
also has an SPDX SBOM named `$archive.sbom.json`. These checks prove origin and
integrity, not code safety.

The macOS archives are not code-signed or notarized. A Homebrew tap
([#28](https://github.com/allisonmahmood/nt/issues/28)) is not published yet.

### Shell integration

Add the matching hook, which keeps the install directory on `PATH` and defines
the `nt` command and tab completion:

```sh
# ~/.zshrc
export PATH="$HOME/.local/bin:$PATH"
eval "$(nt init zsh)"

# ~/.bashrc
export PATH="$HOME/.local/bin:$PATH"
eval "$(nt init bash)"

# ~/.config/fish/config.fish
fish_add_path "$HOME/.local/bin"
nt init fish | source
```

### Why a shell hook?

`nt`'s whole job is to **`cd` your shell** into a worktree — and a child process
can't change its parent shell's directory. So `nt` is a binary *plus* a thin shell
function (the [zoxide](https://github.com/ajeetdsouza/zoxide) model). The binary
writes its target directory to a temp file named in `$NT_CD_FILE`; the function
reads it and does the `cd`. Rich output (the `ls` table, status lines) stays on
stdout, and paths with spaces or quotes round-trip as raw bytes — no `eval`, no
quoting games.

## Usage

| Command | What it does |
|---|---|
| `nt <branch> [base]` | spin up / jump to a worktree and `cd` in |
| `nt cd [branch]` | `cd` to an existing worktree (picker if branch omitted) |
| `nt rm [-f] [target...]` | nuke worktree(s) (multi-picker if no target) |
| `nt done [-f] [target]` | nuke a worktree **and** delete its local branch |
| `nt prune` | tidy up: drop stale worktrees + empty dirs, offer to delete gone branches |
| `nt home` | `cd` back to the main checkout, wherever you are |
| `nt ls` | list this repo's worktrees, with dirty + ahead/behind |
| `nt` / `nt -h` | list + hint / full usage |

### Point it at a branch

The argument is *the branch you want to be on*, and `nt` figures out the rest —
always pulling the latest from origin so you're never stranded on some stale
local copy:

- **new name** → new branch off the latest `origin/main` (it fetches first)
- **exists on origin** (e.g. `nt team/issue-123-foo`) → worktree tracking
  `origin/team/issue-123-foo` at its latest
- **exists locally** → checks the branch out in a fresh worktree, fast-forwarded
  to origin when that's a clean FF. Diverged local commits? Kept, never clobbered —
  it just tells you and uses your copy.
- **already has a worktree** → skips the theatrics and `cd`s you there
- `nt <name> <base>` → fork the new branch off an explicit base instead

### Housekeeping

- **`nt rm [target...]`** → remove worktree(s), **fast**. Deleting a worktree is
  mostly the cost of `rm`-ing its working tree (a fat `node_modules` can take
  seconds *each*), so `nt rm` renames each tree aside instantly and reclaims the
  disk in a background process — it returns at once even when you nuke ten heavy
  trees. Same safety as `git worktree remove`: it refuses a dirty or locked
  worktree (and anything git can't vouch for) unless you pass `-f`. No target → a
  multi-select picker.
- **`nt done [target]`** → remove the worktree **and** delete its local branch in
  one move. Safe by default (`git branch -d` refuses an unmerged branch and keeps
  it, telling you so); `nt done -f` forces both. Detached worktree? It just removes
  it (no branch to delete).
- **`nt prune`** → fetch and prune the configured remote, drop stale worktree
  entries, sweep up the empty `team/`-style parent dirs git leaves behind, reap
  any leftover background-delete trash, then — if any local branches have a
  **gone upstream** (merged & deleted on the remote) — offer to clean them. In a
  non-interactive shell it only *lists* the gone branches — it never deletes
  without you there.

### A nicer `nt ls`

`nt ls` (and bare `nt`) show a dirty marker and ahead/behind vs upstream:

```
  main             =      ~/Developer/acme   (main)
* fix-login        ↑2     ~/Developer/acme.worktrees/fix-login
  team/issue-123   ↓1     ~/Developer/acme.worktrees/team/issue-123

* = uncommitted changes
```

`*` = uncommitted changes, `↑n`/`↓n` = ahead/behind the upstream, `=` = in sync,
`-` = no upstream, `gone` = upstream deleted, `?` = the dir is gone (a `nt prune`
candidate). The main checkout always lists first, columns are aligned, and color
is used only on a real terminal (`NO_COLOR` is honored). In a non-UTF-8 locale the
arrows degrade to `^n`/`vn` so the columns still line up.

### Tab completion

Completion is generated by the binary and works across zsh/bash/fish:

- `nt <tab>` → subcommands plus existing branch names
- `nt cd <tab>` → branches that currently have a worktree
- `nt rm <tab>` / `nt done <tab>` → every worktree except the main checkout (by
  branch name, or full path for detached ones), plus `-f`
- `nt <branch> <tab>` → branches, to pick a base

## Notes & knobs

- New branches are created `--no-track`, so a stray `git push`/`git pull` won't
  accidentally nuke `main`. Push with `-u` when you're ready.
- `NT_NO_FETCH=1 nt …` skips the network fetch (offline, or just impatient).
- `NT_ROOT=/path nt …` overrides where worktrees live (default `<repo>.worktrees`).
- `NT_REMOTE=upstream nt …` fetches/tracks against a remote other than `origin`.
- `nt rm`/`nt done` take a branch name, a full worktree path, or a unique trailing
  path segment. `nt rm` takes several at once (`-f` may go anywhere) — it resolves
  the whole list up front and removes **nothing** unless *every* target checks out
  (an ambiguous tail is refused with the matches listed; naming the main checkout
  aborts the batch). It flat-out refuses to remove the main checkout, and if you
  `rm`/`done` the worktree you're standing in, it steps you back home first.
- The fast deferred delete only handles worktrees `nt` can confirm are simple
  (clean, unlocked, no submodule); a dirty, **locked**, or submodule-containing
  worktree is handed to `git worktree remove` itself, so it behaves exactly as git
  does. Nested worktrees must be named in the same `nt rm` command and are removed
  before their parent. If any child is omitted or its removal fails, the parent
  is preserved. `nt prune` reaps any `.nt-trash-*` a killed delete left behind
  (handy after a reboot mid-delete).
- The interactive pickers are built in — **no `fzf` dependency**. In a
  non-interactive shell, just pass an explicit branch/target.

## Tests

```sh
go test ./...            # unit + end-to-end (testscript) — no shell needed
zsh test/parity/run.zsh  # behavioral parity oracle: drives the binary through a real zsh shim
```

The parity suite is the behavioral spec: ~100 assertions that exercise the binary
through the actual `nt init zsh` cd-shim, so "works the same way" is *proven*, not
claimed. CI runs all of it on Linux and macOS.

MIT.
