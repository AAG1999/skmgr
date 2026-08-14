# Agent prompt: install skmgr

Copy everything below the line into an AI coding agent (Cursor, Claude Code, Copilot, etc.).

---

Install **skmgr** (https://github.com/AAG1999/skmgr) on this machine.

## Goal

Detect the current OS and architecture, install the `skmgr` CLI if it is missing or outdated, then verify it works. Prefer the official one-liner for this platform. Do not ask me which OS I am on — discover it yourself.

## Steps

1. **Detect the environment**
   - OS: Linux, macOS, or Windows (and whether the shell is PowerShell, cmd, bash, zsh, or WSL).
   - CPU: `x86_64` / `amd64` or `arm64` / `aarch64`. Refuse native 32-bit Windows (no build is published).
   - Check whether `skmgr` is already on `PATH` (`skmgr --version`). If it runs, print the version and stop unless I asked to upgrade.

2. **Install using the best method for this host** (try in order; stop at the first success):

   **macOS**
   1. If Homebrew exists: `brew install AAG1999/tap/skmgr`
   2. Else: `curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh`

   **Linux**
   `curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh`

   **Windows (native PowerShell)**
   ```powershell
   irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex
   ```
   Afterward, ensure the current session can see the binary (refresh `$env:Path` from the User + Machine PATH, or invoke `%LOCALAPPDATA%\skmgr\skmgr.exe` directly).

   **Windows WSL**
   Treat as Linux (use `install.sh` or the distro package manager inside WSL), not the PowerShell installer.

   **Fallback (any OS with Go 1.22+)**
   ```text
   go install github.com/AAG1999/skmgr@latest
   ```
   Ensure `$(go env GOPATH)/bin` (or `%USERPROFILE%\go\bin`) is on `PATH`.

   **Last resort**
   Download the matching asset from https://github.com/AAG1999/skmgr/releases (`skmgr_Windows_*.zip`, `skmgr_Darwin_*.tar.gz`, or `skmgr_Linux_*.tar.gz`) and place the binary on `PATH`.

3. **Verify**
   - Run `skmgr --version` and show the output.
   - If `git` is missing, warn that remote skill installs need Git on `PATH`, and suggest installing Git (do not block the skmgr install on this).
   - Optionally run `skmgr doctor` and summarize any failures.

4. **Constraints**
   - Do not invent alternate download URLs or package names.
   - Do not use `sudo` on Windows; on Unix only use it when a system package install requires it.
   - Do not modify global git config or credentials.
   - If the chosen method fails, try the next method and report what failed.
   - When done, give me the exact command(s) you ran and how to install skills next:
     `skmgr init` → `skmgr add <source>` → `skmgr install`.
