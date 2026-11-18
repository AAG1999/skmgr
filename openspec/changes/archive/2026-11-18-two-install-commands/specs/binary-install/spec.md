# Spec Delta

## Purpose

Defines the two commands that install the skmgr binary, and the install methods the project stops offering.

## ADDED Requirements

### Requirement: Two install commands
A person SHALL install skmgr with exactly one of two commands. On macOS or Linux, including WSL, the command is `curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh`. On Windows PowerShell, the command is `irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex`. Each script downloads the matching release archive and puts `skmgr` on `PATH`.

#### Scenario: macOS or Linux
- **WHEN** a person on macOS or Linux runs the `install.sh` command
- **THEN** the script downloads `skmgr_<OS>_<arch>.tar.gz` from the latest GitHub release and installs the `skmgr` binary

#### Scenario: Windows
- **WHEN** a person on Windows runs the `install.ps1` command in PowerShell
- **THEN** the script downloads the Windows zip from the latest GitHub release and installs `skmgr.exe`

#### Scenario: WSL
- **WHEN** the machine is WSL
- **THEN** the install uses `install.sh`, the same command as Linux

### Requirement: Docs offer only those commands
The README install section, `docs/how-it-works.md`, and every other install instruction SHALL show those two commands and SHALL NOT present Homebrew, `go install`, a Linux package, or a hand-downloaded zip as a way to install.

#### Scenario: README install section
- **WHEN** a reader opens the install section of the README
- **THEN** it shows the shell command and the PowerShell command, and it does not mention Homebrew, `go install`, apt, rpm, apk, or downloading a zip by hand

### Requirement: The agent prompt uses the same two commands
`scripts/agent-install-prompt.md` SHALL tell an agent to run `install.sh` on macOS, Linux, and WSL, and `install.ps1` on Windows PowerShell. It SHALL NOT mention Homebrew, `go install`, or a package manager.

#### Scenario: Agent install prompt
- **WHEN** an agent follows `scripts/agent-install-prompt.md`
- **THEN** the only install commands it is given are `install.sh` and `install.ps1`

### Requirement: Homebrew and Linux packages are not published
The release SHALL NOT publish a Homebrew cask, formula, deb, rpm, or apk. Release archives (`tar.gz` and zip) SHALL still be published, because the two install scripts download them.

#### Scenario: Release contents
- **WHEN** a release is built
- **THEN** it contains the platform archives and does not contain a Homebrew cask, deb, rpm, or apk

### Requirement: Self-update stays
`skmgr self-update` SHALL keep updating an already installed binary from the GitHub release. It is not a third way to install.

#### Scenario: Already installed
- **WHEN** `skmgr` is already on `PATH` and the user runs `skmgr self-update`
- **THEN** the command replaces that binary with the newest release
