# Makefile for Go Development & Custom Skills Management

.PHONY: help check install install-agents install-all self-eval generate test fmt lint tidy vulncheck build vcs-build grs-build vcs-run grs-run aerovoice-test build-cross vcs-frontend-e2e demo demo-vcs release-check release-snapshot license-check license-add clean publish-pr ai-pr

help:
	@echo "Available commands:"
	@echo "  Aerovoice Development & Testing:"
	@echo "    build            Build Aerovoice VCS and GRS binaries (bin/vcs, bin/grs-emulator)"
	@echo "    vcs-run          Start Aerovoice VCS Console (http://127.0.0.1:8082)"
	@echo "    grs-run          Start Aerovoice GRS Emulator (http://127.0.0.1:8081)"
	@echo "    demo             Launch Aerovoice VCS interactive 2-screen demo"
	@echo "    test             Run Go tests with race detector and business logic coverage"
	@echo "    aerovoice-test   Run Aerovoice ED-137 Radio & Telephony verification test suite"
	@echo "    vcs-frontend-e2e Run Aerovoice VCS HTMX frontend E2E test & report suite"
	@echo "    fmt              Format Go source files"
	@echo "    lint             Run golangci-lint static analysis"
	@echo "    tidy             Run go mod tidy"
	@echo "    vulncheck        Run govulncheck vulnerability scanner"
	@echo "    build-cross      Cross-compile Pure-Go binaries for Windows and macOS"
	@echo "    release-check    Validate GoReleaser configuration"
	@echo "    release-snapshot Run GoReleaser snapshot build"
	@echo "    license-check    Verify license & author headers in Go files"
	@echo "    license-add      Automatically add license headers to Go files"
	@echo "    publish-pr       Verify formatting/lints/tests, push to origin, and create GitHub PR"
	@echo "    ai-pr            Trigger AI agent to draft a GitHub PR in Japanese"
	@echo "  Custom Skills Management:"
	@echo "    check            Validate custom skill frontmatter and syntax"
	@echo "    install          Install custom skills globally to ~/.claude/skills/"
	@echo "    install-agents   Install/Sync custom skills to .agents/skills/ (Antigravity)"
	@echo "    install-all      Install custom skills to both Claude and Antigravity"
	@echo "    self-eval        Run requirements self-evaluation and update checklist"
	@echo "  General:"
	@echo "    clean            Clean up build artifacts and temporary files"

# --- Go Development ---

generate:
	@echo "==> Running code generators..."
	@go generate ./...

fmt:
	@echo "==> Formatting Go source files..."
	@go fmt ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix ./...; \
	fi

lint:
	@echo "==> Running golangci-lint..."
	@golangci-lint run ./...

tidy:
	@echo "==> Tidying Go modules..."
	@go mod tidy

vulncheck:
	@echo "==> Running govulncheck..."
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

test:
	@bash scripts/check_coverage.sh

build: vcs-build grs-build

vcs-build:
	@echo "==> Building Aerovoice VCS Console..."
	@mkdir -p bin
	@go build -v -o bin/vcs ./cmd/vcs

grs-build:
	@echo "==> Building Aerovoice GRS Emulator..."
	@mkdir -p bin
	@go build -v -o bin/grs-emulator ./cmd/grs-emulator

vcs-run:
	@echo "==> Starting Aerovoice VCS Console on http://127.0.0.1:8082..."
	@go run ./cmd/vcs -config configs/vcs.yaml

grs-run:
	@echo "==> Starting Aerovoice GRS Emulator on http://127.0.0.1:8081..."
	@go run ./cmd/grs-emulator -config configs/grs.yaml

demo-vcs:
	@echo "==> Launching Aerovoice VCS & GRS Interactive 2-Screen Demo..."
	@bash scripts/aerovoice_demo.sh

aerovoice-test:
	@echo "==> Running Aerovoice ED-137 Radio & Telephony Verification Test Suite..."
	@go test -v ./internal/ed137 ./internal/media/... ./internal/sip ./internal/channel ./internal/pcap ./internal/vcs ./internal/grs ./test/e2e

build-cross:
	@echo "==> Cross-compiling for Windows and macOS (CGO_ENABLED=0 Pure Go)..."
	@mkdir -p bin/dist
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bin/dist/vcs-windows-amd64.exe ./cmd/vcs
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bin/dist/grs-windows-amd64.exe ./cmd/grs-emulator
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/dist/vcs-darwin-arm64 ./cmd/vcs
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/dist/grs-darwin-arm64 ./cmd/grs-emulator
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/dist/vcs-darwin-amd64 ./cmd/vcs
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/dist/grs-darwin-amd64 ./cmd/grs-emulator
	@echo "✅ Cross-compilation completed in bin/dist/"

vcs-frontend-e2e: vcs-build grs-build
	@echo "==> Running Aerovoice VCS HTMX Frontend E2E test suite..."
	@bash scripts/vcs_frontend_e2e.sh

demo: demo-vcs

release-check:
	@echo "==> Validating GoReleaser configuration..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		go run github.com/goreleaser/goreleaser/v2@latest check; \
	fi

release-snapshot:
	@echo "==> Building GoReleaser snapshot..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean; \
	fi

license-check:
	@echo "==> Checking Go source files license headers..."
	@python3 scripts/check_license.py --check

license-add:
	@echo "==> Adding license headers to Go source files..."
	@python3 scripts/check_license.py --add

publish-pr:
	@bash scripts/publish_pr.sh

ai-pr:
	@claude "github-pr-creator スキルを使用して、現在のブランチの変更とコミットログを分析し、pull_request_template.md に従って日本語のプルリクエストをドラフト（下書き）で作成してください。"

# --- Custom Skills Management ---

check:
	@echo "==> Validating skill files format..."
	@python3 scripts/check_skills.py

install:
	@echo "==> Installing custom skills globally to ~/.claude/skills/..."
	@mkdir -p ~/.claude/skills/
	@cp -R .claude/skills/* ~/.claude/skills/
	@echo "Claude skills successfully installed!"

install-agents:
	@echo "==> Syncing custom skills to .agents/skills/ (Antigravity)..."
	@mkdir -p .agents/skills/
	@cp -R .claude/skills/* .agents/skills/
	@echo "Antigravity skills successfully synced!"

install-all: install install-agents
	@echo "All custom skills successfully installed for Claude and Antigravity!"

self-eval:
	@echo "==> Running self-evaluation..."
	@python3 scripts/self_eval.py

# --- General ---

clean:
	@echo "==> Cleaning up build artifacts..."
	@rm -rf bin/ dist/ test_reports/
	@go clean -testcache
