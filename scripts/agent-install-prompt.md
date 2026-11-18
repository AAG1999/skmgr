# Agent prompt: install skmgr

Copy everything below the line into an AI coding agent (Cursor, Claude Code, Copilot, etc.).

---

Install **skmgr** (https://github.com/AAG1999/skmgr) on this machine.

## Goal

Detect the current OS and architecture, install the `skmgr` CLI if it is missing, then verify it works. Use the one command for this platform. Do not ask me which OS I am on — discover it yourself.

## Steps

1. **Detect the environment**
   - OS: Linux, macOS, Windows, or WSL (and whether the shell is PowerShell, cmd, bash, or zsh).
   - CPU: `x86_64` / `amd64` or `arm64` / `aarch64`. Refuse native 32-bit Windows (no build is published).
   - Check whether `skmgr` is already on `PATH` (`skmgr --version`). If it runs, print the version and stop unless I asked to upgrade.

2. **Install with the one command for this host**

   **macOS, Linux, or WSL**

   ```bash
   curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh
   ```

   **Windows (native PowerShell)**

   ```powershell
   irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex
   ```

   Afterward, ensure the current session can see the binary (refresh `$env:Path` from the User + Machine PATH, or invoke `%LOCALAPPDATA%\skmgr\skmgr.exe` directly).

3. **Verify**
   - Run `skmgr --version` and show the output.
   - If `git` is missing, warn that remote skill installs need Git on `PATH`, and suggest installing Git (do not block the skmgr install on this).
   - Optionally run `skmgr doctor` and summarize any failures.

4. **Constraints**
   - Do not invent alternate download URLs.
   - Do not use `sudo` on Windows. On macOS or Linux, use it only when `/usr/local/bin` is not writable and the script itself asks for it.
   - Do not modify global git config or credentials.
   - If the command fails, stop and report the error. Do not try a different installer.
   - When done, give me the exact command(s) you ran and how to continue:
     `skmgr init`, restart the IDE, then `/skmgr-find-skill`. Ref bumps stay `skmgr update` (or `/skmgr-update-skill`).
