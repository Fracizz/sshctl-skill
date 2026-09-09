# sshctl

> Formerly `sshfrac` / `invossh`. On first use, legacy `~/.sshfrac/servers.json` (or `~/.invossh/`) is **copied to** `~/.sshctl/servers.json` and the old file is renamed to `servers.json.bak`. Legacy env vars (`SSHFRAC_*`, `INVOSSH_*`) still work when set explicitly.

**English** | [中文](#sshctl-中文)

High-performance cross-platform SSH/SFTP sync CLI with an encrypted local server inventory.

> **Built primarily for AI agents** (Cursor, Claude Code, Codex, and similar).
> Stable, scriptable subcommands and JSON inventory make remote exec / SCP easy for tools to call without interactive prompts.

Humans can use it too — the UX prioritizes machine-friendly flags, exit codes, and search (`-s`) over a rich TUI.

Agent skill (Cursor / Claude / Codex): [skills/sshctl/SKILL.md](skills/sshctl/SKILL.md)

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
git clone https://github.com/Fracizz/sshctl.git
cd sshctl
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

`skills/sshctl/bin/` is gitignored; clone from source, or download from [Releases](https://github.com/Fracizz/sshctl/releases):

| Asset | Notes |
|-------|--------|
| `sshctl-skill.zip` | **AI skill pack** — unzip into `~/.claude/skills/`, `~/.cursor/skills/`, or `~/.codex/skills/` |
| Windows x64 / ARM64 | `sshctl-windows-amd64.exe` / `sshctl-windows-arm64.exe` |
| Linux x64 / ARM64 | `sshctl-linux-amd64` / `sshctl-linux-arm64` |
| macOS Intel / Apple Silicon | `sshctl-darwin-amd64` / `sshctl-darwin-arm64` |

Multi-platform release: push a `v*` tag; GitHub Actions builds the six binaries plus `sshctl-skill.zip`.

**General CLI use** (optional, not required for skills):

```bash
go install github.com/Fracizz/sshctl@latest
```

## Quick start

```bash
# Create ~/.sshctl/servers.json (placeholder only — no real secrets)
sshctl init

# Add a host (password is encrypted on save)
sshctl add --name lab --host 192.0.2.10 --user root --password 'secret' --desc "lab box"

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

---

# sshctl (中文)

高性能跨平台 SSH/SCP 命令行工具，带本地加密主机清单。

> **主要面向 AI 编程助手使用**（Cursor、Claude Code、Codex 等）。
> 子命令稳定、可脚本化，配合 JSON 主机清单，方便 Agent 非交互地执行远程命令与文件传输。

人工使用同样可以；交互式体验让位于：机器友好的参数、退出码，以及 `-s` 模糊搜索。

## 特性

- 单文件二进制：Linux / Windows / macOS（amd64 + arm64）
- 远程命令执行、交互 Shell
- 基于 SFTP 的高速传文件（`scp` 子命令，支持目录）
- 原生增量上传（`rsync`），支持预演、SHA-256 比较和受限删除
- 密码落盘 AES-256-GCM 加密（`enc:v1:`）
- 对 name / IP / description 不区分大小写的包含搜索
- 默认跳过 host key 校验（`--insecure`）；需要校验时加 `--secure` 对照 `~/.ssh/known_hosts`
- 配置默认在仓库外：`~/.sshctl/servers.json`
- 从 `~/.sshfrac` / `~/.invossh` 一次性迁移（自动或 `sshctl migrate`）

## 为何适合 AI？

| 需求 | sshctl 的做法 |
|------|----------------|
| 非交互 | `exec` / `scp` / `rsync` / `list` / `search`，常见流程不弹密码提示 |
| 找机器 | `search -s <关键词>` 匹配名称、IP、描述 |
| 密钥/密码 | 密码加密存储；Agent 仅在 `add` 时传入一次 `--password` |
| 输出稳定 | 表格列表；远端失败返回非 0 退出码 |

## 安装

**AI Agent（推荐）：** 二进制与技能同目录，**不**写入系统 PATH。

```powershell
git clone https://github.com/Fracizz/sshctl.git
cd sshctl
go build -o skills/sshctl/bin/sshctl.exe .
# 或 .\scripts\build.ps1
```

技能说明：[skills/sshctl/SKILL.md](skills/sshctl/SKILL.md) — `$skillRoot` 为本 `SKILL.md` 所在目录，通过 `$sshctl = Join-Path $skillRoot 'bin\sshctl.exe'` 调用。

`skills/sshctl/bin/` 不入库；克隆后需本地构建，或从 [Releases](https://github.com/Fracizz/sshctl/releases) 下载：
- **`sshctl-skill.zip`**（AI skills 整包）：解压到 `~/.claude/skills/` / `~/.cursor/skills/` / `~/.codex/skills/`
- 或单平台裸二进制（Windows 用 `sshctl-windows-amd64.exe`，Agent 默认可作 `bin/sshctl.exe`）

**通用 CLI（可选）：**

```bash
go install github.com/Fracizz/sshctl@latest
```

## 快速开始

```bash
sshctl init
sshctl add --name lab --host 192.0.2.10 --user root --password 'secret' --desc "实验机"
sshctl add --name prod --host 192.0.2.11 --user root --key ~/.ssh/id_ed25519 --desc "生产"

sshctl list
sshctl search -s 192.0.2
sshctl exec lab -- hostname
# 多参数会逐项 shell quote，可安全使用 bash -lc / 含空格路径
sshctl exec lab -- bash -lc 'cd /tmp && pwd'
# 单参数原样传递，保留 && 等 shell 语法
sshctl exec lab -- 'cd /tmp && pwd'
sshctl shell lab
sshctl scp ./app.tar.gz lab:/tmp/app.tar.gz
sshctl scp source:/tmp/app.tar.gz test-a:/tmp/app.tar.gz test-b:/tmp/app.tar.gz
sshctl rsync ./app-store prod:/srv/app-store --dry-run
sshctl rsync ./app-store prod:/srv/app-store --delete
```

`rsync` 直接基于 SFTP 实现，Windows 无需安装本地 `rsync`，也无需导出 SSH
凭据。当前支持从本地同步到远端；默认按大小和修改时间判断变化，需要内容级确认时加
`--checksum`。`--delete` 会拒绝空路径和远端根目录，但仍应只对专用目标目录使用。

远端到远端复制时，`scp` 会把源内容下载一次到本机 ArtifactCache 的独立暂存目录，
再将内容不变地依次上传到所有目标。可通过 `--artifact-cache <路径>` 或
`SSHCTL_ARTIFACT_CACHE` 指定缓存根目录；未指定时使用操作系统的用户缓存目录。
命令结束后会清理本次暂存目录。

配置优先级：`--config` > `$SSHCTL_CONFIG` > `$SSHFRAC_CONFIG` > `~/.sshctl/servers.json`

### 从 sshfrac / invossh 迁移

若仍有 `~/.sshfrac/servers.json` 或 `~/.invossh/servers.json`，且尚未存在 `~/.sshctl/servers.json`：

```bash
sshctl migrate
```

或直接运行 `list`、`exec` 等命令——在使用主配置前会**自动迁移**。

| 步骤 | 结果 |
|------|------|
| 复制 | 旧清单 → `~/.sshctl/servers.json` |
| 备份 | 原文件同目录重命名为 `servers.json.bak` |
| 顺序 | 优先 `.sshfrac`，其次 `.invossh` |

迁移完成后默认只用 `~/.sshctl/servers.json`（除非 `--config` 或环境变量指定其他路径）。

### 主密码（enc:v2）

```bash
export SSHCTL_MASTER_PASSWORD='强密码'
export SSHCTL_BIND_MACHINE=1   # 可选：绑定本机

sshctl add --name lab --host 192.0.2.10 --user root --password 'host-pass' --desc "实验机"
```

详见 [SECURITY.md](SECURITY.md)。

### 退出码与补全

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | 本地错误 |
| N | 远端进程状态（`exec`） |

```bash
sshctl version
sshctl completion powershell > sshctl.ps1
```

默认跳过 host key 校验。需要校验时加 `--secure`，对照 OpenSSH `~/.ssh/known_hosts`。

## 安全说明

详见 [SECURITY.md](SECURITY.md)。密文与本机用户环境绑定，换机器需重新录入密码。

## 许可证

MIT — 见 [LICENSE](LICENSE)。


