// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

# ADR 0004: 多層 E2E テストフレームワーク (Multi-Tier E2E Testing Framework)

## ステータス
承認済み (Accepted)

## コンテキスト
航空管制音声通信（ED-137B/C）の実装品質を保証するためには、単体テスト（Unit Test）だけでなく、SIP/SDP 呼制御、RTP / ED-137 拡張ヘッダー音声伝送、ネットワーク障害（ジッタ・パケットロス・Silent Drop）、HTMX による管制卓フロントエンドUIのレンダリング、および事後PCAP解析までを網羅する包括的な検証が必要です。
また、外部の専用VoIP測定器やハードウェア、重厚な Docker コンテナに依存せず、ローカル環境および GitHub Actions CI 上で高速かつ確実に完結するテスト階層が求められます。

## 意思決定

以下の 3 層からなる「多層 E2E テストフレームワーク」を構築し、Pure Go による決定論的かつ高速な検証を実現します。

1. **Layer 1: 単体テスト & コアプロトコルカバレッジ (`make test`)**:
   - `t.Parallel()` による高速並行実行。
   - 動的 UDP / TCP ポート割り当てによる並行テスト時のポート競合完全防止。
   - `scripts/check_coverage.sh` による Aerovoice コアプロトコル（ed137, channel, sip, pcap, media/codec, config）のカバレッジ検証（80% 以上必須、現在 87.37%）。
   - `-race` フラグによる低レイテンシ並行処理のデータ競合検出。
2. **Layer 2: ED-137 Radio & Telephony プロトコル検証スイート (`make aerovoice-test`)**:
   - `internal/ed137`, `internal/sip`, `internal/media`, `internal/channel`, `internal/vcs`, `internal/grs`, `test/e2e`。
   - PTT Type（Normal / Priority / Emergency）、Downlink SQUELCH、ジッタバッファ（10ms〜120ms）、Silent Drop 障害シミュレーション、Direct Access 全二重電話通話、Loopback Echo の完全プロトコル疎通テスト。
3. **Layer 3: Standalone VCS HTMX フロントエンド E2E (`make vcs-frontend-e2e`)**:
   - `scripts/vcs_frontend_e2e.sh`
   - Headless Chrome を用いた自動スナップショット撮影（`docs/images/vcs_htmx_dashboard.png`）。
   - HTMX による Radio チャンネル制御、Telephony DA 発着信、Audio Recordings 再生テーブル、Node Supervision 死活監視、Comm Logs リアルタイムログターミナルの一括検証。
   - スタンドアロン HTML レポート生成（`test_reports/vcs_frontend_e2e_report.html`）。

## 帰結

- **利点**:
  - Docker や外部DBを一切起動することなく、ローカル・CI双方で数秒〜数十秒で全レイヤーの検証が完了。
  - プロトコル層（SIP/RTP）からUI層（HTMX/ブラウザ表示）までを一気通貫で自動検証可能。
  - HTML ビジュアルレポートおよび Chrome スナップショットにより、変更時のリグレッションを視覚的に即座に検知。
