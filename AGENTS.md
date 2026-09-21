# AGENTS.md - Antigravity Agent Guidelines

This document provides behavioral constraints, architectural conventions, and execution workflows for Google Antigravity and other AI coding agents working in this repository.

## Repository Overview

`go_template` is an enterprise-grade Go repository template providing:
1. **Zero-npm Standalone HTMX Dashboard & SSG**: High-performance UI server (`internal/web`) using `//go:embed`, local HTMX (`static/js/htmx.min.js`), and pre-rendered static site export.
2. **SQLite Governance & Reliability**: CGO-free WAL-mode persistence, SHA-256 verified backup archives (`tar.gz`), atomic transactional restore, and retention cleaner.
3. **Multi-Tier E2E Testing Framework**: Fast No-Docker SQLite E2E (`make sqlite-e2e`), Headless Chrome frontend UI assertions (`make frontend-e2e`), and Aerovoice VCS frontend E2E (`make vcs-frontend-e2e`).
4. **Production Observability**: Prometheus metric endpoint (`/metrics`), pprof (`127.0.0.1:6060`), and live signal monitoring.
5. **SSOT Version Management**: Version centrally defined in `internal/version/version.go`, Go version unified across GitHub Actions and `go.mod` via `go-version-file: 'go.mod'`, release automation via GoReleaser v2.

---

## Agent Behavioral Rules

1. **Custom Skills First**:
   - Relevant skills are located in `.agents/skills/` and `.claude/skills/`.
   - Adhere to `golang-lint-governance`, `golang-htmx-frontend`, `golang-sqlite-governance`, `multi-tier-e2e-testing`, and `observability-stack` for architecture, code quality, and implementation decisions.
2. **Zero-Lint Tolerance**:
   - Every code modification, new file, or refactoring must pass `make lint` with **zero issues** before task conclusion.
   - Strictly apply the error handling, concurrency, and security rules defined in `golang-lint-governance` (`defer func() { _ = x.Close() }()`, `copylocks` avoidance via snapshots, `G112` Slowloris timeout, `G301`/`G306` file permissions, `paralleltest`, etc.).
3. **License Header Integrity**:
   - Every Go source file must contain the standard Apache-2.0 and Author header.
   - Always verify with `make license-check` before concluding tasks.
4. **SSOT Principle**:
   - Never hardcode Go versions in GitHub Actions workflows; always use `go-version-file: 'go.mod'`.
   - Version injection must go through `internal/version.Version`.
5. **Test Isolation**:
   - In Go unit/integration tests (`internal/...`), isolate SQLite memory databases using distinct connection names (`fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())`) to ensure parallel safety.
6. **Requirements Compliance**:
   - After updating features or configurations, execute `make self-eval` and ensure 100% compliance in `REQUIREMENTS.md`.
7. **Documentation Synchronization (README & Manual Governance)**:
   - Whenever **user experience (UX)** changes — such as UI interaction flows, audio/visual perceptions, configuration procedures (`configs/*.yaml`, env variables), or troubleshooting behaviors — the agent MUST immediately review and update `README.md` and user manuals (`docs/manual.md`).
   - Adhere to `documentation-governance` skill to eliminate cognitive friction and guarantee 100% consistency between active runtime behavior and user documentation.
8. **Verified Signatures Required for Git Commits**:
   - Every Git commit must be signed with a verified signature (`commit.gpgsign=true` or `-S`) to earn the GitHub "Verified" badge and satisfy branch protection rules.
   - **Never bypass commit signing with `--no-gpg-sign`**.
   - If an SSH or GPG signing key requires an interactive passphrase that is unavailable in non-interactive agent execution, prompt the user to load the key into `ssh-agent` or commit via an interactive terminal rather than committing without signatures.

---

## Standard Development & Testing Commands

```bash
# Code generation & formatting
make generate
make fmt
make lint

# Verification & Multi-Tier E2E
make test               # Unit tests with -race and coverage check
make sqlite-e2e         # Ultra-fast No-Docker SQLite E2E
make frontend-e2e       # Headless Chrome HTMX UI & snapshot verification
make vcs-frontend-e2e   # Aerovoice VCS HTMX UI & report verification
make aerovoice-test     # Aerovoice ED-137 protocol test suite
make ssg-build          # Static site generation export

# Release & Governance
make release-check   # Validate GoReleaser v2 configuration
make license-check   # Verify license headers
make check           # Validate skill frontmatter
make self-eval       # Update REQUIREMENTS.md checklist & compliance score
```
