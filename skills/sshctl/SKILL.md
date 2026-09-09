---
name: sshctl
description: |
  sshctl 远程主机 CLI（清单 search/exec/shell/scp/rsync/add/migrate/skills）。优先 sshctl，尽量不用原生 ssh/scp/rsync。
  二进制与技能同目录（SKILL.md 所在文件夹下的 bin/sshctl.exe），不安装到系统 PATH。
  触发词：sshctl、search -s、exec、shell、scp、rsync、skills、servers.json、SSHCTL、.sshctl。
---

# sshctl · 远程主机 CLI（Windows）

**默认用 sshctl 做所有远程操作**；不要手写 `ssh` / `scp` / `sshpass`（配免密见 `ssh-key-auth-setup` 技能）。

**Agent 必须**通过技能目录下的二进制调用，**不要**假设 `sshctl` 在系统 PATH 中。

```powershell
# skillRoot = 本 SKILL.md 所在目录（从附带技能文件路径解析，禁止硬编码仓库路径）
$skillRoot = '...'  # 例：skills/sshctl/ 或 %USERPROFILE%\.claude\skills\sshctl\
$sshctl = Join-Path $skillRoot 'bin\sshctl.exe'
& $sshctl version
```

| 操作 | 命令 |
|------|------|
| 搜主机 | `& $sshctl search -s <关键词>` |
| 远程执行 | `& $sshctl --insecure exec <host> -- <cmd>` |
| 交互 shell | `& $sshctl --insecure shell <host>` |
| 传文件 | `& $sshctl --insecure scp <src> <dst>` |
| 远端中转/多目标 | `& $sshctl --insecure scp <remote-src> <remote-dst> [remote-dst...]` |
| 增量同步 | `& $sshctl --insecure rsync <local-src> <host:remote-dst>` |
| 写入清单 | `& $sshctl add --host ... --user ... --password '...'` |
| 清单迁移 | `& $sshctl migrate` |
| 列 skills | `& $sshctl skills` / `& $sshctl skills -s sshctl` |

---

## 二进制（与 skill 同目录）

| 项 | 路径 |
|----|------|
| skillRoot | 本 `SKILL.md` 所在文件夹 |
| 二进制 | `$skillRoot\bin\sshctl.exe` |

### 构建 / 更新

在仓库根目录交叉编译 6 平台可执行文件，同步到仓库 skill 与已存在的 `.claude` / `.cursor` / `.codex` skill `bin\`：

```powershell
$env:VERSION = '0.4.0'
.\scripts\build.ps1
```

`bin/` 下二进制 **不入库**。也可从 [Releases](https://github.com/Fracizz/sshctl/releases) 获取：

| 资源 | 说明 |
|------|------|
| `sshctl-skill.zip` | **AI skills 整包**：解压到 `~/.claude/skills/` / `~/.cursor/skills/` / `~/.codex/skills/` |
| `sshctl-windows-amd64.exe` | Windows x64（Agent 默认可作 `bin/sshctl.exe`） |
| `sshctl-windows-arm64.exe` | Windows ARM64 |
| `sshctl-linux-amd64` / `sshctl-linux-arm64` | Linux x64 / ARM64 |
| `sshctl-darwin-amd64` / `sshctl-darwin-arm64` | macOS Intel / Apple Silicon |

多平台发布：推送 `v*` 标签，由 GitHub Actions 构建裸二进制 + `sshctl-skill.zip`。

### 验证

```powershell
& $sshctl version    # 0.4.0+
& $sshctl skills -s sshctl
& $sshctl list
```

---

## 配置

| 项 | 路径 / 变量 |
|----|-------------|
| 清单 | `%USERPROFILE%\.sshctl\servers.json` |
| 迁移 | `& $sshctl migrate`（`~/.sshfrac` → `~/.sshctl`，旧文件改 `.bak`） |
| 覆盖 | `$SSHCTL_CONFIG` |
| Legacy | `$SSHFRAC_CONFIG`（显式指定时） |
| 主密码（可选） | `--master-password` / `$SSHCTL_MASTER_PASSWORD` → 才走 `enc:v2`；默认不需要 |
| 机器绑定（仅 v2） | `--bind-machine` / `$SSHCTL_BIND_MACHINE=1`；`enc:v1` 已本机绑定 |

**规则：** 每个 IP 仅一条；`add` 同 IP 覆盖；密码特殊字符须完整引号包裹。

### 清单密码加密（对称）

清单里的 SSH 密码以对称加密存放（AES-GCM），不入库明文。

| 方案 | 条件 | 说明 |
|------|------|------|
| `enc:v1`（**默认推荐**） | 不设主密码 | 密钥由本机材料派生；**本机无感**；把 `servers.json` 拷到其他机器会解密失败 |
| `enc:v2` | 显式设主密码 | Argon2id + AES-GCM；适合多用户/共享机，需每次能拿到主密码 |
| `enc:v2` + 机器绑定 | 另开 `--bind-machine` | 换机即使主密码相同也不可解 |

**设计意图（默认路径）：** 加密只为「分享/拷贝 JSON 到别的机器失效」；日常本机 `add` / `exec` / `scp` / `rsync` **不要**向用户要主密码，也不要设 `SSHCTL_MASTER_PASSWORD`。

**Agent 策略：**

1. **默认用 `enc:v1`**：不设主密码、不问主密码、不设 `SSHCTL_BIND_MACHINE`（v1 已绑定本机）。
2. **不要**把 `enc:v2` 当默认；仅当用户明确要求「主密码 / enc:v2 / 更强保护」时再启用。
3. 若环境里**已经**有 `SSHCTL_MASTER_PASSWORD`（用户自管），则沿用 `enc:v2`，读写保持一致；仍勿把密码写入仓库或 skill。
4. 已是 `enc:v2` 的条目缺主密码时：提示补 env 或对该主机重新 `add`（可改回无感的 v1）；**不要**反复追问「要不要开加密」。

```powershell
# 默认无感（enc:v1，拷到别的机器失效）
& $sshctl add --host 192.168.x.x --user root --password '...' --desc "..."
& $sshctl --insecure exec 192.168.x.x -- "hostname"

# 仅用户明确要求主密码时
$env:SSHCTL_MASTER_PASSWORD = '...'   # 用户自管，勿入库
$env:SSHCTL_BIND_MACHINE = '1'        # 可选
& $sshctl add --host 192.168.x.x --user root --password '...' --desc "..."
```

换机后旧 `enc:v1` / 绑定机的 `enc:v2` 密文无法解密，需在新机器重新 `add`。

---

## 常用命令

```powershell
& $sshctl migrate
& $sshctl list
& $sshctl search -s 192.168
& $sshctl add --host 192.168.x.x --user administrator --password '...' --os Windows --desc "说明"
& $sshctl --insecure exec 192.168.x.x -- "hostname && whoami"
# 多参数会做 shell quote，可安全使用 bash -lc
& $sshctl --insecure exec 192.168.x.x -- bash -lc 'cd /tmp && pwd'
& $sshctl --insecure scp .\a.txt 192.168.x.x:C:/temp/a.txt
& $sshctl --insecure scp source:/tmp/a.tar target-a:/tmp/a.tar target-b:/tmp/a.tar --artifact-cache Y:\ArtifactCache
& $sshctl --insecure rsync .\app-store 192.168.x.x:/srv/app-store --dry-run
& $sshctl --insecure rsync .\app-store 192.168.x.x:/srv/app-store --delete
```

大目录或重复发布优先使用 `rsync`：它复用加密清单，通过 SFTP 原生比较大小和修改时间，
只上传变化文件。内容级验收可加 `--checksum`；机器可读汇总用 `--json`。`--delete`
会拒绝空路径和远端根目录，但仍只允许用于明确的专用目标目录。

`scp` 支持远端到远端及多个远端目标：源内容只下载一次到本机 ArtifactCache，再原样
上传到各目标。缓存根目录可用 `--artifact-cache` 或 `SSHCTL_ARTIFACT_CACHE` 指定；
未指定时使用系统用户缓存目录，命令结束后清理本次暂存目录。

**Agent 流程：** `search -s` → 不在清单则 `add`（确认凭据；**默认 enc:v1 无感**，勿要主密码）→ **默认加 `--insecure`** 再 `exec` / `scp` / `rsync`（不要问用户）。

---

## 首次连接 / host key

**默认允许 `--insecure`：** 跳过 host key 校验。CLI 默认已是跳过；Agent 仍应显式加上，兼容旧二进制。sshctl **不会**交互确认或自动写入 `known_hosts`。

| 场景 | 处理 |
|------|------|
| 默认 / Agent 日常 | `& $sshctl --insecure exec <host> -- "..."`（跳过校验；**不**写入 known_hosts） |
| 用户明确要求校验 | `& $sshctl --secure exec <host> -- "..."`（对照 `~/.ssh/known_hosts`） |

```powershell
# 默认：显式 --insecure
& $sshctl --insecure exec 192.168.x.x -- "hostname && uname -r"
# 仅用户要求校验 host key 时
& $sshctl --secure exec 192.168.x.x -- "hostname"
```

`--insecure` 与 CLI 默认行为相同；`--secure` 才启用 known_hosts。两者互斥。

---

## 错误速查

| 情况 | 处理 |
|------|------|
| 找不到 sshctl | 构建/复制到 `$skillRoot\bin\sshctl.exe` |
| duplicate host | `add` 同 IP 覆盖，或删 JSON 重复项 |
| Windows 密码失败 | 确认密码完整；`--os Windows`；v0.2.1+ |
| `bash -lc` 路径错乱 | 需 v0.2.3+（多参数已 shell quote） |
| unknown host key / Handshake failed | 去掉 `--secure`（默认跳过校验），或先用 OpenSSH 写入 known_hosts |
| decrypt failed / enc:v2 requires master password | 补上相同主密码与 `--bind-machine` 设置；或重新 `add` |
| 仍用旧清单目录 | `& $sshctl migrate` |

## 边界

- 远程操作只用 `$sshctl`，不用原生 ssh/scp/rsync
- **不**安装到系统 PATH（技能工作流）
- 配免密 → **ssh-key-auth-setup**
- 清单不入库；勿把主密码写入 skill / 仓库
- 加密默认 `enc:v1`（本机无感、拷贝 JSON 换机失效）；`enc:v2` 仅用户明确要求时用
- host key **默认允许 `--insecure`**（连接时加上，勿问用户）；仅用户明确要求时改用 `--secure`
