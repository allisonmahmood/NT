# `nt` Go rewrite — plan

Goal: turn the single zsh plugin (`nt.plugin.zsh`, ~640 lines) into a cross-shell,
easily-installed CLI built in Go, **without changing observable behavior**. The
existing zsh functional suite (`test/test_nt.zsh`, ~100 assertions) is the
behavioral spec — the rewrite is "done" when those same assertions pass through
the Go implementation.

---

## 1. The core constraint (drives the whole architecture)

`nt`'s reason to exist is that it **`cd`s your shell** into a worktree. A child
process cannot change its parent shell's directory. So the binary can never *be*
the whole tool — there is always a thin shell function that does the actual `cd`.
This is exactly the [zoxide](https://github.com/ajeetdsouza/zoxide) model
(`eval "$(zoxide init zsh)"`).

### The cd-signal protocol

zoxide prints the target dir on **stdout** and the function `cd`s to it. That
doesn't fit `nt`, because `nt` has rich stdout (the `ls` table, status lines,
help). Mixing "where to cd" into stdout would make `nt ls | …` and coloring
awkward.

**Decision: signal the cd out-of-band via a temp file named in `$NT_CD_FILE`.**
The binary writes all human output to stdout/stderr as normal, and *only* when it
wants the shell to move does it write the absolute target path to `$NT_CD_FILE`.
The shell function reads that file after the binary exits and `cd`s.

```sh
# what `nt init zsh|bash` emits
nt() {
  local _ntf; _ntf="$(mktemp -u "${TMPDIR:-/tmp}/nt-cd.XXXXXX")"
  NT_CD_FILE="$_ntf" command nt "$@"          # `command` bypasses this function
  local _rc=$?
  if [ -f "$_ntf" ]; then cd -- "$(cat "$_ntf")"; rm -f "$_ntf"; fi
  return $_rc
}
```

```fish
function nt
    set -l ntf (mktemp -u)
    NT_CD_FILE=$ntf command nt $argv
    set -l rc $status
    if test -f $ntf; cd (cat $ntf); rm -f $ntf; end
    return $rc
end
```

Why this protocol:
- **No shell quoting/`eval` injection** — paths with spaces/quotes/newlines (the
  test suite has a `has space-and'quote` case) round-trip as raw bytes.
- **Crash-safe** — binary dies → no file written → no spurious `cd`.
- **Trivially unit-testable without a shell**: Go tests set `NT_CD_FILE` to a temp
  path and assert its contents == expected worktree. We test "would cd here"
  in pure Go; the real `cd` is verified once by the zsh parity suite (§6).
- `command nt` runs the *binary* even though a function `nt` exists, so the binary
  and the user-facing command can share the name `nt`.

---

## 2. Tooling choices (the "2026 CLI" stack)

| Concern | Choice | Why / alternatives |
|---|---|---|
| Command framework | **spf13/cobra** | Subcommands + **dynamic** shell completions (bash/zsh/fish/pwsh) generated for free — this is what replaces the hand-rolled `completions/_nt`. Alt: `urfave/cli` v3 (simpler), `alecthomas/kong` (struct-tags). Cobra wins on completion maturity. |
| CLI polish | **charmbracelet/fang** | Wraps cobra's root: styled help/errors, `--version`, auto manpages & completion command. This is the "feels built in 2026" layer. |
| Styling / `ls` table | **charmbracelet/lipgloss** (+ `table`) | Aligned columns, color that auto-respects `NO_COLOR`/non-tty. Replaces the hand-rolled `printf '%-*s'` width math. |
| Interactive pickers | **charmbracelet/huh** | In-binary multi-select for `nt rm`/`nt prune`/`nt cd`. **Removes the hard `fzf` dependency** — a real "use it more broadly" win. Falls back to a numbered prompt when non-tty/dumb-term. |
| Git access | **shell out to `git`** via `os/exec` | The contract the tests rely on is `git worktree`'s exact semantics (locked/dirty/submodule refusals). `go-git` has weak worktree support. Shelling out = guaranteed parity. |
| Concurrency | stdlib `errgroup` + bounded sem | Replaces `xargs -P 16` for the parallel `git status` walk; `go test -race` covers it. |
| Tests | `testing` + **rogpeppe/go-internal `testscript`** | testscript (`.txtar`) is the idiomatic way to test Go CLIs end-to-end with golden-file auto-update; plus the ported zsh suite as the parity oracle. |
| Release | **goreleaser** + GitHub Actions | Cross-compile, archives, checksums, GH Release, Homebrew tap, SBOM + cosign. |
| Lint | **golangci-lint v2** | v2.x, single `.golangci.yml`. |

Module path: `github.com/allisonmahmood/nt`. Unix-first (linux/darwin, amd64+arm64);
Windows is a later target (worktree + detached-delete semantics differ).

---

## 3. Package layout

```
nt/
  go.mod                       module github.com/allisonmahmood/nt
  main.go                      thin: fang.Execute(ctx, cmd.Root())
  cmd/
    root.go                    bare `nt` = hint line + ls; wires fang
    create.go                  default path: nt <branch> [base]
    cd.go  rm.go  done.go  prune.go  home.go  ls.go
    init.go                    nt init zsh|bash|fish  → embedded shim
  internal/
    git/        exec wrappers: WorktreeList(), Fetch(), ShowRef(), ForEachRef()…
    worktree/   domain: Resolve(id), Classify() (simple/delegate), Remove(), List()
    shell/      cd-signal (writes $NT_CD_FILE), //go:embed shim scripts
    ui/         lipgloss ls table, huh pickers, status messages, glyph fallback
    config/     NT_ROOT, remote name, NT_NO_FETCH, trash prefix, on-enter hook
  shellinit/    nt.zsh nt.bash nt.fish        (//go:embed)
  testdata/script/*.txtar                      (testscript scenarios)
  test/parity/  the ported zsh suite (runs the Go binary via its shim)
  .goreleaser.yaml  .golangci.yml
  .github/workflows/ci.yml  release.yml
```

---

## 4. Subcommand → behavior parity contract

Every row below is pinned by an assertion in `test/test_nt.zsh`; the Go version
must reproduce it (same stdout markers, same exit code, same filesystem effect).

### `nt <branch> [base]` (create / switch)
- Worktrees live at `<main>.worktrees/<branch>` (slash branches nest: `team/x`).
- Anchor to the **main checkout** (first `worktree` porcelain line) even when run
  from inside another worktree → siblings, never nested.
- Re-run for an existing worktree → just `cd` (no duplicate).
- Fetch `origin` first unless `NT_NO_FETCH=1`; warn (not fail) on fetch error.
- Branch resolution:
  - exists locally → `worktree add`; fast-forward to `origin/<b>` **only on a clean
    FF** (`merge-base --is-ancestor`); diverged local commits kept, never clobbered.
  - exists on remote only → `worktree add --track -b`.
  - new → `worktree add --no-track -b` off `base` or remote default
    (`origin/HEAD` → `main` → `master` fallback).
  - target path already exists → error, exit 1.
- On success: write target to `$NT_CD_FILE`, print `nt: → <dest>  (branch: …)`.
- **Open Q:** drop the `run 'cc' to start Claude` tail (personal); make it a
  configurable `NT_ON_ENTER` hint or remove. Recommend remove for a general tool.

### `nt cd [branch]`
- Name → resolve worktree by branch; unknown → exit 1.
- No arg → picker (huh) over all worktree paths; feed full paths (space-safe).

### `nt home` → cd to main checkout (always exit 0).

### `nt ls` (and bare `nt`)
- One row/worktree: dirty `*`, ahead/behind, path, `(main)` tag, legend line.
- Ahead/behind from **one** `git for-each-ref` (`%(upstream)` / `%(upstream:track)`),
  scoped to worktree branches; dirty flags fanned out in parallel.
- Glyphs: `↑`/`↓` under UTF-8 locale, ASCII `^`/`v` fallback; `=` in sync, `-` no
  upstream, `gone` upstream deleted, `?` dir missing on disk.
- Columns aligned (lipgloss table). Main checkout listed first.

### `nt rm [-f] [target…]`  (the dangerous one — most assertions live here)
- `-f`/`--force` may appear **anywhere** among targets.
- Resolve every target up front; if **any** is unknown/ambiguous/the main checkout
  → remove **nothing**, exit 1 (a typo never half-finishes a batch).
- Identifier = branch name | absolute path | unique trailing path segment;
  ambiguous tail → list candidates, exit 2-internally→1.
- De-dupe targets (branch + its own path = one removal).
- If standing inside a doomed tree → cd to main first (via `$NT_CD_FILE`).
- **Fast deferred delete:** for a tree we can *positively* confirm is **simple**
  (git-readable, clean w/ `--ignore-submodules=none`, unlocked, no `160000` gitlink),
  rename it aside to `.nt-trash-<pid>-<seq>` (instant `mv`), one `git worktree prune`,
  and reclaim disk in a **detached** background process → returns instantly.
- **Fail-safe classification:** anything not provably simple (dirty, locked,
  submodule/gitlink, or any git error) is handed to `git worktree remove` itself —
  so misclassification falls back to git's correct, slower path. Never reimplement
  git's removal semantics.
  - dirty without `-f` → refused, kept (incl. nasty quoted/spaced paths → **no data loss**).
  - locked → refused even **with** `-f` (mirror git; needs explicit unlock).
  - clean-but-submodule / gitlink-no-.gitmodules → refused without `-f`.
  - git-error worktree → fail **closed** (refuse), never silently "clean".
- Nested parent+child in one batch → child already gone counts as success (honest 0).
- No target → huh multi-picker (space=mark, enter=remove; no marks = highlighted).
- `git fsck` stays clean after deferred removal; back-to-back removals in one process
  must not collide on trash names (pid + atomic counter, like zsh's `$$-seq`).

### `nt done [-f] [target]`
- Remove worktree **and** delete its local branch (`-d`; `-D` under `-f`).
- Detached worktree → remove, print "no branch to delete", exit 0.
- Unmerged branch without `-f` → worktree removed, branch **kept**, exit 1, with the
  "use 'nt done -f' / 'git branch -D'" hint. Dirty worktree needs `-f`.
- Refuse the main checkout. Report deleted sha ("recover via git reflog").

### `nt prune`
- `git worktree prune`; sweep empty `team/`-style parent dirs **without** descending
  into live worktrees; reap abandoned `.nt-trash-*` (interrupted bg delete).
- Local branches with `[gone]` upstream → huh picker (incl. an `[ALL]` row) to delete;
  **non-tty → list only, never delete** (CI safety).

### Global
- Not inside a git repo → `nt: not inside a git repository`, exit 1 (every subcommand).
- Knobs: `NT_NO_FETCH`, `NT_ROOT` (new — make the `.worktrees` location configurable
  instead of "edit one line"), remote name (new, default `origin`).

### Background delete in Go (replacing `rm -rf … &!`)
Rename-aside synchronously → `git worktree prune` → start a **detached** child
(`exec.Command` of self as hidden `nt __reap <paths…>`, `SysProcAttr{Setsid:true}`,
no `Wait`). The `nt prune` reaper still sweeps any `.nt-trash-*` a killed delete left
behind (crash recovery), so correctness never depends on the child finishing.

---

## 5. Completions (replacing `completions/_nt`)

Cobra **dynamic** completions cover everything the `_nt` file did, cross-shell:
- root: subcommands + existing branch names (`ValidArgsFunction`).
- `cd`: branches that currently have a worktree.
- `rm` / `done`: every worktree except main (branch name, or full path for detached) + `-f`.
- `<branch> <tab>`: branches, to pick a base.

`nt completion zsh|bash|fish|powershell` emits the script; `nt init <shell>` bundles
that **plus** the `cd`-shim function in one line for `~/.zshrc`.

---

## 6. Test strategy — *proving* parity, not claiming it

Four layers, fastest first:

1. **Go unit tests** (`go test -race ./...`, no shell needed):
   - pure logic: target resolution, the "simple" classifier decision table,
     `upstream:track` ahead/behind parsing, glyph/locale fallback, ls-table golden,
     base-desc. git-touching helpers use throwaway repos built in `t.TempDir()` via a
     shared helper that mirrors the suite's bare-origin + clone + remote-branch setup.
   - cd-signal: assert `$NT_CD_FILE` contents for create/cd/home/rm-from-inside.

2. **testscript `.txtar`** end-to-end on the *binary* (`rogpeppe/go-internal`):
   each scenario builds a git repo, runs `nt …`, asserts stdout/stderr/exit +
   `exists`/`! exists`/`cmp` on the tree and on `$NT_CD_FILE`. Golden output
   auto-updates with `-update`. Covers exit codes, messages, fs effects, the
   deferred-delete + trash-reap, nested batches, fail-safe refusals.

3. **The zsh parity suite = the oracle.** Adapt `test/test_nt.zsh` so that instead
   of `source nt.plugin.zsh` it does: put the Go `nt` on `PATH`, then
   `source <(nt init zsh)`. The *same ~100 assertions* now drive the Go binary
   through the real zsh `cd`-shim. This is the literal "works the same way the zsh
   version did" check, including the `cd` actually happening and completion
   registering. Runs in CI on zsh-available runners.

4. **Completion tests:** call the cobra `ValidArgsFunction`s directly and assert
   candidates (replaces `test_completion.zsh`'s intent), plus a smoke test that each
   `nt completion <shell>` emits a non-empty, sourceable script.

Parity acceptance gate: layer 3 must be **100% green** before the zsh plugin is retired.

---

## 7. CI & release (all free for a public repo)

### `.github/workflows/ci.yml` (push / PR)
- **lint**: `golangci/golangci-lint-action` (v2.x) + `gofmt` check + `go vet` + `govulncheck`.
- **test** matrix: os `[ubuntu-latest, macos-latest]` × Go `[stable, oldstable]`,
  `setup-go` (build cache on), `go test -race -coverprofile ./...`.
- **parity**: ubuntu + macos; `apt-get install zsh git` (macOS has zsh); build the
  binary, run the ported zsh suite via the generated shim.
- **build-snapshot**: `goreleaser build --snapshot --clean` to prove all
  cross-compile targets still build on every PR (catches breakage pre-release).

### `.github/workflows/release.yml` (tag `v*`)
- `goreleaser/goreleaser-action` with `fetch-depth: 0`; `GITHUB_TOKEN` for the
  release + a `GH_PAT` secret to push the **Homebrew tap** (second repo
  `homebrew-nt`). Produces: linux/darwin amd64+arm64 archives, `checksums.txt`,
  GH Release with auto changelog, Homebrew formula, optional Scoop/Nix/AUR, SBOM,
  and cosign signing (modern supply-chain hygiene).

### Free extras worth turning on
- **Dependabot** (gomod + github-actions), **CodeQL** (Go), an install one-liner
  (`curl -fsSL …/install.sh | sh`), and badges in the README.

### Distribution channels this unlocks
`go install github.com/allisonmahmood/nt@latest`, Homebrew (`brew install allisonmahmood/nt/nt`),
Scoop/Nix/AUR, `curl | sh`, or a downloaded binary — then one line
`eval "$(nt init zsh)"` for the shell integration. Replaces "clone and source".

---

## 8. Phased rollout

0. **Scaffold** — module, `main.go`+fang, cobra root, CI skeleton, `.golangci.yml`.
1. **Read paths** — `internal/git` + `internal/worktree`, then `create`/`cd`/`home`/`ls`
   with unit + testscript tests (incl. cd-signal).
2. **Mutating paths** — `rm`/`done`/`prune`: the classifier, deferred delete, detached
   reaper. Port *every* safety assertion (dirty/locked/submodule/gitlink/git-error/
   nasty-path/nested/back-to-back). Highest-risk phase.
3. **Shell integration** — `nt init` for zsh/bash/fish + dynamic completions; wire the
   zsh parity suite into CI. Parity gate.
4. **Polish** — lipgloss ls table, huh pickers (drop fzf), fang help, config knobs
   (`NT_ROOT`, remote), README rewrite.
5. **Release** — goreleaser + homebrew tap + signing; tag `v0.1.0`.

Keep `nt.plugin.zsh` in the repo (or `legacy/`) until phase 3's parity suite is
green, then retire it (leave a pointer in the README for existing users).

---

## 9. Open questions to confirm before coding

1. **Keep or drop the `run 'cc' to start Claude` enter-hint?** Recommend dropping it
   (or `NT_ON_ENTER`) for a general-purpose tool.
2. **Drop the `fzf` dependency in favor of an in-binary picker (huh)?** Recommended —
   it's the biggest "broader audience" win. (Could keep `fzf` as an opt-in backend.)
3. **Windows now or later?** Recommend later (unix-first); the deferred-delete and
   worktree semantics need separate handling.
4. **Repo strategy for the rewrite** — land it on this branch alongside the zsh
   plugin (parallel, retire after parity), vs a fresh top-level Go module. Recommend
   in-place with the plugin kept until parity is green.
5. **Homebrew tap** — create `allisonmahmood/homebrew-nt` + a `GH_PAT` secret (needed
   before the release workflow can publish).
