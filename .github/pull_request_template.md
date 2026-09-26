## 📝 概要 / Summary
<!--
[EN] Please provide a brief summary of the purpose and changes in this PR.
[JA] このPRの目的や変更内容について簡潔に記述してください。
-->


## 🔗 関連する Issue / Related Issues
- #<!-- Issue number / Issue番号 -->

## 📦 変更カテゴリ / Change Category
<!--
[EN] Please check all categories that apply.
[JA] 該当するカテゴリにチェックを入れてください。
-->
- [ ] VCS コンソール / VCS Console (`cmd/vcs`, `internal/vcs`)
- [ ] GRS エミュレータ / GRS Radio Emulator (`cmd/grs`, `internal/grs`)
- [ ] ED-137 プロトコル / ED-137 Radio Header & Signaling (`internal/ed137`)
- [ ] SIP / SDP シグナリング / SIP & SDP Session (`internal/sip`)
- [ ] RTP / 音声コーデック / Audio Codec & Jitter Buffer (`internal/media`)
- [ ] HTMX フロントエンド / HTMX Web UI (`internal/web`, `static/`)
- [ ] パケットキャプチャ / PCAP Analyzer (`internal/pcap`)
- [ ] 設定 / チャネル管理 / Configuration & Channels (`internal/config`, `internal/channel`)
- [ ] CI / CD / テスト / CI, E2E & GitHub Pages (`.github/`, `test/`, `scripts/`)
- [ ] ドキュメント / Documentation (`README.md`, `docs/`)
- [ ] AI スキル / AI Skills (`.agents/skills/`, `.claude/skills/`)
- [ ] その他 / Other

## 🛠️ 変更内容 / Changes
<!--
[EN] List the specific changes introduced by this PR.
[JA] どのような変更を加えたかを箇条書きで記載してください。
-->
- 

## 🧪 検証チェックリスト / Verification Checklist
<!--
[EN] Please check the verification steps performed before submitting.
[JA] 実施した検証にチェックを入れてください。
-->
- [ ] `make fmt` を実行し、Goコードのフォーマットが整っていることを確認した / Verified code formatting
- [ ] `make lint` が 0 issues で成功することを確認した / Passed golangci-lint with 0 issues
- [ ] `make test` が全テスト PASS かつコアプロトコルカバレッジ 80% 以上で成功することを確認した / Unit & coverage tests passed
- [ ] `make aerovoice-test` でプロトコル統合テストが成功することを確認した / ED-137 protocol test suite passed
- [ ] `make vcs-frontend-e2e` で HTMX フロントエンド検証とレポート生成が成功することを確認した / Frontend E2E tests passed
- [ ] `make build` および `make build-cross` が正常に完了することを確認した / Build & cross-compile succeeded
- [ ] `make license-check` で Apache-2.0 ライセンスヘッダーが適切であることを確認した / Verified license headers
- [ ] `make self-eval` で REQUIREMENTS.md のスコアを更新・検証した / Updated REQUIREMENTS.md compliance

## 🚨 注意事項・懸念点 / Notes & Concerns
<!--
[EN] Mention breaking changes, performance impact, or operational notes if any.
[JA] 破壊的変更、互換性の懸念、パフォーマンスへの影響などがあれば記述してください。
-->
- 
