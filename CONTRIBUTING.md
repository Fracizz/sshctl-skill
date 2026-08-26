# Contributing

Thanks for contributing to sshctl.

## Development

```bash
go test ./...
go vet ./...
go build -o bin/sshctl .
```

Windows local build (amd64 only; syncs skill bins):

```powershell
powershell -File scripts/build.ps1
```

Optional lint (same as CI):

```bash
golangci-lint run
```

Multi-platform release binaries (GitHub Actions on tag):

```bash
# bump Version in cmd/root.go / scripts/build.ps1 as needed, then:
git tag -a v0.3.0 -m "sshctl v0.3.0"
git push origin v0.3.0
# → .github/workflows/release.yml tests, builds 6 executables + sshctl-skill.zip, creates GitHub Release
# Assets: sshctl-{linux,darwin,windows}-{amd64,arm64}[.exe] and sshctl-skill.zip
```

CI on `main` / PRs still uploads per-platform build artifacts.

## Pull requests

- Keep changes focused.
- Add/adjust tests for config, search, and crypto when behavior changes.
- Never commit `~/.sshctl/servers.json`, `~/.sshfrac/servers.json`, `bin/`, `dist/`, `skills/*/bin/`, or real credentials.
- Prefer English for new CLI help strings; README may stay bilingual.
- Run `go test ./...` before opening a PR.

## Code style

`gofmt` / `go fmt ./...`.
