# sshctl

**English** · [简体中文](README.zh-CN.md)

Find a host in your local inventory, then open an SSH shell, run a command, or transfer files without retyping its connection details. `sshctl` stores passwords encrypted on disk and gives agents and scripts predictable flags, output, and exit codes.

[Download](https://github.com/Fracizz/sshctl-skill/releases/latest) · [Quick start](#quick-start) · [Agent Skill](skills/sshctl/SKILL.md) · [Security](SECURITY.md)

## Features

- Single static binaries: Linux / Windows / macOS (amd64 + arm64)
- Remote command execution and interactive shells
- Fast file transfer via SFTP (`scp` subcommand, directories supported)
- Native incremental upload (`rsync`) with dry-run, SHA-256 comparison, and scoped deletion
- AES-256-GCM encrypted passwords at rest (`enc:v1:`)
- Case-insensitive contains search on name / IP / description
- Host key verification skipped by default (`--insecure`); pass `--secure` to check `~/.ssh/known_hosts`
- Default config outside the repo: `~/.sshctl/servers.json`
- One-time migration from `~/.sshfrac` / `~/.invossh` (automatic or `sshctl migrate`)

## Why AI-first?

| Need | How sshctl helps |
|------|------------------|
| Non-interactive | `exec` / `scp` / `rsync` / `list` / `search` — no TTY prompts for common flows |
| Discover hosts | `search -s <keyword>` on name, IP, description |
| Safe-ish secrets | Passwords encrypted on disk; agents pass `--password` only when adding |
| Predictable I/O | Tabular list output + non-zero exit on remote failure |

## Install

**For AI agents (recommended):** build the binary next to the skill — do **not** add sshctl to system PATH.

```powershell
git clone https://github.com/Fracizz/sshctl-skill.git
cd sshctl-skill
go build -o skills/sshctl/bin/sshctl.exe .
# or: .\scripts\build.ps1   # Windows amd64 → bin/ + skill bins (~/.claude|~/.codex if present)
```

Agent skill: [skills/sshctl/SKILL.md](skills/sshctl/SKILL.md) — resolve from the skill file directory (not repo root):

```powershell
# skillRoot = directory containing SKILL.md (agent resolves from attached skill path)
$skillRoot = '...'  # e.g. skills/sshctl/ or ~/.claude/skills/sshctl/
$sshctl = Join-Path $skillRoot 'bin\sshctl.exe'
& $sshctl version
```

`skills/sshctl/bin/` is gitignored; clone from source, or download from [Releases](https://github.com/Fracizz/sshctl-skill/releases):

| Asset | Notes |
|-------|--------|
| `sshctl-skill.zip` | **AI skill pack** — `sshctl skills install --zip` overlays `SKILL.md` + `bin/` (does not wipe inventory) |
| Windows x64 / ARM64 | `sshctl-windows-amd64.exe` / `sshctl-windows-arm64.exe` |
| Linux x64 / ARM64 | `sshctl-linux-amd64` / `sshctl-linux-arm64` |
| macOS Intel / Apple Silicon | `sshctl-darwin-amd64` / `sshctl-darwin-arm64` |

Multi-platform release: push a `v*` tag; GitHub Actions builds the six binaries plus `sshctl-skill.zip`.

**General CLI use** (optional, not required for skills):

```bash
go install github.com/Fracizz/sshctl@latest
```

## Quick start

After installing the binary, register a host you administer. Substitute its address, SSH user, and key path. Verify the host key independently and make sure it is recorded in your OpenSSH known_hosts file before connecting.

```bash
sshctl init
sshctl add --name lab --host 192.0.2.20 --user developer --key ~/.ssh/id_ed25519
sshctl search -s lab
sshctl --secure exec lab -- hostname
```

### More examples

```bash
# Add a host (password is encrypted on save)
sshctl add --name lab --host 192.0.2.10 --user root --password 'secret' --desc "lab box"
# Same IP updates in place; omitted flags keep description and ciphertext
sshctl add --host 192.0.2.10 --user administrator

# Or key-based auth
sshctl add --name prod --host 192.0.2.11 --user root --key ~/.ssh/id_ed25519 --desc "prod"

sshctl list
sshctl search -s 192.0.2
sshctl exec lab -- hostname
# multi-arg: each arg is shell-quoted (safe for bash -lc / paths with spaces)
sshctl exec lab -- bash -lc 'cd /tmp && pwd'
# single-arg: passed through so shell operators like && remain intact
sshctl exec lab -- 'cd /tmp && pwd'
sshctl shell lab
sshctl scp ./app.tar.gz lab:/tmp/app.tar.gz
sshctl scp source:/tmp/app.tar.gz test-a:/tmp/app.tar.gz test-b:/tmp/app.tar.gz
sshctl rsync ./app-store prod:/srv/app-store --dry-run
sshctl rsync ./app-store prod:/srv/app-store --delete
```

`rsync` is implemented directly over SFTP, so Windows does not need a local
`rsync` executable or exported SSH credentials. It currently supports
local-to-remote synchronization. Files are compared by size and modification
time by default; use `--checksum` when content identity matters more than scan
speed. `--delete` refuses empty paths and remote roots, but should still be
used only with a dedicated destination directory.

For remote-to-remote copies, `scp` downloads the source once to a private
staging directory under the local ArtifactCache, then uploads the unchanged
files to each destination. Set the root with `--artifact-cache <path>` or
`SSHCTL_ARTIFACT_CACHE`; otherwise the OS user cache directory is used. The
per-run staging directory is removed when the command finishes.

Config path priority: `--config` > `$SSHCTL_CONFIG` > `$SSHFRAC_CONFIG` > `~/.sshctl/servers.json`

### Migrate from sshfrac / invossh

Legacy environment variables (`SSHFRAC_*` and `INVOSSH_*`) remain supported when explicitly set.

If you still have `~/.sshfrac/servers.json` or `~/.invossh/servers.json` and `~/.sshctl/servers.json` does not exist yet:

```bash
sshctl migrate
```

Or run any command (`list`, `exec`, …) — migration runs automatically before the primary config is used.

| Step | Result |
|------|--------|
| Copy | Legacy inventory → `~/.sshctl/servers.json` |
| Backup | Legacy file renamed to `servers.json.bak` in the same directory |
| Priority | First legacy source wins: `.sshfrac`, then `.invossh` |

After migration, all commands use `~/.sshctl/servers.json` only (unless `--config` or `$SSHCTL_CONFIG` / `$SSHFRAC_CONFIG` overrides).

### Master password (enc:v2)

```bash
export SSHCTL_MASTER_PASSWORD='your-strong-secret'
# optional: also bind ciphertext to this machine
export SSHCTL_BIND_MACHINE=1

sshctl add --name lab --host 192.0.2.10 --user root --password 'host-pass' --desc "lab"
sshctl exec lab -- hostname
```

Or pass `--master-password` / `--bind-machine` on each invocation. See [SECURITY.md](SECURITY.md).

### Exit codes & completion

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | local error |
| 2 | usage (documented; reserved) |
| N | remote process status (`exec`) |

```bash
sshctl version
sshctl completion powershell > sshctl.ps1
sshctl completion bash > /etc/bash_completion.d/sshctl
```

Host key verification is skipped by default. Pass `--secure` to check OpenSSH `~/.ssh/known_hosts`.

## JSON format

```json
{
  "servers": [
    {
      "name": "example-host",
      "description": "demo entry",
      "host": "192.0.2.10",
      "port": 22,
      "user": "root",
      "password": "enc:v1:...",
      "os": "Linux"
    }
  ]
}
```

See `configs/servers.example.json` for placeholders. Never commit real passwords.

## Security

Read [SECURITY.md](SECURITY.md). Ciphertext is bound to the local machine user material; moving the file to another PC requires re-entering passwords.

## License

MIT — see [LICENSE](LICENSE).

<a name="sshctl-中文"></a>

[简体中文 README](README.zh-CN.md)
