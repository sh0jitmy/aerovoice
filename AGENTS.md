# AGENTS.md - Antigravity Agent Guidelines

This document provides behavioral constraints, architectural conventions, and execution workflows for Google Antigravity and other AI coding agents working in this repository.

## Repository Overview

`aerovoice` is an enterprise-grade pure Go implementation of the ED-137 European aviation VoIP standard for Air Traffic Management (ATM):
1. **Zero-npm Standalone HTMX Web Consoles**: High-performance VCS (`internal/vcs`) and GRS (`internal/grs`) consoles using `//go:embed` and local HTMX (`static/js/htmx.min.js`) for real-time frequency selection, PTT control, and audio visualization.
2. **Pure Go & Zero-DB In-Memory Architecture**: Zero CGO, zero external database dependencies. In-memory session tracking, ring-buffer logging, and audio quota management for deterministic low-latency audio transmission.
3. **ED-137B/C Compliance**: Complete SIP (`sipgo`), SDP (`pion/sdp`), and RTP (`pion/rtp`) stack supporting G.711 μ-law, dynamic jitter buffering, and ED-137 radio header extensions (PTT/Squelch/RSSI).
4. **Multi-Tier E2E Testing Framework**: Core protocol unit tests (`make test`), ED-137 protocol integration suite (`make aerovoice-test`), and Headless Chrome VCS frontend assertions (`make vcs-frontend-e2e`).
5. **SSOT Version Management**: Version centrally defined in `internal/version/version.go`, Go version unified across GitHub Actions and `go.mod` via `go-version-file: 'go.mod'`, release automation via GoReleaser v2.

---

## Agent Behavioral Rules

1. **Custom Skills First**:
   - Relevant skills are located in `.agents/skills/` and `.claude/skills/`.
   - Adhere to `golang-lint-governance`, `golang-htmx-frontend`, `golang-design`, `golang-implementation`, `multi-tier-e2e-testing`, and `documentation-governance` for architecture, code quality, and implementation decisions.
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
   - In Go unit/integration tests (`internal/...`), ensure goroutine leak detection with `goleak` and independent port allocation for network listeners to guarantee parallel test safety (`t.Parallel()`).
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
make test               # Unit tests with -race and core package coverage check
make aerovoice-test     # Aerovoice ED-137 protocol test suite
make vcs-frontend-e2e   # Aerovoice VCS HTMX UI & report verification
make build              # Build bin/vcs and bin/grs-emulator
make build-windows      # Build Windows (x86_64) binaries in bin/dist/
make build-cross        # Cross-compile for Windows & macOS (CGO_ENABLED=0)

# Execution & Demo
make vcs-run            # Run VCS Console (http://127.0.0.1:8082)
make grs-run            # Run GRS Emulator (http://127.0.0.1:8081)
make demo               # Launch interactive 2-screen demo

# Release & Governance
make release-check   # Validate GoReleaser v2 configuration
make license-check   # Verify license headers
make check           # Validate skill frontmatter
make self-eval       # Update REQUIREMENTS.md checklist & compliance score
```
