## 📝 概要
<!-- このPRの目的や変更内容について簡潔に記述してください。 -->


## 🔗 関連する Issue
- #<!-- Issue番号を記載してください -->

## 📦 変更カテゴリ
<!-- 該当するカテゴリにチェックを入れてください。 -->
- [ ] VCS コンソール (`cmd/vcs`, `internal/vcs`)
- [ ] GRS エミュレータ (`cmd/grs`, `internal/grs`)
- [ ] ED-137 プロトコル (`internal/ed137`)
- [ ] SIP / SDP シグナリング (`internal/sip`)
- [ ] RTP / 音声コーデック (`internal/media`)
- [ ] HTMX フロントエンド (`internal/web`, `static/`)
- [ ] パケットキャプチャ / PCAP (`internal/pcap`)
- [ ] 設定 / チャネル管理 (`internal/config`, `internal/channel`)
- [ ] CI / CD / テスト / GitHub Pages (`.github/`, `test/`, `scripts/`)
- [ ] ドキュメント (`README.md`, `docs/`)
- [ ] AI スキル (`.agents/skills/`, `.claude/skills/`)
- [ ] その他

## 🛠️ 変更内容
<!-- どのような変更を加えたかを箇条書きで記載してください。 -->
- 

## 🧪 検証チェックリスト
<!-- 実施した検証にチェックを入れてください。 -->
- [ ] `make fmt` を実行し、Goコードのフォーマットが整っていることを確認した
- [ ] `make lint` が 0 issues で成功することを確認した
- [ ] `make test` が全テスト PASS かつコアプロトコルカバレッジ 80% 以上で成功することを確認した
- [ ] `make aerovoice-test` でプロトコル統合テストが成功することを確認した
- [ ] `make vcs-frontend-e2e` で HTMX フロントエンド検証とレポート生成が成功することを確認した
- [ ] `make build` および `make build-cross` が正常に完了することを確認した
- [ ] `make license-check` でライセンスヘッダーが適切であることを確認した
- [ ] `make self-eval` で REQUIREMENTS.md のスコアを更新・検証した

## 🚨 注意事項・懸念点
<!-- 破壊的変更、互換性の懸念、パフォーマンスへの影響などがあれば記述してください。 -->
- 
