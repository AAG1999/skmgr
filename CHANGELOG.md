# Changelog

All notable changes to `skmgr` are recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
While `skmgr` is pre-1.0, breaking changes may land in any release and are
called out explicitly below.

## [Unreleased]

### Added

- **Windows one-liner installer.** `scripts/install.ps1` downloads the latest
  Windows release zip, installs `skmgr.exe` under `%LOCALAPPDATA%\skmgr`, and
  adds that directory to the user `PATH`. Same idea as `scripts/install.sh`:
  `irm …/install.ps1 | iex`. Handles WOW64 arch detection, TLS 1.2 on Windows
  PowerShell 5.1, and case-insensitive PATH checks. CI covers a mocked
  end-to-end run plus rejection of empty/missing zips; optional
  `SKMGR_ZIP_PATH` / `SKMGR_INSTALL_DIR` / `SKMGR_SKIP_PATH` overrides support
  that without hitting the network.
- **Agent install prompt.** `scripts/agent-install-prompt.md` is a copy-paste
  prompt that tells an AI coding agent to detect OS/arch and install skmgr via
  the correct one-liner or package manager.

### Changed

- **Homebrew packaging now publishes a cask** instead of a formula that
  installed a prebuilt binary. After the next release, existing tap users may
  need one upgrade that migrates them (delete `Formula/skmgr.rb` and add
  `tap_migrations.json` in the tap). `brew install AAG1999/tap/skmgr`
  keeps working once that migration is in place.

### Fixed

- **Fetching a skill works on Windows.** Extracting a commit piped `git archive`
  into the `tar` binary, and the MSYS `tar` that ships with Git for Windows
  cannot open a drive-letter path, so every fetch failed with `tar: C\:\\...:
  Cannot open`. The archive is now decoded in process with `archive/tar`, which
  also drops the dependency on a `tar` being installed at all.
- **`skmgr self-update` no longer downloads the macOS build on Windows.** Asset
  names were matched as plain substrings, and `darwin` ends in `win` — the alias
  for Windows — so `skmgr_Darwin_x86_64.tar.gz` matched first. Platform names
  now have to appear as whole words.
- **Rewriting a file another program has open no longer fails on Windows.**
  Windows refuses to rename over a file with an open handle, which is exactly
  what happens when an editor or a running agent is reading `skmgr.lock` or
  `CLAUDE.md`. The replacement is now retried briefly instead of failing the
  command outright.
- **A corrupt lockfile reports a path you can act on.** The error quoted the
  path with Go escaping, so a Windows user was told to delete
  `C:\\Users\\me\\skmgr.lock`.
- **Global installs no longer delete each other.** `~/.agents/` is shared by
  every project, but orphan cleanup previously used only the current project's
  manifest. Running `skmgr install` in a project with no global skills deleted
  every globally installed skill on the machine. Ownership is now tracked in
  `~/.skmgr/global.lock` and pruning uses the union across all projects.
  (gaps A6-28, A6-29)
- **Global skills that appear after the ledger exists are no longer deleted.**
  Pre-existing installs in the shared `~/.agents/` tree were only adopted when
  `~/.skmgr/global.lock` was absent. Once any project had created the ledger,
  a global skill skmgr had no record of installing — one placed by hand, or by
  a tool that predates the ledger on that machine — was pruned as an orphan on
  the next project install. Unclaimed installs are now adopted on every run,
  while a global skill dropped from a manifest is still pruned. (gap A6-28)
- **`scope: global` no longer writes into the current project.** Cursor,
  Claude Code and Copilot resolved their agent directories from the project root
  even at global scope, so a global install symlinked into whichever project you
  happened to be standing in. Global scope now resolves every agent path under
  your home directory. (gap A6-30)
- **Removing a merged rule now cleans up its block.** Rules merged into
  `CLAUDE.md`, `copilot-instructions.md` or `AGENTS.md` leave no symlink behind,
  so the `<!-- skmgr:start:name -->` block survived `skmgr remove` indefinitely.
  `Sync` now diffs the previous lockfile against the new manifest and strips the
  blocks of anything you dropped. (gap A6-31)
- **Skills that ship symlinks install correctly.** `copyDir` silently skipped
  every symlink, quietly installing an incomplete tree. Symlinks are now
  recreated; one pointing outside the skill aborts the install. (gap A8-41)
- **Unbalanced managed delimiters no longer destroy files.** A
  `skmgr:start` marker without its matching `skmgr:end` — after a bad merge
  conflict resolution, say — caused the next rewrite to discard everything that
  followed it. Merges and `.gitignore` updates now refuse to rewrite such a file
  and name it in the error. (gap A9-43)

- **Cached repositories now pick up new commits.** A branch ref was served from
  whatever commit the cache held when it was first cloned, so `skmgr install`
  kept installing stale content until you deleted `~/.skmgr/cache` by hand.
  Branches now follow `origin/<branch>` after fetching, while tags and commit
  SHAs stay pinned and skip the network once cached. (gap A1-6)
- **`skmgr update <name>` now updates only that skill.** The argument was parsed
  and then ignored, so every skill moved. Other skills are now held at their
  locked commit. (gap A1-2)
- **`--verbose` and `--quiet` now do something.** Both were registered and never
  read. They map to a real leveled logger, and passing both is an error rather
  than a silent precedence rule. (gaps A1-3, B3-23)
- **`skmgr remove` deletes the canonical copy.** It previously removed only the
  agent-facing links, leaving the skill on disk under `.agents/`. (gap A1-5)
- **Flipping a skill's `type` removes the old copy.** Orphan cleanup matches on
  name alone, so a `skill` that became a `rule` left its previous directory in
  place and agents kept reading it. (gap A6-33)
- **Removing an agent from a skill's `targets` removes its links.** Cleanup
  swept only the manifest-level target list, so per-skill overrides could never
  be narrowed. (gap A6-32)
- **`skmgr clean --global` works.** The flag was accepted and ignored, leaving no
  way to clear `~/.agents/`. It now clears the global tree and releases this
  project's claims in the ledger. (gap A11-55)
- **`skmgr clean` strips managed rule blocks.** Blocks in `CLAUDE.md` and
  friends outlived the skills they described. (gap A11-56)
- **`skmgr.yaml` works everywhere.** Commands hardcoded `skmgr.yml`, so a
  project using the `.yaml` spelling was reported as uninitialized.
  (gaps A4-15, B5-32)
- **Rule content is found where it actually lives.** Merging only ever read
  `SKILL.md`; `AGENTS.md`, `rule.md` and `README.md` are now accepted too.
  (gap A4-18)
- **Invalid `--type` and `--scope` values are rejected.** A typo such as
  `--scope globl` silently installed to project scope. (gap A10-49)
- **`skmgr init` refuses to shadow an existing `skmgr.yaml`.** Discovery prefers
  `.yml`, so writing one beside a `.yaml` left two manifests drifting apart.
  (gap A10-50)
- **`--frozen` now actually verifies content.** It pinned the commit SHA but
  never checked what arrived, so a rewritten tag or a corrupted cache installed
  silently. The staged content is now hashed and compared to the lockfile before
  anything is swapped into place, and a mismatch leaves the previous install
  untouched. (gaps A1-1, A2-8, B2-9)
- **An interrupted install no longer corrupts a skill.** Files were copied
  directly over the live directory, so a failure part-way through left a mix of
  old and new. Installs now stage into `.agents/.staging/<name>` and rename into
  place only after the copy fully succeeds. (gap A8-37)
- **A crash mid-write no longer truncates the files skmgr owns.** The manifest,
  lockfile, global ledger, merged rule files and the managed `.gitignore` block
  are written to a temporary file and renamed over the target, so you are left
  with the old contents or the new ones, never half of each. (gap A8-38)
- **A corrupt lockfile is reported as corrupt.** Any read error was treated as
  "no lockfile", so a truncated or malformed `skmgr.lock` was silently discarded
  and regenerated, losing every pin. Missing and unreadable are now distinct, and
  only missing is tolerated. (gap A8-40)
- **Concurrent runs no longer fight over the cache.** Two `skmgr` processes
  cloning or fetching the same repository could interleave and leave it in a
  state neither expected. Clone and fetch now run under a per-repository advisory
  lock. (gaps A10-54, B2-12)
- **Different refs of one repository no longer overwrite each other.** The cache
  held a single checkout per repository, so a project on `main` and a project on
  `v2` took turns clobbering it. Each commit is now extracted into its own
  immutable directory. (gap B2-11)
- **`skmgr list` reports what is actually on disk.** Status came from lockfile
  presence alone, so a deleted or edited skill still showed as installed. It is
  now computed by hashing the installed directory. (gap A10-51)
- **Moving a project no longer breaks every agent link.** Symlinks pointed at
  absolute canonical paths, so renaming or relocating a checkout left every
  agent directory pointing at nowhere until a reinstall. Targets are now
  relative to the link's own directory, and existing absolute links are
  rewritten on the next sync. (gap A9-46)
- **Broken-link detection no longer depends on where you ran the command.** A
  relative link target was resolved against the process working directory rather
  than the link's directory, so valid links could be pruned. (gap A9-45)
- **Windows installs work without Developer Mode.** When `os.Symlink` is denied,
  `skmgr` now creates a directory junction, and copies the tree as a last
  resort. The copy is marked so later runs can refresh it without risking a
  directory the user put there themselves. (gaps A3-11, B6-35, B6-36)
- **A relative directory is no longer treated as a git remote.**
  `source: skills/foo` went to the git provider and failed with a clone error
  about a URL nobody wrote. An existing directory on disk is now used as a local
  source, unless the source is unmistakably remote. (gap A10-52)
- **Auto-discovery no longer installs an entire repository.** When no `skills/`
  or `rules/` directory was found, the fallback copied the repository root in as
  a single skill — README, build files and all. A `SKILL.md` or `AGENTS.md` at
  the root is now required, and the error names `path:` as the fix. (gap A10-53)
- **A `path:` wildcard is no longer silently dropped.** Matches outside a
  `skills/` or `rules/` parent were warned about and skipped even when the user
  had written the pattern themselves, so `path: packages/*/agent` installed
  nothing. The convention now applies only to auto-discovery, where nothing
  points at a particular directory. (gap A4-17)
- **Skill names that differ only in case are rejected.** `Greeter` and `greeter`
  are two manifest entries but one directory on macOS and Windows, and whichever
  installed second silently won. (gap B6-37)
- **`skmgr list --json` is valid JSON.** It was built by string concatenation
  with no escaping; it now goes through `encoding/json` and includes status,
  targets and ref. (gap A4-14)

### Security

- **Skill names can no longer escape the install tree.** A `name` containing
  `/`, `\` or `..` was joined straight onto `.agents/` and the agent
  directories. Names are now validated as a single path component at parse time
  and again at install time, and Windows reserved device names are rejected.
  (gap A7-34)
- **`path:` can no longer escape the source.** Both the git and local providers
  join `path` onto the fetched source without containment checks, so
  `path: ../../etc/ssh` on a local source could copy arbitrary host files into
  `.agents/`. Literal paths are now required to stay inside the source root, and
  glob matches are re-checked after expansion. (gaps A7-35, A7-36)
- **A `source` or `ref` can no longer smuggle options into `git`.** A value
  starting with a dash was passed straight to `git clone` or `git checkout`,
  where `--upload-pack=...` runs an arbitrary command. Such values are now
  rejected when the manifest is parsed and again immediately before the git
  call, and `git clone` terminates its options with `--`.
- The manifest and lockfile parsers are fuzzed, and the manifest parser has
  regression tests for alias bombs, self-referencing aliases, deep nesting and
  very large inputs, since a manifest is often fetched from a repository
  somebody else controls. The parser fuzzer asserts that whatever is accepted is
  safe to act on, not merely that it does not panic. (gap B8-44)

### Changed

- **Breaking:** `scope: global` now links into `~/.cursor/`, `~/.claude/`,
  `~/.github/` and `~/.gemini/config/` instead of the project's equivalents. If
  you were relying on the old behavior, the skill you want is project-scoped.
  Stale links left in your project by the old behavior are pruned by the
  broken-link sweep on the next `skmgr install`.
- `HashDirectory` hashes a symlink's target string rather than following it.
  This changes the content hash of any skill containing symlinks; those entries
  are re-recorded on the next `skmgr install`.
- **Breaking:** the cache layout changed from one checkout per repository to
  `~/.skmgr/cache/git/<source-hash>/commits/<sha>/`. Old entries are unreadable
  by this version and are deleted automatically on the first run.
- Git clones are complete rather than shallow. A shallow clone only carried the
  default branch, so any manifest naming a tag or another branch re-fetched on
  every run and some refs could not be resolved at all.
- The lockfile is now version `2`, adding `ref` and `resolved_ref` to each
  entry. Version `1` and unversioned lockfiles are still read and are upgraded
  the next time one is written, so a team does not have to upgrade in lockstep.
  (gap B1-5)
- **Breaking:** `--global` moved off the root command and onto `add`, `list` and
  `clean`, the three commands where it means something. `install`, `sync`,
  `update` and `remove` now reject it instead of accepting and ignoring it —
  their scope comes from the manifest. Nothing changes about what those commands
  do; the mistake is simply visible now. (gap A10-47)
- **`skmgr add --ref` defaults to empty** rather than `main`, so a repository
  whose default branch is `master` or `trunk` no longer gets a nonexistent ref
  written into its manifest. The provider resolves the real default branch.
  (gap A10-48)
- `SKMGR_HOME` and `SKMGR_CACHE_DIR`, and their config-file equivalents, expand
  a leading `~` and are resolved to absolute paths. A relative value previously
  produced a different cache depending on which directory you ran from.

### Migration

No action required. On the first run of this version, any global skills already
present in `~/.agents/` are adopted into `~/.skmgr/global.lock` and protected
from pruning, since `skmgr` has no record of which project installed them. Use
`skmgr clean --global` when you want to clear them deliberately.

The cache is migrated by deletion: entries in the old layout are removed the
first time this version runs, and the next install re-fetches what it needs. The
only cost is one slower run.

If you are upgrading a CI pipeline, `skmgr install --frozen` may now fail where
it previously passed — that is the point. A failure means the source behind a
pinned ref changed content. Confirm the change is legitimate, run `skmgr install`
without `--frozen`, and commit the updated lockfile.

### Added

- **Skill names are derived automatically.** `name` is now optional: it comes
  from the last segment of `path`, or from the repository name when there is no
  path. Set it explicitly only to choose a different alias or to disambiguate
  two entries that would derive the same name. Commands that rewrite the
  manifest keep it that way: derived names and default `type` / `scope` values
  are not written back. (gaps A5-20, A5-22 through A5-27)
- **`skmgr install --frozen`** pins every skill to the commit SHA in the
  lockfile and resolves nothing, so a moving ref cannot drift mid-pipeline.
  Local `file://` sources have no SHA and are still re-read from disk.
- **`--offline`** on `install`, `update` and `sync` forbids all network access;
  anything not already cached fails with a clear error.
- **`--log-level`** global flag (`debug`, `info`, `warn`, `error`). Logs go to
  stderr so command output stays pipeable.
- **`skmgr init` detects your agents** from the `.cursor`, `.gemini`, `.claude`
  and `.github` directories already in the project, and writes matching targets.
  (gap A3-10)
- Lockfile entries record `source`, `path` and `resolved_at`. (gap A2-7)
- The managed `.gitignore` block carries a notice that it is generated, since it
  is rebuilt on every run and anything written inside it is discarded.
  (gap A9-44)
- **Semantic version ranges.** `ref` now accepts a range — `^1.2.0`, `~1.2`,
  `>=1.0.0 <2.0.0`, `1.x`, `1.0.0 - 1.4.0`, `1.x || 2.x`, `*` or `latest` — and
  resolves it against the tags the repository publishes. A bare `v1.2.3` is
  still an exact tag, and a branch name is still a branch name: only
  unambiguous constraint forms are treated as ranges. Prereleases are excluded
  unless the range names one. (gaps B1-1 through B1-4, B1-8)
- **`skmgr outdated`** compares each skill against its repository's tags and
  reports three versions: installed, newest inside the declared range, and
  newest overall. `--json` for scripting. (gap B1-7)
- **`skmgr update --latest`** takes the newest release even when it falls
  outside the declared range, for crossing a major version deliberately. It is
  a one-shot override; the manifest is not rewritten, so the next plain update
  returns to the declared bounds. (gap B1-6)
- **`git::` source prefix** forces a source to be treated as a git repository.
  A path on disk is otherwise read as a plain directory, which leaves `ref` and
  version ranges with nothing to act on — the wrong reading for a local clone or
  mirror, as used in air-gapped setups.
- **`skmgr verify`** re-hashes every installed skill and compares it to the
  lockfile, reporting each as `ok`, `modified` or `missing`. It never touches the
  network and exits non-zero on any drift, so it works as a CI gate or a
  pre-commit check. `--json` for scripting. (gap B2-10)
- **`skmgr cache`** subcommands: `path` prints the cache directory (useful as a
  CI cache key), `info` reports repositories, commit trees and how much of the
  total is clones, and `prune` deletes trees by age (`--older-than 30d`) or
  total size (`--max-size 2GB`), with `--dry-run`. A repository's clone is
  dropped once every tree extracted from it has been. (gap B2-15)
- **`SKMGR_OFFLINE`** environment variable, equivalent to passing `--offline` to
  every command. Easier than editing each invocation in an air-gapped job.
- **Transient network failures are retried** three times with exponential
  backoff before a command gives up. (gap B2-14)
- **The lockfile version is checked on read.** A lockfile written by a newer
  `skmgr` is refused with an explanation rather than being partially understood;
  one written before versioning is accepted and stamped. (gap A2-9)
- **`--dry-run`** on `add`, `install`, `remove`, `sync` and `update` prints the
  skills that would be installed, updated, reinstalled or removed and writes
  nothing at all. The preview is produced by the same resolver the real run
  uses, so it cannot disagree with what follows. (gap B3-16)
- **`skmgr doctor`** checks git, the config file, the cache, the canonical
  store, symlink support, the lockfile and whether installs still match their
  recorded hashes. `--json` for scripting. (gap B3-19)
- **`skmgr tree`** shows the installed layout: scopes, skills, and which agents
  each one is linked into, marking anything missing. `--json` for scripting.
  (gap B3-18)
- **`skmgr self-update`** replaces the binary with the newest GitHub release,
  with `--check` to look without installing. (gap B3-20)
- **`skmgr completion bash|zsh|fish|powershell`**, plus man pages and shell
  completions shipped in the release archives, the Homebrew formula and the
  Linux packages. Both are generated from the live command tree, so they cannot
  describe a flag that no longer exists. (gaps B3-21, B3-22)
- **A config file** at `~/.config/skmgr/config.yml` for `home`, `cache_dir`,
  `offline`, `log_level` and default `targets`, with `SKMGR_CONFIG`,
  `SKMGR_HOME`, `SKMGR_CACHE_DIR`, `SKMGR_OFFLINE` and `SKMGR_LOG_LEVEL`
  overriding it and flags overriding those. A file that cannot be parsed stops
  the command rather than being ignored. (gaps B4-27, B4-28)
- **A JSON Schema** at `schemas/skmgr.schema.json` for editor autocomplete and
  inline validation, shipped with each release. (gap B5-31)
- **Manifest errors name the line they are on**, in the `line 12: …` form
  editors turn into a link, and suggest a correction for a misspelled agent
  name. Unknown keys are now rejected instead of ignored, so a misspelled `pth:`
  is an error rather than a skill installing from the wrong directory.
  (gap B5-33)
- **Documented exit codes**: `0` success, `1` failure, `2` usage, `3` drift.
  `skmgr verify` and `skmgr doctor` both use `3`, so a pipeline can tell "your
  skills changed" from "skmgr could not run". (gap B3-26)
- `~/.skmgr/global.lock`: ledger of global skill ownership per project.
- `CHANGELOG.md` and `gap-fixed.md`.

### Documentation

- The README was rewritten where it described behavior that did not exist: the
  agent path table was wrong for every row, the `.gitignore` markers were not
  the ones in the code, and the `init` auto-detection claim was false (that one
  was fixed by implementing detection rather than deleting the claim).
  Caching and ref semantics, the lockfile shape, and frozen and offline mode are
  now documented. (gaps A3-10, A3-12, A3-13, B9-47)
- A Versioning section in the README covers ranges, prerelease rules, what
  `skmgr outdated` reports, `--latest`, and versioning against a local mirror
  with `git::`.
- `architecture.md` gained sections on the cache layout and on durability;
  `flow.md` gained the verify and install-integrity flows and an accurate resolve
  path. The README documents `skmgr verify`, `skmgr cache prune`, `SKMGR_OFFLINE`,
  and a CI recipe that caches downloads and verifies the result.
- New reference pages under `docs/reference/`: environment variables, exit
  codes, the config file, and working with private repositories behind a proxy
  or an SSH key. (gaps B4-29, B4-30, B9-50)
- The README explains how `path` is interpreted in each of its three modes, what
  a validation error looks like, and the real Windows fallback chain — it
  previously claimed a junction fallback that did not exist.
- The README documents that installed files keep their permission bits — a skill
  shipping an executable helper installs one that still runs — and that
  ownership is not copied, because doing so needs privileges a dependency
  manager should not hold. (gap A8-42)
- **Name derivation no longer produces a name that cannot be a directory.**
  A path whose last segment was only whitespace was returned as the name and
  then rejected with "name must not be empty", blaming a name the manifest
  never contained; a segment containing a character that is legal in a path but
  not in a directory name skmgr creates was returned unchecked. Candidates are
  now validated, an unusable one falls through to the repository name, and when
  nothing works the error names the problem and points at `name`. Both cases
  were found by fuzzing.
- The lint configuration was migrated to golangci-lint v2. It had been in v1
  format against a pinned linter build that predated the Go toolchain the rest
  of CI uses, so running it locally produced a config error rather than a lint
  run. Re-enabling it caught an unchecked write in the logger and a dead
  function left over from the content-addressed cache work.
- A "Further reading" table links `architecture.md`, `flow.md`, the changelog,
  the contributing guide and the four reference pages, which were previously
  reachable only by knowing they existed. (gap B9-48)
- `CONTRIBUTING.md` is new: setup, the checks to run before opening a PR, and
  the areas where review is strict — deletion paths, global scope, path
  handling, git arguments and atomic writes. The README's contributing section
  now points at it rather than keeping a shorter copy that had already drifted:
  it claimed adding an agent needed "no core logic changes", omitting the
  supported-name list, the schema enum and `init` detection. (gap B9-49)
