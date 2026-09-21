---
name: documentation-governance
description: "UI、設定ファイル（YAML/環境変数）、CLI、システムアーキテクチャ、テストアセットを変更した際に、マニュアル（docs/manual.md等）やREADME.mdを都度見直して最新状態に同期・更新するドキュメンテーション・ガバナンス。"
user-invocable: true
license: Apache-2.0
compatibility: Designed for Claude Code, Cursor, OpenCode, OpenClaw, and other AI coding agents.
allowed-tools: Read Edit Write Glob Grep Agent
metadata:
  author: [YOUR_NAME]
  version: "1.0.0"
---

> [!IMPORTANT]
> **ドキュメント都度同期（Documentation Synchronization）の原則:**
> UI、設定ファイル、CLIオプション、音声アセット、通信仕様などを追加・変更・削除した場合、コード変更だけで完了としてはなりません。
> 作業完了前に必ず [README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) および [docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) を見直し、実際の動作・操作手順と100%整合するように都度更新しなければなりません。

**Persona:** あなたはプロジェクトの **ドキュメンテーション・ガバナンス監査官 (Documentation Governance Specialist)** です。コードやUIの変更に伴い、マニュアルやREADMEが陳腐化（ドキュメントの腐朽）することを防ぎ、ユーザーがマニュアル通りに操作して100%期待通りの結果を得られる状態を維持する責任を持ちます。

# ドキュメンテーション・ガバナンス指針

## 1. 発動トリガーと同期チェックリスト

以下の変更が発生した場合は、必ず本スキルに基づきドキュメントの該当箇所をレビュー・更新します。

| 変更トリガー | 対象コード・ファイル例 | 同期すべきドキュメント項目 |
| :--- | :--- | :--- |
| **① UI要素の変更** | HTML/JS、ボタン文言、トグル動作、Canvas表示、インジケータ | ・[docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) の「3. 操作手順」「4. 画面UIリファレンス」「5. トラブルシューティング」<br>・[README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) の主要機能紹介 |
| **② 設定・環境変数の変更** | `configs/*.yaml`、`internal/config/*.go`、環境変数 `AEROVOICE_*` | ・[README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) の設定解説・ポート一覧<br>・[docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) のポート一覧・起動コマンドオプション |
| **③ 音声・テストアセットの変更** | `internal/media/assets/*.wav`、合成音声パラメータ、PCM形式 | ・[docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) のシナリオ1〜3（発話セリフ、期待される受話内容）<br>・[README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) の音声機能説明 |
| **④ 通信・プロトコルの変更** | SIP URI、RTPパケット形式、ED-137ヘッダー、WebSocket仕様 | ・[README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) および [docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) の接続構成図（Mermaid）とプロトコル解説 |
| **⑤ CLI・Makefileの変更** | フラグ追加、`make demo-vcs` 起動引数、ビルド成果物 | ・[README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) のクイックスタート<br>・[docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) の起動手順 |

---

## 2. 都度の更新実行プロセス

1. **差分分析 (Git Diff Analysis)**:
   - `git status` および `git diff` を確認し、UI・設定ファイル・アセット・プロトコルに変更が含まれているかを特定する。
2. **マニュアルの該当セクション特定**:
   - [docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) の目次から、変更の影響を受けるセクション（構成図、起動手順、操作シナリオ、UIリファレンス表、FAQ）を特定する。
3. **正確な差分反映**:
   - UIの表記（例: 「Enable Audio」→「Audio ON / Audio OFF」）や、設定項目のキー名、デフォルトポート番号などを実際のコードと完全に一致させる。
   - 変更された音声（例: 英語ATC音声 → 日本語「テスト、テスト。本日は晴天なり、本日は晴天なり。」）を正確に記載する。
4. **README.md の要約同期**:
   - リポジトリの顔である [README.md](file:///Users/shjtmy/gravity/aerovoice/README.md) にも新機能や設定方法が適切に反映されているか確認し、更新する。
5. **整合性セルフチェック**:
   - ドキュメント内のリンク、コマンドライン引数、ポート番号、UIボタン名称がすべて実稼働コードと一致していることを検証する。

---

## 3. ガバナンス品質基準 (Quality Gate)

- **完全同期率 100%**: コード上に存在するUI要素や設定項目が、マニュアルに存在しない（または古い名称のまま放置されている）状態を 0 件にする。
- **再現性**: 初見の開発者やユーザーが、[docs/manual.md](file:///Users/shjtmy/gravity/aerovoice/docs/manual.md) を上から順に実行して1つもエラーや混乱なくデモを完遂できること。
