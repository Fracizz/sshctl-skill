# Security Policy

## Supported versions

Security fixes are applied to the latest `main` branch.

## Reporting a vulnerability

Please open a private GitHub security advisory, or contact the maintainer via https://github.com/Fracizz.

Do **not** open a public issue for credential leaks or remote code execution.

## Threat model

### Assets

- SSH passwords stored in `~/.sshctl/servers.json`
- Private key paths referenced by `key_file` (keys themselves live on disk)
- Ability to run remote commands as configured users

### Trust boundaries

| Attacker | What they can do |
|----------|------------------|
| Network eavesdropper | Cannot read SSH session contents (SSH/SFTP). Default `--insecure` skips host key checks (no MITM resistance). `--secure` verifies against `known_hosts`. |
| Same OS user, no master password | Can decrypt **enc:v1** inventory (machine-derived key). |
| Same OS user, with master password | Needs `SSHCTL_MASTER_PASSWORD` / `--master-password` (and matching `--bind-machine` if used) to decrypt **enc:v2**. |
| Other OS users | Should not read `servers.json` (mode 0600) or your private keys if permissions are correct. |
| Malware as your user | Can read env, keys, and drive the CLI — treat the workstation as trusted. |

### Encryption schemes

| Prefix | KDF / key | When used |
|--------|-----------|-----------|
| `enc:v1:` | SHA-256(machine material) → AES-256-GCM | Default when no master password is set (legacy / convenience). |
| `enc:v2:` | Argon2id(master password [+ optional machine bind], random salt) → AES-256-GCM | When `--master-password` or `SSHCTL_MASTER_PASSWORD` is set. |

**Recommendations**

1. **Default / agents:** use **enc:v1** (no master password). Local use is seamless; copying `servers.json` to another machine fails to decrypt — that is the intended anti-share behavior.
2. Prefer **enc:v2** with a strong master password only on shared or multi-user machines, or when the user explicitly wants a portable secret they control.
3. Optionally set `--bind-machine` / `SSHCTL_BIND_MACHINE=1` with enc:v2 so ciphertext will not decrypt on another host even with the same master password.
4. Agents must not ask for a master password by default; if using enc:v2, pass it via env, never commit it.
5. Default connections skip host key verification. Use `--secure` on untrusted networks after `known_hosts` is populated.
6. Prefer SSH public keys (`key_file`) over passwords when possible.

OS keychain integration is not built-in; wrap sshctl with a script that loads the secret from your keychain into `SSHCTL_MASTER_PASSWORD`.
