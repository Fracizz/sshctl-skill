# sshctl

[English](README.md) · **简体中文**

在本地主机清单中搜索机器，打开 SSH 终端、运行命令或传输文件，下次直接使用保存的连接信息。`sshctl` 会加密保存密码，提供适合 Agent 和脚本使用的命令参数、输出格式和退出码。

[下载](https://github.com/Fracizz/sshctl-skill/releases/latest) · [快速开始](#快速开始) · [Agent Skill](skills/sshctl/SKILL.md) · [安全说明](SECURITY.md)

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
git clone https://github.com/Fracizz/sshctl-skill.git
cd sshctl-skill
go build -o skills/sshctl/bin/sshctl.exe .
# 或 .\scripts\build.ps1
```

技能说明：[skills/sshctl/SKILL.md](skills/sshctl/SKILL.md) — `$skillRoot` 为本 `SKILL.md` 所在目录，通过 `$sshctl = Join-Path $skillRoot 'bin\sshctl.exe'` 调用。

`skills/sshctl/bin/` 不入库；克隆后需本地构建，或从 [Releases](https://github.com/Fracizz/sshctl-skill/releases) 下载：
- **`sshctl-skill.zip`**（AI skills 整包）：`sshctl skills install --zip` overlay 到已有技能目录，不要先删文件夹
- 或单平台裸二进制（Windows 用 `sshctl-windows-amd64.exe`，Agent 默认可作 `bin/sshctl.exe`）

**通用 CLI（可选）：**

```bash
go install github.com/Fracizz/sshctl@latest
```

## 快速开始

安装后，把示例地址、SSH 用户和密钥路径换成你管理的主机信息。连接前先通过可信渠道核实主机身份，并将公钥记录到 OpenSSH 的 known_hosts 文件。

```bash
sshctl init
sshctl add --name lab --host 192.0.2.20 --user developer --key ~/.ssh/id_ed25519
sshctl search -s lab
sshctl --secure exec lab -- hostname
```

### 更多命令示例

```bash
sshctl add --name lab --host 192.0.2.10 --user root --password 'secret' --desc "实验机"
# 同 IP 合并更新：没传的描述、密文、密钥等会保留
sshctl add --host 192.0.2.10 --user administrator
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

显式设置的旧环境变量 `SSHFRAC_*` 和 `INVOSSH_*` 仍然有效。

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
