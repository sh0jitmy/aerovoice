# Aerovoice 操作・検証マニュアル (User & Verification Manual)

EUROCAE ED-137C Radio & Telephony 検証用 OSS プロトタイプ「Aerovoice」の操作手順、VCS・GRS 接続アーキテクチャ、および各種検証シナリオを解説するマニュアルです。

---

## 1. システム概要と接続構成図

Aerovoice は、管制官が操作する **VCS（Voice Communication System）** と、滑走路脇などのアンテナ局である **GRS（Ground Radio Station）** の 2 つの独立したプログラムが、標準規格 **EUROCAE ED-137C** に則って UDP 通信を行う構成になっています。

### 1.1 VCS (Voice Communication System) 側システム構成図

管制官が操作するタッチパネル卓・コンソールサーバーの内部構成です。

```mermaid
flowchart TB
    subgraph Browser_VCS["管制官ブラウザ (Web UI)"]
        VCS_UI["管制卓画面 (:8082)<br/>(HTMX + Tailwind CSS)"]
        VCS_Audio["ブラウザ Web Audio API<br/>(マイク集音 & VU/FFT表示)"]
    end

    subgraph VCS_Host["Aerovoice VCS サーバー (:8082)"]
        VCS_Web["Web サーバー (:8082)<br/>(/ui/*, /api/*)"]
        VCS_WS["WebSocket ハブ<br/>(/ws/audio, /ws/events)"]
        VCS_Core["VCS コア制御<br/>・Channel FSM 状態遷移<br/>・Telephony (DA電話) 制御<br/>・Audio Recorder (WAV 8kHz)"]
        VCS_SIP["SIP エージェント (:5060/udp)<br/>・呼制御 (INVITE, BYE)<br/>・死活監視 (OPTIONS Ping)"]
        VCS_RTP["RTP メディアエンジン (:10000~/udp)<br/>・動的ジッタバッファ (10-120ms)<br/>・G.711 PCMA/PCMU エンコード<br/>・ED-137 送話ヘッダー付与"]
    end

    subgraph GRS_Remote["【対向】GRS 無線基地局"]
        Remote_SIP["SIP 待受ポート (:5070/udp)"]
        Remote_RTP["RTP 待受ポート (:20000~/udp)"]
    end

    VCS_UI <-->|HTTP GET/POST| VCS_Web
    VCS_Audio <-->|WebSocket 双方向音声| VCS_WS
    VCS_Web --> VCS_Core
    VCS_WS <--> VCS_Core

    VCS_Core <--> VCS_SIP
    VCS_Core <--> VCS_RTP

    VCS_SIP <===>|"① 呼制御 & 死活監視 (SIP:5060 ⇄ 5070)"| Remote_SIP
    VCS_RTP <===>|"② ED-137 音声通信 (RTP:10000~ ⇄ 20000~)"| Remote_RTP
```

### 1.2 GRS (Ground Radio Station) 側システム構成図

空港滑走路脇に設置された無線中継基地局を模擬するテストベンチの内部構成です。

```mermaid
flowchart TB
    subgraph Browser_GRS["無線局管理ブラウザ (Web UI)"]
        GRS_UI["GRS テストベンチ画面 (:8081)<br/>(HTMX + レスポンシブUI)"]
        GRS_Audio["ブラウザ Web Audio API<br/>(生スピーカー受話 & FFT解析)"]
    end

    subgraph GRS_Host["Aerovoice GRS エミュレータ (:8081)"]
        GRS_Web["Web サーバー (:8081)<br/>(/api/snapshot, /api/control)"]
        GRS_WS["WebSocket 音声配信<br/>(/ws/audio)"]
        GRS_Core["GRS コア制御<br/>・SQU スケルチ制御<br/>・音声ソース切替 (トーン/模擬音声)<br/>・300ms ループバック通話"]
        GRS_Impair["ネットワーク障害注入器<br/>・ジッタ注入 (0〜100ms)<br/>・パケットロス (0〜50%)<br/>・Silent Drop (無応答障害)"]
        GRS_SIP["SIP エージェント (:5070/udp)<br/>・自動着呼応答 (200 OK)<br/>・OPTIONS 死活監視応答"]
        GRS_RTP["RTP メディアエンジン (:20000~/udp)<br/>・ED-137 ヘッダー解析 (PTT, SQU, SQI)<br/>・G.711 デコード / 送出"]
    end

    subgraph VCS_Remote["【対向】VCS 管制卓"]
        Remote_VCS_SIP["SIP 送受信ポート (:5060/udp)"]
        Remote_VCS_RTP["RTP 送受信ポート (:10000~/udp)"]
    end

    Speaker["PC スピーカー音声出力"]

    GRS_UI <-->|HTTP GET/POST| GRS_Web
    GRS_Core -->|リアルタイム音声データ| GRS_WS
    GRS_WS -->|WebSocket| GRS_Audio
    GRS_Audio --> Speaker

    GRS_Web --> GRS_Core
    GRS_Core <--> GRS_Impair
    GRS_Impair <--> GRS_SIP
    GRS_Impair <--> GRS_RTP

    Remote_VCS_SIP <===>|"① 呼制御 & 死活監視 (SIP:5060 ⇄ 5070)"| GRS_SIP
    Remote_VCS_RTP <===>|"② ED-137 音声通信 (RTP:10000~ ⇄ 20000~)"| GRS_RTP
```

### 1.3 ポート割り当て一覧と設定ファイル (YAML / 環境変数)

Aerovoice の全ポート番号・ホストIP・SIP URI は、設定ファイル（`configs/vcs.yaml`、`configs/grs.yaml`）または環境変数から柔軟に変更可能です。

| コンポーネント | デフォルトポート | プロトコル | 設定項目 (`configs/*.yaml`) | 用途 |
| :--- | :--- | :--- | :--- | :--- |
| **VCS Web Console** | `8082` | HTTP / WebSocket | `http_port: 8082` | 管制官用 Web UI 画面、FFT メトリクス配信 |
| **GRS Testbench** | `8081` | HTTP / WebSocket | `http_port: 8081` | 地上無線局エミュレータ用 Web UI 画面 |
| **VCS SIP Agent** | `5060` | UDP | `sip_port: 5060` | ED-137 呼制御シグナリング（クライアント/サーバー） |
| **GRS SIP Agent** | `5070` | UDP | `sip_port: 5070` | ED-137 呼制御シグナリング（対向局サーバー） |
| **VCS RTP** | `10000`〜 | UDP | `rtp_port_start: 10000` | 音声送受信（G.711 A-law/μ-law + ED-137 拡張） |
| **GRS RTP** | `20000`〜 | UDP | `rtp_port_start: 20000` | 音声送受信（G.711 A-law/μ-law + ED-137 拡張） |
| **VCS 録音保持件数** | `100` 件 | - | `recording.max_recordings: 100` | 保持する最大録音件数（上限: 1,000件、超過時は最古ファイルを自動FIFO削除） |
| **VCS 1件最大時間** | `300` 秒 (5分) | - | `recording.max_duration_seconds: 300` | 1通話・1交信あたりの最大録音時間（上限: 1,800秒 / 30分、到達時は正常クリップ保存） |

#### 容量計算とストレージ占有量（PCM 8kHz 16-bit モノラル）
Aerovoice の録音音声は、航空管制標準の PCM 8kHz / 16-bit モノラル形式（16,000 バイト/秒 ≈ 16 KB/秒 ≈ 0.96 MB/分）で WAV 保存されます。

- **1件あたりの最大容量**:
  - デフォルト（5分 / 300秒）: **約 4.8 MB**
  - ハードリミット（30分 / 1,800秒）: **約 28.8 MB**
- **全体ストレージ最大占有量**:
  - デフォルト（100件 × 5分）: **最大 約 480 MB**（安全に 500MB 以内に収まる設計）
  - ハードリミット（1,000件）: **最大 約 4.8 GB〜28.8 GB**

#### 設定ファイルによる変更例
```yaml
# configs/vcs.yaml
node_id: "vcs-01"
http_port: 8082
sip_port: 5060
rtp_port_start: 10000
rtp_host: "127.0.0.1"        # バインド先IPアドレス
vcs_sip_uri: "sip:101@127.0.0.1:5060"
grs_sip_uri: "sip:100@127.0.0.1:5070"

# 録音ガバナンス設定
recording:
  max_recordings: 100        # 最大保存件数（デフォルト: 100件、ハードリミット: 1,000件）
  max_duration_seconds: 300  # 1件あたりの最大録音時間（デフォルト: 300秒=5分、ハードリミット: 1,800秒=30分）
```

#### 環境変数によるオーバーライド
コンテナ運用時などは、設定ファイルを変更せずに環境変数でオーバーライド可能です：
- `AEROVOICE_VCS_HTTP_PORT=9082`
- `AEROVOICE_VCS_SIP_PORT=5062`
- `AEROVOICE_VCS_RTP_HOST=0.0.0.0`
- `AEROVOICE_VCS_REC_MAX_RECORDINGS=200`
- `AEROVOICE_VCS_REC_MAX_DURATION_SECONDS=600`
- `AEROVOICE_GRS_HTTP_PORT=9081`

---

## 2. 2画面インタラクティブ環境の立ち上げ

### 2.1 サーバーの起動
ターミナルから以下のコマンドを実行します。VCS と GRS が自動的にバックグラウンド起動します。
```bash
make demo-vcs
```
*(個別に起動したい場合は `make grs-run` と `make vcs-run` を別ターミナルで実行してください)*

### 2.2 ブラウザの配置
ブラウザ（Google Chrome / Microsoft Edge 推奨）でウィンドウを 2 つ開き、左右に並べます。

```
+------------------------------------+------------------------------------+
|  左画面: VCS コンソール             |  右画面: GRS テストベンチ           |
|  http://127.0.0.1:8082             |  http://127.0.0.1:8081             |
|  (管制官のオペレーション卓)        |  (滑走路脇の無線中継基地局)        |
+------------------------------------+------------------------------------+
```

1. **左画面** で [**http://127.0.0.1:8082**](http://127.0.0.1:8082) を開きます。
2. **右画面** で [**http://127.0.0.1:8081**](http://127.0.0.1:8081) を開きます。
3. **音声の自動有効化と手動制御**:
   - **自動有効化（Auto Activation）**: 「📡 Connect to GRS」ボタンや電話の DA 発信ボタンをクリックした際、ブラウザの Web Audio API が自動的に初期化・起動して **「🔊 Audio ON」** に切り替わります。**一度 PTT を押す必要はなく、接続直後から GRS からの受話音声がスピーカーより即座に聞こえます。**
   - **手動切替**: 画面右上の **「🔊 Audio ON / 🔇 Audio OFF」** ボタンをクリックすることで、いつでも手動で ON/OFF を切り替えられます。
   - **完全無効化（Complete Disable）機能**: 「Audio OFF」に切り替えると、Web Audio ハードウェア（AudioContext）が完全に破棄・クローズされ、マイク入力トラックも物理停止します。マイクの暗騒音ループバックやヒスノイズは一切発生せず、完全な無音状態（ノイズゼロ）になります。

---

## 3. ステップ・バイ・ステップ操作手順

### シナリオ 1: 無線を発信する (日本語音声で PTT 送話テスト)
管制官がマイクの送信スイッチ（PTT: Push-To-Talk）を押して航空機に向けて発信するシナリオです。マイクが接続されていない環境でも、**クリアな日本語テスト音声（「テスト、テスト。本日は晴天なり、本日は晴天なり。」）** で自動送話テストが行えます。

```mermaid
sequenceDiagram
    autonumber
    actor User as あなた (VCS卓)
    participant VCS as VCS (:8082)
    participant GRS as GRS (:8081)
    actor Speaker as PCスピーカー

    User->>VCS: 「Connect to GRS」をクリック
    VCS->>GRS: SIP INVITE (周波数 118.100MHz 接続要求)
    GRS-->>VCS: SIP 200 OK (接続承認)

    User->>VCS: 「🗣️ Send ATC Voice」をクリック (または PTT 長押し)
    VCS->>GRS: ED-137 RTP 送信 (PTT=1, 日本語テスト音声 PCM 8kHz)
    GRS->>GRS: 画面の VU メーターが振れ、ログに PTT 検出が表示
    GRS->>Speaker: PCスピーカーから「テスト、テスト。本日は晴天なり...」がクリアに再生
    Note over VCS,GRS: 音声終了後、自動的に PTT が解除されます
```

1. **左画面（VCS）** の `118.100 MHz TWR Main` カードにある **「📡 Connect to GRS」** をクリックします。
   - カード右上のステータスが `connected`（緑色）に変わります。
2. カード内の **「🗣️ Send ATC Voice (Speech TX)」** ボタンを **クリック** します。
   - ボタンが `🗣️ Speaking (ATC Voice)...` に変わり、**PTT ランプ（赤）** が点灯します。
   - **右画面 (GRS)**: PTT インジケータが点灯し、VU メーターが振れ、右画面の `🔊 Speaker: ON` を有効にしていれば日本語テスト音声がクリアに再生されます。
   - 音声が終了すると自動的に PTT が OFF（解除）されます（約8.5秒間の落ち着いた自然な無線発話完了後に自動復帰）。
3. 通常の **「PUSH TO TALK (PTT)」** ボタン（またはスペースキー長押し）でも、マイク非接続時は日本語テスト音声がストリーミング送信されます。

---

### シナリオ 2: 航空機からの電波を受信する (パイロット日本語音声受話 & FFT スペクトラム)
パイロットが送信した電波を GRS アンテナが受信し、スケルチ（SQU: スピーカーの消音を解除する信号）を開いて管制官に音声を届けるシナリオです。**落ち着いた自然な速度の日本語テスト音声（「テスト、テスト。本日は晴天なり、本日は晴天なり。」）** が、パケット重なりや音割れのない最新スケジューリングキューによりクリアに受話されます。

1. **右画面（GRS）** の送信音声設定で **「🧑‍✈️ パイロット音声 (テスト、テスト。本日は晴天なり、本日は晴天なり。)」** が選択されていることを確認します（デフォルトで選択済み）。
2. 右画面の中央にある **「Squelch (SQU) Control」** のボタンをクリックして **ON (Transmitting)** にします。
3. **左画面（VCS）** を確認します：
   - チャンネルカードの **「SQU ランプ（緑）」** が点灯し、受信信号強度（SQI: 100）が表示されます。
   - スピーカーからクリアな日本語テスト音声（「テスト、テスト。本日は晴天なり、本日は晴天なり。」）が流れます。
   - 画面右側の **「Real-Time Audio Spectrum (Canvas)」** に、人の声特有の **豊かな声帯フォルマント（母音・子音の複数ピーク）** が 60FPS でリアルタイム描画されます。
   - チャンネルカード内の **「Jitter Buffer」スライダー** を動かすことで、音声のバッファリング深さを 10ms 〜 120ms でリアルタイムに調整できます。
4. 右画面（GRS）の SQUELCH を **OFF** に戻すと、スケルチが閉じ、受話音声が止まります。

#### 🔁 (応用試験) GRS Loopback Echo による無線エコー通話試験
1. **右画面（GRS）** の Sound Source で **「🔁 Loopback Echo (VCS Audio)」** を選択し、**Squelch を ON** にしておきます。
2. **左画面（VCS）** で **「🗣️ Send ATC Voice」**（または PTT）を押して話しかけます。
3. GRS 側の内部 FIFO キューが VCS の音声を蓄積し、途中で止まることなく **話した内容がそっくりそのまま VCS 側のスピーカーに折り返しエコーバック** されます。
4. VCS 側の送話が終了した後も、蓄積されたエコーが最後まで綺麗に流れきり、完了後に自動で無音に戻ります。

---

### シナリオ 3: 直通電話をかける (Telephony DA & 日本語音声による通話品質試験)
管制塔（TWR）と進入管制所（APP）の間でワンタッチで内線通話を行うダイレクトアクセス（DA）機能のテストです。

1. **左画面（VCS）** の上部ナビゲーションで **「Telephony (DA)」** タブをクリックします。
2. **「🗣️ Human Speech Test Call」** ボタンをクリックします。
   - 対向 GRS への SIP 電話発信が自動で行われ、通話中（connected / mode: speech）になります。
   - 通話相手に向けて、日本語テスト音声（「テスト、テスト。本日は晴天なり、本日は晴天なり。」）が送出され、遅延やジッタ測定値がリアルタイムに更新されます。
3. **「Hangup Call (終話)」** をクリックすると通話が終了します。
4. 従来の規格基準トーンである **「1kHz Tone Test Call」** や **「300ms Echo Loopback Test」** も引き続き選択可能です。

---

### シナリオ 4: 録音の再生・ダウンロード (Recordings タブ)
ED-137 Volume 4（録音規格）に準拠し、すべての無線交信・電話通話はバックグラウンドで自動的に WAV 音声ファイルとして保管されています。

1. **左画面（VCS）** の上部ナビゲーションで **「Recordings」** タブをクリックします。
2. 先ほどシナリオ 1〜3 で行った無線交信や電話通話の履歴がテーブル形式で一覧表示されています。
3. テーブル内の **再生プレイヤー（`<audio>`）** の再生ボタンを押すと、交信音声をブラウザ上で即座に確認できます。
4. **「Download」** ボタンをクリックすると、WAV ファイルをローカル PC にダウンロードできます。

#### 録音リソースガバナンスと容量保護機構
Aerovoice のレコーダーは、長時間の PTT 誤操作や大量通話によるディスク容量枯渇を防止するため、以下の安全機構が自動稼働しています：
- **1件あたりの最大録音時間制限**:
  - デフォルト **5分（300秒 / 約4.8MB）** で上限ガード。万一 PTT が押しっぱなしになっても、5分経過時点でサンプル蓄積を安全にキャップし、正常な WAV ファイルとして保存を完了します（設定上限: 30分 / 1,800秒）。
- **最大保持件数の自動ローテーション (FIFO)**:
  - デフォルト **100件（最大約480MB）** を維持。101件目の新規録音が完了した瞬間に、ディスク上で最も古い WAV ファイルが物理削除（自動ローテーション）されます（設定上限: 1,000件）。
- **起動時ストレージ健全化**:
  - サーバー再起動時にもディスクスキャンを行い、設定された最大件数（100件）を超える古い WAV ファイルが存在する場合は即座に自動パージされ、ディスク枯渇を未然に防ぎます。

---

### シナリオ 5: 基地局の死活監視 (Supervision タブ)
ED-137 Volume 5 規格に準拠した SIP 死活監視をリアルタイムに確認できます。

1. **左画面（VCS）** の上部ナビゲーションで **「Supervision」** タブをクリックします。
2. 各 GRS 基地局に対してバックグラウンドで 5 秒おきに `SIP OPTIONS` Keepalive が送出されており、**RTT（応答往復時間：0.5ms など）** および死活状態（Online/Offline）が監視されています。
3. *(応用試験)* **右画面（GRS）** で **「Silent Drop (Keepalive無応答障害)」** のスイッチを ON にすると、VCS 側の Supervision パネルでノードが赤色（Offline）に変わり、障害検知アラートの動作を確認できます。

---

### シナリオ 6: 事後 VoIP パケット診断 (PCAP Analyzer タブ)
Wireshark 等で取得した `.pcap` / `.pcapng` ファイルから、ED-137 固有の制御情報と音声を復元・解析します。

1. **左画面（VCS）** の上部ナビゲーションで **「PCAP Analyzer」** タブをクリックします。
2. ドラッグ＆ドロップエリアに `.pcap` または `.pcapng` ファイルをドロップします（またはクリックして選択）。
3. Go 言語の内蔵パーサーがパケットを解析し、以下の診断結果が瞬時に出力されます：
   - パケット総数、RTP パケット数、平均ジッタ、パケットロス率
   - ED-137 拡張ヘッダー（PTT ON/OFF, SQU, SQI, PTT-ID）の時系列タイムライン
   - 音声ペイロードから復元された WAV ファイルのインライン再生・ダウンロードリンク

---

### シナリオ 7: プロトコル通信ログの確認 (Comm Logs タブ & GRS Event Log)
VCS と GRS の双方で、送受信される SIP シグナリング、ED-137 RTP パケット、死活監視 OPTIONS などの通信ログをリアルタイムに確認できます。

1. **左画面（VCS）**:
   - 上部ナビゲーションの **「📜 Comm Logs」** タブをクリックします。
   - ターミナル画面上に、`[SIP]`, `[ED-137]`, `[RTP]`, `[SYS]` ごとに色分けされたプロトコル送受信ログがリアルタイムに流れます。
   - 上部の **フィルターボタン（All / SIP / ED-137 / RTP / SYS）** で表示を絞り込んだり、**「Clear Logs」** で消去できます。
   - 他のタブ（Radio や Telephony）を開いているときでも、画面最下部の **「📡 LIVE COMM LOG」ティッカーバー** に最新の通信イベントが1行でリアルタイム更新されます（クリックすると Comm Logs タブへ移動）。
2. **右画面（GRS）**:
   - 画面下部の **「📋 GRS Protocol & Event Log」** コンソールに、受信した SIP INVITE/OPTIONS、送出した SQU/RTP パケットなどのログがバッジ付きでリアルタイム表示されます。

---

## 4. 画面 UI リファレンス

### 4.1 VCS コンソール (`:8082`)

| 画面要素 | 種類 | 説明 |
| :--- | :--- | :--- |
| **Audio ON / Audio OFF** | ボタン (右上) | ブラウザの Web Audio API を有効化 / 完全解放（ハードウェアクローズ & マイク停止）するトグル。 |
| **Radio Console タブ** | タブ 1 | 無線周波数チャンネル（TWR 118.100MHz, APP 120.500MHz 等）の制御画面。 |
| **Telephony タブ** | タブ 2 | ダイレクトアクセス（DA）短縮電話、1kHz 試験トーン呼、300ms エコー呼、日本語音声呼。 |
| **Recordings タブ** | タブ 3 | 録音された WAV ファイルの一覧表示、ブラウザ再生、ダウンロード。 |
| **Supervision タブ** | タブ 4 | 各 GRS 局の SIP OPTIONS 死活状態および RTT 往復遅延の常時監視。 |
| **PCAP Analyzer タブ**| タブ 5 | PCAP ファイルのドラッグ＆ドロップ事後解析、タイムライン表示、音声復元。 |
| **Comm Logs タブ** | タブ 6 | ED-137 / SIP / RTP のリアルタイム送受信ログコンソール（フィルタ・クリア対応）。 |
| **Live Comm Ticker** | バー (最下部) | 全タブ共通で画面下部に最新通信イベントをリアルタイム表示するティッカー。 |
| **Connect to GRS** | ボタン | 対象周波数の対向 GRS と SIP/SDP 接続を確立します。 |
| **PUSH TO TALK** | ボタン | 長押しで無線送信（PTT=1）を行います（スペースキー長押しでも可）。 |
| **PTT ランプ (赤)** | インジケータ | 自卓が送信中（PTT ON）であることを示します。 |
| **SQU ランプ (緑)** | インジケータ | 対向基地局が航空機電波を受信中（Squelch 開）であることを示します。 |
| **Jitter Buffer** | スライダー | 受信パケットのジッタ吸収バッファ遅延（10〜120ms）を動的に設定します。 |
| **Audio Spectrum** | Canvas | 受信または送信音声の 0〜4kHz リアルタイム周波数スペクトラムを描画します。 |

### 4.2 GRS テストベンチ (`:8081`)

| 画面要素 | 種類 | 説明 |
| :--- | :--- | :--- |
| **Speaker: ON / OFF** | ボタン (右上) | 受信音のスピーカー再生を有効化 / 完全解放（close）するトグル。 |
| **PTT Indicator** | 表示 | VCS から PTT 送信が届いているかを表示します。 |
| **VU Meter** | メーター | VCS から受信した音声レベルをリアルタイムに表示します。 |
| **Sound Source** | ラジオ選択 | 送出音声ソース（🧑‍✈️ パイロット音声 [日本語・自然速度] / 📞 電話音声 / 1kHz Tone / Beep / 🔁 Loopback Echo）を選択します。<br>※ Loopback Echo は VCS からの受信音声を FIFO キューで保持し、途切れず滑らかに送り返します。 |
| **Squelch (SQU) Control** | ボタン | 選択された音声ソースを VCS に向けて送出（SQU ON/OFF）します。 |
| **Jitter Injection** | スライダー | VCS へ送出する RTP に人工的なジッタ（0〜100ms）を注入します。 |
| **Packet Loss** | スライダー | 人工的なパケット破棄（0〜50%）を発生させ、耐障害性をテストします。 |
| **Silent Drop** | スイッチ | SIP OPTIONS への応答を停止し、Supervision の回線断検知をテストします。 |

---

## 5. トラブルシューティング (FAQ)

### Q1. ブラウザにアクセスしても 404 Not Found になる
- **原因**: 別のコンテナ（例: `musubi-server` など）がポート `8080` を専有している可能性があります。
- **解決策**: Aerovoice の VCS コンソールは **ポート `8082`** で起動しています。[**http://127.0.0.1:8082**](http://127.0.0.1:8082) にアクセスしてください。

### Q2. スピーカーから音が出ない / スペクトラムが動かない
- **原因**: 最新のブラウザ（Chrome 等）の自動再生ポリシー、または Audio が OFF になっているためです。
- **解決策**: VCS 画面右上の **「🔊 Audio OFF」** ボタンをクリックして **「🔊 Audio ON」** に切り替えてください。

### Q3. PTT を押しても GRS 側に届かない
- **解決策**: チャンネルカードの右上が `connected`（緑色）になっているか確認してください。`disconnected` の場合は **「📡 Connect to GRS」** をクリックして SIP セッションを確立してください。

### Q4. Audio OFF にしたのにノイズが聞こえることはないか？
- **解説**: Aerovoice では、Audio OFF 時に `audioCtx.close()` を実行してブラウザのオーディオハードウェアを完全に解放し、マイク入力ストリームも物理停止（`track.stop()`）します。また、マイク音声がスピーカーにループバックしないゼロゲイン設計となっているため、OFF 時は電気的・物理的に 100% ノイズゼロ（完全 disable）になります。

### Q5. 終了・再起動したい
- ターミナルで `Ctrl + C` を押すと、バックグラウンドの VCS・GRS プロセスが自動クリーンアップされて停止します。
- 再度立ち上げる際は `make demo-vcs` を実行してください。

### Q6. 録音の最大時間と最大件数はいくつですか？ストレージが枯渇する心配はありませんか？
- **解説**:
  - **最大録音時間**: デフォルトで **1件あたり最大5分（300秒）** に制限されています。PTT を長時間押しっぱなしにした場合でも 5 分で安全に頭打ちとなり、正常な WAV ファイルとして保存されます。
  - **最大保存件数**: デフォルトで **最大 100 件** です。上限に達すると、最も古い録音ファイルが自動的に削除（FIFO 自動ローテーション）されるため、ディスク容量が無制限に膨らむことはありません。
  - **最大ストレージ占有量**: PCM 8kHz 16-bit モノラル（約16KB/秒）基準で、1件5分＝約4.8MB。100件すべてが5分間フル録音だった場合でも **最大約480MB** であり、一般的な環境で500MB以内に安全に収まります。
  - **変更方法**: `configs/vcs.yaml` の `recording.max_recordings`（上限1,000件）や `recording.max_duration_seconds`（上限1,800秒=30分）、または環境変数 `AEROVOICE_VCS_REC_MAX_RECORDINGS`, `AEROVOICE_VCS_REC_MAX_DURATION_SECONDS` で任意に調整可能です。

