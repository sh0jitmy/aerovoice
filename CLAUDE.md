# CLAUDE.md

## プロジェクトの概要

このプロジェクトは、航空管制（ATM）向けの欧州VoIP規格 **ED-137** に準拠した、Pure Go実装の音声通信システム（Aerovoice VCS Console & GRS Emulator）です。
本リポジトリには、開発の品質・設計・セキュリティ・可観測性を向上させるための**日本語カスタムスキル（`.claude/skills/`）**が同梱されています。

## AIエージェント（Claude Code）への指示

> [!IMPORTANT]
> - **スキルの厳格な適用**: 本プロジェクトにおけるコードの実装、設計、リファクタリング、およびコードレビューを行う際は、必ず `.claude/skills/` にある各スキル（`golang-lint-governance`、`golang-implementation`、`golang-design`、`golang-htmx-frontend`、`multi-tier-e2e-testing`、`documentation-governance` 等）の指針に準拠してください。
> - **ゼロLintガバナンス**: コード変更後は必ず `make lint` を実行し、全 linter の指摘事項が 0 件（`0 issues`）であることを確認してください（詳細は `golang-lint-governance` スキルを参照）。
> - **言語の統一**: コミットメッセージおよびPRタイトルは**英語**、それ以外のPR説明、Issue、およびAIによるレビューレポートは**完全な日本語**で記述してください。
> - **ライセンスヘッダーの維持**: 新規追加した Go ソースコードには必ず Apache-2.0 ライセンスヘッダーを付与し、`make license-check` をパスさせてください。
> - **Verified Signaturesの厳守**: すべての Git コミットは署名付き（`-S` または `commit.gpgsign=true`）で行い、絶対に `--no-gpg-sign` で回避しないでください。

## 開発・検証コマンド一覧

### Aerovoice 開発 & ローカル実行
- **バイナリビルド (VCS & GRS)**: `make build`
- **VCS コンソール起動**: `make vcs-run` (http://127.0.0.1:8082)
- **GRS エミュレータ起動**: `make grs-run` (http://127.0.0.1:8081)
- **Aerovoice 2画面ライブデモ**: `make demo` (または `make demo-vcs`)
- **Windowsバイナリビルド**: `make build-windows`
- **Windows/macOSクロスコンパイル**: `make build-cross`
- **コードフォーマット**: `make fmt`
- **静的解析の実行**: `make lint`
- **脆弱性スキャンの実行**: `make vulncheck`

### 検証 & 多層 E2E テスト
- **単体テスト & コアロジック網羅検証**: `make test`
- **ED-137 プロトコル検証スイート**: `make aerovoice-test`
- **VCS HTMX フロントエンド E2E**: `make vcs-frontend-e2e`

### 品質・スキル管理タスク
- **カスタムスキルのインストール**: `make install-all` (Claude & Antigravity)
- **プロジェクトの要件セルフチェック**: `make self-eval`
- **スキルの構文検証**: `make check`

