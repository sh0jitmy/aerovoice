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

package vcs

const IndexHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Aerovoice - ED-137C VCS Console</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/htmx.min.js"></script>
</head>
<body>
    <header class="app-header">
        <div class="brand">
            <div class="radar-sweep-icon"></div>
            <div>
                <h1>AEROVOICE <span class="badge-vcs">VCS</span></h1>
                <p class="subtitle">EUROCAE ED-137C Radio & Telephony Verification Console</p>
            </div>
        </div>
        <div class="header-status">
            <div id="ws-indicator" class="ws-status disconnected">
                <span class="dot"></span> <span id="ws-label">WS Disconnected</span>
            </div>
            <div class="audio-toggle" style="display:flex; gap:0.5rem;">
                <button id="btn-manual" class="btn-secondary" onclick="toggleManualModal()" style="font-size:0.85rem; padding:0.4rem 0.8rem;">
                    <span class="icon">&#128214;</span> 操作マニュアル
                </button>
                <button id="btn-audio-init" class="btn-neon-small" onclick="toggleWebAudio()">
                    <span class="icon">&#128266;</span> Enable Audio
                </button>
            </div>
        </div>
    </header>

    <nav class="tab-nav">
        <button class="tab-btn active" onclick="showTab('radio')">
            <span class="tab-icon">&#128225;</span> Radio Console
        </button>
        <button class="tab-btn" onclick="showTab('telephony')">
            <span class="tab-icon">&#128222;</span> Telephony (DA)
        </button>
        <button class="tab-btn" onclick="showTab('recordings')" hx-get="/ui/components/recordings-table" hx-target="#recordings-container">
            <span class="tab-icon">&#127911;</span> Recordings
        </button>
        <button class="tab-btn" onclick="showTab('supervision')" hx-get="/ui/components/supervision-panel" hx-target="#supervision-container">
            <span class="tab-icon">&#128737;</span> Supervision
        </button>
        <button class="tab-btn" onclick="showTab('pcap')">
            <span class="tab-icon">&#128269;</span> PCAP Analyzer
        </button>
        <button class="tab-btn" onclick="showTab('logs')">
            <span class="tab-icon">&#128220;</span> Comm Logs <span id="log-unread-badge" class="badge-pill" style="display:none;">0</span>
        </button>
    </nav>

    <main class="main-content">
        <!-- TAB 1: RADIO CONSOLE -->
        <section id="tab-radio" class="tab-content active">
            <div class="console-layout">
                <div class="channel-grid" id="channels-container" hx-get="/ui/components/radio-channels" hx-trigger="load, every 2s">
                    <!-- Dynamically rendered channels -->
                    <div class="loading-spinner">Loading Radio Channels...</div>
                </div>

                <div class="live-analysis-panel">
                    <div class="card glass">
                        <div class="card-header">
                            <h3>Real-Time Audio Spectrum & VU</h3>
                            <span class="tag-mono" id="active-audio-channel">Idle</span>
                        </div>
                        <div class="card-body">
                            <div class="visualizer-container">
                                <canvas id="spectrum-canvas" width="600" height="180"></canvas>
                                <div class="vu-meter-container">
                                    <div class="vu-bar" id="vu-bar"></div>
                                </div>
                            </div>
                            <div class="spectrum-labels">
                                <span>0 Hz</span>
                                <span>1 kHz (Test Tone)</span>
                                <span>2 kHz</span>
                                <span>3 kHz (Voice)</span>
                                <span>4 kHz (Nyquist)</span>
                            </div>
                            <div class="ptt-guide">
                                <p><strong>PTT Shortcuts:</strong> Press and hold <code>SPACE</code> key or click & hold the <strong>PTT Button</strong> to transmit.</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </section>

        <!-- TAB 2: TELEPHONY (DIRECT ACCESS) -->
        <section id="tab-telephony" class="tab-content">
            <div class="telephony-layout" id="telephony-container" hx-get="/ui/components/telephony-panel" hx-trigger="load, every 2s">
                <div class="loading-spinner">Loading Telephony Subsystem...</div>
            </div>
        </section>

        <!-- TAB 3: RECORDINGS -->
        <section id="tab-recordings" class="tab-content">
            <div class="card glass">
                <div class="card-header">
                    <h3>WAV Audio Logs (ED-137 Volume 4 Compliant)</h3>
                    <button class="btn-secondary" hx-get="/ui/components/recordings-table" hx-target="#recordings-container">
                        &#8635; Refresh
                    </button>
                </div>
                <div class="card-body" id="recordings-container" hx-get="/ui/components/recordings-table" hx-trigger="load, every 2s">
                    <div class="loading-spinner">Loading Recordings...</div>
                </div>
            </div>
        </section>

        <!-- TAB 4: SUPERVISION -->
        <section id="tab-supervision" class="tab-content">
            <div class="card glass">
                <div class="card-header">
                    <h3>GRS Node Supervision & SIP Health (ED-137 Volume 5)</h3>
                    <button class="btn-secondary" hx-get="/ui/components/supervision-panel" hx-target="#supervision-container">
                        &#8635; Refresh
                    </button>
                </div>
                <div class="card-body" id="supervision-container">
                    <div class="loading-spinner">Loading Supervision Status...</div>
                </div>
            </div>
        </section>

        <!-- TAB 5: PCAP ANALYZER -->
        <section id="tab-pcap" class="tab-content">
            <div class="pcap-layout">
                <div class="card glass">
                    <div class="card-header">
                        <h3>Post-Mortem VoIP PCAP Diagnostic Engine</h3>
                    </div>
                    <div class="card-body">
                        <div class="upload-zone" id="upload-zone" onclick="document.getElementById('pcap-file-input').click()">
                            <input type="file" id="pcap-file-input" accept=".pcap,.pcapng" style="display:none;" onchange="handlePCAPUpload(this)">
                            <div class="upload-icon">&#128190;</div>
                            <h4>Click or Drag & Drop PCAP File Here</h4>
                            <p class="muted">Supports pure Go Wireshark PCAP parsing, ED-137 header extension extraction, jitter & loss calculations, and WAV extraction.</p>
                        </div>

                        <div id="pcap-results" style="display:none; margin-top: 1.5rem;">
                            <!-- Injected by JS -->
                        </div>
                    </div>
                </div>
            </div>
        </section>

        <!-- TAB 6: COMM LOGS -->
        <section id="tab-logs" class="tab-content">
            <div class="card glass">
                <div class="card-header" style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:0.5rem;">
                    <div>
                        <h3>ED-137 Protocol & Signaling Live Logs</h3>
                        <p class="muted" style="font-size:0.75rem;">Real-time SIP dialogs, ED-137 RTP header extensions, PTT/SQU events, and Supervision metrics.</p>
                    </div>
                    <div style="display:flex; gap:0.5rem; align-items:center;">
                        <div class="filter-group">
                            <button class="btn-filter active" onclick="filterLogs('ALL')">All</button>
                            <button class="btn-filter" onclick="filterLogs('SIP')">SIP</button>
                            <button class="btn-filter" onclick="filterLogs('ED-137')">ED-137</button>
                            <button class="btn-filter" onclick="filterLogs('RTP')">RTP</button>
                            <button class="btn-filter" onclick="filterLogs('SYS')">SYS</button>
                        </div>
                        <label style="font-size:0.75rem; display:flex; align-items:center; gap:4px; cursor:pointer;">
                            <input type="checkbox" id="chk-auto-scroll" checked> Auto-scroll
                        </label>
                        <button class="btn-secondary" onclick="clearLogs()" style="padding:0.25rem 0.6rem; font-size:0.75rem;">
                            Clear Logs
                        </button>
                    </div>
                </div>
                <div class="card-body" style="padding: 0;">
                    <div id="vcs-log-terminal" class="log-terminal">
                        <!-- Streamed live via WebSocket / refreshed via API -->
                        <div class="muted" style="padding: 1rem;">Connecting to real-time communication log stream...</div>
                    </div>
                </div>
            </div>
        </section>
    </main>

    <!-- LIVE COMM LOG TICKER (Global footer bar) -->
    <div class="live-ticker-bar" onclick="showTab('logs')">
        <span class="ticker-label">&#128220; LIVE COMM LOG:</span>
        <span id="ticker-text" class="ticker-text">Waiting for protocol events...</span>
        <span class="ticker-hint">Click to view full logs &rarr;</span>
    </div>

    <!-- MANUAL & HELP MODAL -->
    <div id="modal-manual" class="modal-overlay" style="display:none;" onclick="if(event.target === this) toggleManualModal()">
        <div class="modal-content glass">
            <div class="modal-header">
                <h2>&#128214; Aerovoice 操作・検証マニュアル (Quick Guide)</h2>
                <button class="btn-close" onclick="toggleManualModal()">&times;</button>
            </div>
            <div class="modal-body">
                <h3>1. GRS と VCS の接続構成図 (ED-137C)</h3>
                <div class="code-block" style="background:#0d1117; padding:1rem; border-radius:6px; font-family:monospace; font-size:0.8rem; line-height:1.4; overflow-x:auto; margin-bottom:1rem;">
[ ブラウザ画面 (Chrome / Edge) ]
  左: VCS コンソール (:8082) &lt;-- HTTP / WebSocket --&gt; VCS サーバー (:8082)
  右: GRS テストベンチ (:8081) &lt;-- HTTP ------------&gt; GRS サーバー (:8081)

[ サーバー間通信 (EUROCAE ED-137C 規格準拠) ]
  VCS (:5060/udp) &lt;==== SIP INVITE / 200 OK (呼制御) ====&gt; GRS (:5070/udp)
  VCS (:5060/udp) &lt;==== SIP OPTIONS (5秒死活監視 Ping) ===&gt; GRS (:5070/udp)
  VCS (:10000/udp) &lt;=== RTP G.711 + ED-137 拡張ヘッダ ===&gt; GRS (:20000/udp)
                        [ PTT=1, SQU, SQI, PTT-ID ]
                </div>

                <h3>2. 2画面インタラクティブ検証の手順 (5分)</h3>
                <ol style="padding-left:1.5rem; margin-bottom:1.5rem; line-height:1.8;">
                    <li><strong>画面の準備:</strong> ブラウザを左右に2つ並べ、左で <code>http://127.0.0.1:8082</code> (VCS)、右で <code>http://127.0.0.1:8081</code> (GRS) を開きます。画面右上の <strong>「🔊 Enable Audio」</strong> をクリックして音声を有効化（ON）します（再度クリックすることでいつでも OFF に切り替えられます）。</li>
                    <li><strong>無線発信 (PTT送信):</strong> 左画面の <code>TWR Main</code> で「<strong>Connect to GRS</strong>」をクリック &rarr; 「<strong>PUSH TO TALK</strong>」ボタンまたは <strong>スペースキー長押し</strong> で発信します。右画面（GRS）の VU メーターが振れ、PCスピーカーから受話音が流れます。</li>
                    <li><strong>無線受信 (SQU受信 & FFT):</strong> 右画面（GRS）の「<strong>Squelch Downlink</strong>」スイッチを ON にします &rarr; 左画面（VCS）の <strong>SQU ランプが点灯</strong> し、スピーカーから受信音が鳴り、画面右の <strong>FFT スペクトラム（Canvas）</strong> に 400Hz の綺麗なピークが描画されます。</li>
                    <li><strong>直通電話 (Telephony DA):</strong> 上部「<strong>Telephony (DA)</strong>」タブを開き、「<strong>1kHz Tone Test Call</strong>」または「<strong>300ms Echo Loopback Test</strong>」をクリックします &rarr; 全二重通話とジッタ測定が実行されます。</li>
                    <li><strong>録音の確認 (Recordings):</strong> 上部「<strong>Recordings</strong>」タブを開くと、すべての交信・通話の WAV ファイルが一覧化されており、ブラウザ上でそのまま試聴・ダウンロードできます。</li>
                    <li><strong>死活監視 (Supervision):</strong> 「<strong>Supervision</strong>」タブでは各局の SIP 死活状態とミリ秒単位の RTT（往復遅延）が常時モニタリングされます。</li>
                    <li><strong>PCAP 事後診断 (PCAP Analyzer):</strong> 「<strong>PCAP Analyzer</strong>」タブに <code>.pcap</code> ファイルをドラッグ＆ドロップすると、ED-137 拡張ヘッダーのタイムライン解析と音声復元が行われます。</li>
                </ol>

                <div style="text-align:right;">
                    <button class="btn-neon-small" onclick="toggleManualModal()">閉じる (Close)</button>
                </div>
            </div>
        </div>
    </div>

    <footer class="app-footer">
        <p>AEROVOICE VCS Prototype &bull; ED-137C Radio & Telephony Verification &bull; Pure Go & Web Audio API &bull; <a href="/docs/manual" target="_blank" style="color:var(--color-cyan);">詳細マニュアル (docs/manual.md)</a></p>
    </footer>

    <script src="/static/app.js"></script>
</body>
</html>
`

const StyleCSS = `/* Aerovoice Avionics & Dark Design System */
:root {
    --bg-base: #0a0e17;
    --bg-surface: #111827;
    --bg-card: rgba(17, 24, 39, 0.75);
    --border-subtle: rgba(255, 255, 255, 0.08);
    --border-focus: #00f0ff;
    --text-main: #f3f4f6;
    --text-muted: #9ca3af;
    --color-cyan: #00f0ff;
    --color-green: #00ff88;
    --color-amber: #ffaa00;
    --color-red: #ff3366;
    --font-sans: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
    --font-mono: 'JetBrains Mono', monospace;
}

* { box-sizing: border-box; margin: 0; padding: 0; }
body {
    background-color: var(--bg-base);
    color: var(--text-main);
    font-family: var(--font-sans);
    line-height: 1.5;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
}

/* Header */
.app-header {
    background: rgba(10, 14, 23, 0.95);
    border-bottom: 1px solid var(--border-subtle);
    padding: 0.85rem 1.5rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    backdrop-filter: blur(10px);
}
.brand { display: flex; align-items: center; gap: 0.75rem; }
.brand h1 { font-size: 1.35rem; font-weight: 700; letter-spacing: 0.05em; color: #fff; }
.badge-vcs {
    background: linear-gradient(135deg, #00f0ff, #0077ff);
    color: #000;
    font-size: 0.7rem;
    font-weight: 800;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    vertical-align: middle;
}
.subtitle { font-size: 0.75rem; color: var(--text-muted); }

.radar-sweep-icon {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: 1px solid var(--color-cyan);
    position: relative;
    box-shadow: 0 0 10px rgba(0, 240, 255, 0.3);
}
.radar-sweep-icon::after {
    content: '';
    position: absolute;
    top: 50%; left: 50%;
    width: 14px; height: 1px;
    background: var(--color-cyan);
    transform-origin: 0 0;
    animation: radar-sweep 2s linear infinite;
}
@keyframes radar-sweep { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

.header-status { display: flex; align-items: center; gap: 1rem; }
.ws-status {
    font-size: 0.75rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.6rem;
    border-radius: 20px;
    background: rgba(255, 255, 255, 0.05);
}
.ws-status .dot { width: 8px; height: 8px; border-radius: 50%; }
.ws-status.connected .dot { background: var(--color-green); box-shadow: 0 0 8px var(--color-green); }
.ws-status.disconnected .dot { background: var(--color-red); }

/* Tabs */
.tab-nav {
    display: flex;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-subtle);
    padding: 0 1.5rem;
    gap: 0.5rem;
}
.tab-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 0.85rem;
    font-weight: 500;
    padding: 0.75rem 1rem;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    transition: all 0.2s;
}
.tab-btn:hover { color: #fff; }
.tab-btn.active {
    color: var(--color-cyan);
    border-bottom-color: var(--color-cyan);
}

/* Main */
.main-content { flex: 1; padding: 1.5rem; }
.tab-content { display: none; }
.tab-content.active { display: block; }

/* Cards & Glassmorphism */
.card {
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    overflow: hidden;
    backdrop-filter: blur(12px);
}
.card-header {
    padding: 0.75rem 1.25rem;
    border-bottom: 1px solid var(--border-subtle);
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(255, 255, 255, 0.02);
}
.card-header h3 { font-size: 0.95rem; font-weight: 600; letter-spacing: 0.02em; }
.card-body { padding: 1.25rem; }

/* Radio Grid */
.console-layout { display: grid; grid-template-columns: 1fr 420px; gap: 1.25rem; }
@media (max-width: 1024px) { .console-layout { grid-template-columns: 1fr; } }

.channel-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 1.25rem; }

.channel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    position: relative;
    transition: transform 0.2s, border-color 0.2s;
}
.channel-card:hover { border-color: rgba(0, 240, 255, 0.3); }
.channel-card.transmitting { border-color: var(--color-red); box-shadow: 0 0 15px rgba(255, 51, 102, 0.2); }
.channel-card.receiving { border-color: var(--color-green); box-shadow: 0 0 15px rgba(0, 255, 136, 0.2); }

.ch-header { display: flex; justify-content: space-between; align-items: flex-start; }
.ch-freq { font-family: var(--font-mono); font-size: 1.25rem; font-weight: 700; color: var(--color-cyan); }
.ch-name { font-size: 0.8rem; color: var(--text-muted); }

.state-badge {
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    letter-spacing: 0.05em;
}
.state-disconnected { background: #374151; color: #9ca3af; }
.state-connected { background: rgba(0, 255, 136, 0.15); color: var(--color-green); border: 1px solid var(--color-green); }
.state-transmitting { background: rgba(255, 51, 102, 0.2); color: var(--color-red); border: 1px solid var(--color-red); animation: pulse 1s infinite; }
.state-receiving { background: rgba(0, 240, 255, 0.2); color: var(--color-cyan); border: 1px solid var(--color-cyan); }
.state-fault { background: #ef4444; color: #fff; }

@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.6; } }

/* Indicators */
.ch-indicators { display: flex; gap: 1rem; align-items: center; }
.indicator-light {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    font-family: var(--font-mono);
}
.lamp { width: 10px; height: 10px; border-radius: 50%; background: #374151; }
.lamp.active.squ { background: var(--color-green); box-shadow: 0 0 8px var(--color-green); }
.lamp.active.ptt { background: var(--color-red); box-shadow: 0 0 8px var(--color-red); }

/* PTT Button */
.ptt-button-container { margin-top: 0.5rem; }
.btn-ptt {
    width: 100%;
    padding: 1.1rem;
    font-family: var(--font-mono);
    font-size: 1.1rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    color: #fff;
    background: linear-gradient(180deg, #374151, #1f2937);
    border: 2px solid #4b5563;
    border-radius: 8px;
    cursor: pointer;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
    user-select: none;
    transition: all 0.1s;
}
.btn-ptt:hover { border-color: #9ca3af; }
.btn-ptt:active, .btn-ptt.active {
    background: linear-gradient(180deg, #dc2626, #991b1b);
    border-color: #f87171;
    transform: translateY(2px);
    box-shadow: 0 0 20px rgba(239, 68, 68, 0.6);
}
.btn-ptt:disabled { opacity: 0.3; cursor: not-allowed; transform: none; }

/* Sliders & Stats */
.control-row { display: flex; justify-content: space-between; align-items: center; font-size: 0.75rem; }
.slider-control { display: flex; align-items: center; gap: 0.5rem; }
.slider-control input[type="range"] { accent-color: var(--color-cyan); width: 110px; }

.stats-box {
    background: rgba(0, 0, 0, 0.3);
    border-radius: 4px;
    padding: 0.5rem;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.4rem;
    font-family: var(--font-mono);
    font-size: 0.7rem;
}
.stats-box .val { font-weight: 700; color: #fff; }

/* Buttons */
.btn-neon-small {
    background: rgba(0, 240, 255, 0.1);
    border: 1px solid var(--color-cyan);
    color: var(--color-cyan);
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.3rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s;
}
.btn-neon-small:hover { background: var(--color-cyan); color: #000; box-shadow: 0 0 10px var(--color-cyan); }
.btn-neon-small.active {
    background: rgba(0, 240, 255, 0.25);
    color: #00f0ff;
    box-shadow: 0 0 10px rgba(0, 240, 255, 0.5);
}
.btn-neon-small.muted {
    background: rgba(100, 116, 139, 0.2);
    border-color: #64748b;
    color: #94a3b8;
    box-shadow: none;
}
.btn-secondary {
    background: #374151;
    border: 1px solid #4b5563;
    color: #f3f4f6;
    font-size: 0.75rem;
    padding: 0.35rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
}
.btn-secondary:hover { background: #4b5563; }
.btn-danger { background: #dc2626; border: none; color: #fff; padding: 0.4rem 0.8rem; border-radius: 4px; cursor: pointer; }

/* Spectrum Canvas */
.visualizer-container {
    background: #05070d;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    padding: 0.5rem;
    position: relative;
    margin-bottom: 0.5rem;
}
#spectrum-canvas { width: 100%; height: 160px; display: block; }
.spectrum-labels {
    display: flex;
    justify-content: space-between;
    font-size: 0.65rem;
    font-family: var(--font-mono);
    color: var(--text-muted);
}
.vu-meter-container {
    height: 6px;
    background: #1f2937;
    border-radius: 3px;
    margin-top: 0.5rem;
    overflow: hidden;
}
.vu-bar {
    height: 100%;
    width: 0%;
    background: linear-gradient(90deg, var(--color-green) 60%, var(--color-amber) 85%, var(--color-red) 100%);
    transition: width 0.05s ease-out;
}
.ptt-guide {
    margin-top: 1rem;
    padding: 0.75rem;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 6px;
    font-size: 0.78rem;
}
.ptt-guide code {
    background: #1f2937;
    color: var(--color-cyan);
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    font-family: var(--font-mono);
}

/* Telephony */
.telephony-layout { display: grid; grid-template-columns: 1fr 340px; gap: 1.25rem; }
@media (max-width: 768px) { .telephony-layout { grid-template-columns: 1fr; } }

.da-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1rem; }
.da-button {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 1.25rem;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
}
.da-button:hover { border-color: var(--color-cyan); background: rgba(0, 240, 255, 0.05); }
.da-button .da-title { font-weight: 700; font-size: 1rem; margin-bottom: 0.25rem; }
.da-button .da-uri { font-size: 0.7rem; color: var(--text-muted); font-family: var(--font-mono); }

/* Tables */
.data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    margin-top: 0.5rem;
}
.data-table th {
    text-align: left;
    padding: 0.6rem 0.8rem;
    background: rgba(255, 255, 255, 0.03);
    color: var(--text-muted);
    font-weight: 600;
    border-bottom: 1px solid var(--border-subtle);
}
.data-table td {
    padding: 0.6rem 0.8rem;
    border-bottom: 1px solid var(--border-subtle);
    font-family: var(--font-mono);
}
.data-table tr:hover td { background: rgba(255, 255, 255, 0.02); }

/* PCAP Upload */
.upload-zone {
    border: 2px dashed #374151;
    border-radius: 8px;
    padding: 3rem;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
}
.upload-zone:hover { border-color: var(--color-cyan); background: rgba(0, 240, 255, 0.03); }
.upload-icon { font-size: 2.5rem; margin-bottom: 0.75rem; }

/* Footer */
.app-footer {
    border-top: 1px solid var(--border-subtle);
    padding: 0.75rem 1.5rem;
    font-size: 0.7rem;
    color: var(--text-muted);
    text-align: center;
}

/* Modal Overlay & Dialog */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(4px);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 9999;
}
.modal-content {
    background: #111827;
    border: 1px solid var(--color-cyan);
    box-shadow: 0 0 25px rgba(0, 240, 255, 0.2);
    border-radius: 8px;
    width: 90%;
    max-width: 780px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
}
.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--border-subtle);
}
.modal-header h2 { font-size: 1.15rem; color: var(--color-cyan); }
.modal-body {
    padding: 1.5rem;
    overflow-y: auto;
    font-size: 0.9rem;
}
.btn-close {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.5rem;
    cursor: pointer;
    line-height: 1;
}
.btn-close:hover { color: #fff; }

/* Log Terminal & Comm Logs */
.log-terminal {
    background: #080c14;
    border-radius: 0 0 8px 8px;
    height: 480px;
    overflow-y: auto;
    font-family: var(--font-mono);
    font-size: 0.78rem;
    padding: 0.75rem 1rem;
    line-height: 1.6;
    display: flex;
    flex-direction: column;
    gap: 2px;
}
.log-row {
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
    padding: 2px 4px;
    border-radius: 3px;
    transition: background 0.1s;
}
.log-row:hover { background: rgba(255, 255, 255, 0.04); }
.log-time { color: #6b7280; font-size: 0.72rem; min-width: 82px; }
.log-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-weight: 700;
    font-size: 0.68rem;
    text-transform: uppercase;
}
.badge-SIP { background: #1d4ed8; color: #93c5fd; }
.badge-ED-137 { background: #0e7490; color: #67e8f9; }
.badge-RTP { background: #15803d; color: #86efac; }
.badge-SYS { background: #6b21a8; color: #d8b4fe; }

.log-dir { font-weight: 600; font-size: 0.72rem; }
.dir-TX { color: #f87171; }
.dir-RX { color: #4ade80; }
.dir-INT { color: #9ca3af; }

.log-msg { color: #e5e7eb; word-break: break-all; }
.log-level-WARN .log-msg { color: #fbbf24; }
.log-level-ERROR .log-msg { color: #f87171; font-weight: 700; }

.filter-group { display: flex; gap: 2px; background: rgba(0,0,0,0.3); padding: 2px; border-radius: 4px; }
.btn-filter {
    background: transparent;
    border: none;
    color: var(--text-muted);
    padding: 0.2rem 0.5rem;
    border-radius: 3px;
    font-size: 0.72rem;
    cursor: pointer;
}
.btn-filter.active { background: var(--color-cyan); color: #000; font-weight: 700; }

/* Live Ticker Bar */
.live-ticker-bar {
    background: #0d131f;
    border-top: 1px solid var(--border-subtle);
    padding: 0.4rem 1.25rem;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-family: var(--font-mono);
    font-size: 0.75rem;
    cursor: pointer;
    transition: background 0.2s;
}
.live-ticker-bar:hover { background: #141c2e; }
.ticker-label { color: var(--color-cyan); font-weight: 700; white-space: nowrap; }
.ticker-text { color: #d1d5db; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.ticker-hint { color: var(--text-muted); font-size: 0.7rem; white-space: nowrap; }
`

const AppJS = `// Aerovoice VCS Client Audio Engine & WebSocket Handler

let ws = null;
let audioCtx = null;
let micStream = null;
let micNode = null;
let analyser = null;
let canvasCtx = null;
let isPTTActive = false;
let activePTTChannel = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    initWebSocket();
    setupCanvas();
    setupKeyboardPTT();
    loadCommLogs();
    setInterval(loadCommLogs, 3000); // Periodic sync fallback
});

let unreadLogCount = 0;

function showTab(tabId) {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

    const activeBtn = document.querySelector('button[onclick="showTab(\'' + tabId + '\')"]');
    if (activeBtn) activeBtn.classList.add('active');

    const target = document.getElementById('tab-' + tabId);
    if (target) {
        target.classList.add('active');
        if (tabId === 'logs') {
            unreadLogCount = 0;
            const badge = document.getElementById('log-unread-badge');
            if (badge) badge.style.display = 'none';
            loadCommLogs();
            const term = document.getElementById('vcs-log-terminal');
            if (term) term.scrollTop = term.scrollHeight;
        } else if (tabId === 'recordings') {
            const container = document.getElementById('recordings-container');
            if (container && window.htmx) {
                htmx.ajax('GET', '/ui/components/recordings-table', '#recordings-container');
            }
        }
    }
}

// WebSocket Connection
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws';
    ws = new WebSocket(wsUrl);
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
        document.getElementById('ws-indicator').className = 'ws-status connected';
        document.getElementById('ws-label').innerText = 'WS Connected';
    };

    ws.onclose = () => {
        document.getElementById('ws-indicator').className = 'ws-status disconnected';
        document.getElementById('ws-label').innerText = 'WS Reconnecting...';
        setTimeout(initWebSocket, 2000);
    };

    ws.onmessage = (event) => {
        if (typeof event.data === 'string') {
            try {
                const msg = JSON.parse(event.data);
                handleEventMessage(msg);
            } catch (e) {
                console.error('WS JSON parse error:', e);
            }
        } else if (event.data instanceof ArrayBuffer) {
            handleBinaryAudio(event.data);
        }
    };
}

function handleEventMessage(msg) {
    if (msg.type === 'log' && msg.data) {
        appendLogEntry(msg.data);
    }
}

// Web Audio API Setup & Toggle
let isAudioActive = false;
let nextPlayTime = 0;

async function toggleWebAudio() {
    const btn = document.getElementById('btn-audio-init');

    if (isAudioActive) {
        // Complete Disable: Close AudioContext and release hardware
        isAudioActive = false;
        if (micStream) {
            micStream.getTracks().forEach(t => t.stop());
            micStream = null;
        }
        if (audioCtx) {
            try {
                await audioCtx.close();
            } catch (e) {
                console.warn('Error closing audioCtx:', e);
            }
            audioCtx = null;
            analyser = null;
        }
        nextPlayTime = 0;

        // Reset VU meter
        const vuBar = document.getElementById('vu-bar');
        if (vuBar) vuBar.style.width = '0%';

        if (btn) {
            btn.innerHTML = '<span class="icon">&#128263;</span> Audio OFF';
            btn.classList.remove('active');
            btn.classList.add('muted');
        }
    } else {
        // Enable: Fresh initialization
        try {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 8000 });
            analyser = audioCtx.createAnalyser();
            analyser.fftSize = 256;
            analyser.smoothingTimeConstant = 0.8;

            // Try getting microphone access
            try {
                micStream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, sampleRate: 8000 } });
                const micSource = audioCtx.createMediaStreamSource(micStream);
                
                // Script processor to sample PCM 16-bit
                const processor = audioCtx.createScriptProcessor(512, 1, 1);
                processor.onaudioprocess = (e) => {
                    if (isAudioActive && isPTTActive && ws && ws.readyState === WebSocket.OPEN) {
                        const inputData = e.inputBuffer.getChannelData(0);
                        const pcm16 = new Int16Array(inputData.length);
                        for (let i = 0; i < inputData.length; i++) {
                            let s = Math.max(-1, Math.min(1, inputData[i]));
                            pcm16[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
                        }
                        ws.send(pcm16.buffer);
                    }
                };
                micSource.connect(processor);

                // Zero-gain routing: Prevents microphone background noise from leaking to the speaker
                const muteGain = audioCtx.createGain();
                muteGain.gain.value = 0;
                processor.connect(muteGain);
                muteGain.connect(audioCtx.destination);
            } catch (e) {
                console.warn('Microphone access denied or unavailable, synthetic tone will be used for PTT:', e);
            }

            if (audioCtx.state === 'suspended') {
                await audioCtx.resume();
            }

            isAudioActive = true;
            nextPlayTime = audioCtx.currentTime + 0.025;

            if (btn) {
                btn.innerHTML = '<span class="icon">&#128266;</span> Audio ON';
                btn.classList.remove('muted');
                btn.classList.add('active');
            }

            startSpectrumRender();
        } catch (err) {
            console.error('Failed to init Web Audio:', err);
            isAudioActive = false;
        }
    }
}

// Backward compatibility for existing buttons or scripts
function initWebAudio() {
    return toggleWebAudio();
}

function handleBinaryAudio(arrayBuffer) {
    if (!audioCtx || !isAudioActive) return;

    const pcm16 = new Int16Array(arrayBuffer);
    const float32Array = new Float32Array(pcm16.length);

    let sumSquares = 0;
    for (let i = 0; i < pcm16.length; i++) {
        const val = pcm16[i] / 32768.0;
        float32Array[i] = val;
        sumSquares += val * val;
    }

    const audioBuf = audioCtx.createBuffer(1, float32Array.length, 8000);
    audioBuf.copyToChannel(float32Array, 0);

    // Low-Latency Adaptive Scheduling with Strict Max Latency Cap (<= 50ms)
    const now = audioCtx.currentTime;
    const targetBuffer = 0.020; // 20ms target jitter buffer
    const maxBuffer = 0.050;    // 50ms maximum buffer cap (strict real-time constraint)

    if (nextPlayTime < now || nextPlayTime > now + maxBuffer) {
        // Fast-forward / re-sync when buffer starves or latency accumulates
        nextPlayTime = now + targetBuffer;
    }

    // Playback
    const source = audioCtx.createBufferSource();
    source.buffer = audioBuf;
    source.connect(analyser);
    analyser.connect(audioCtx.destination);
    source.start(nextPlayTime);
    nextPlayTime += audioBuf.duration;

    // VU meter
    const rms = Math.sqrt(sumSquares / pcm16.length);
    const vuPct = Math.min(100, Math.round(rms * 400));
    const vuBar = document.getElementById('vu-bar');
    if (vuBar) vuBar.style.width = vuPct + '%';
}

// Canvas Spectrum Visualizer
function setupCanvas() {
    const canvas = document.getElementById('spectrum-canvas');
    if (canvas) canvasCtx = canvas.getContext('2d');
}

function startSpectrumRender() {
    if (!analyser || !canvasCtx) return;

    const canvas = document.getElementById('spectrum-canvas');
    const bufferLength = analyser.frequencyBinCount;
    const dataArray = new Uint8Array(bufferLength);

    function render() {
        if (!isAudioActive || !analyser) return;
        requestAnimationFrame(render);
        analyser.getByteFrequencyData(dataArray);

        canvasCtx.fillStyle = '#05070d';
        canvasCtx.fillRect(0, 0, canvas.width, canvas.height);

        // Grid lines
        canvasCtx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
        canvasCtx.lineWidth = 1;
        for (let y = 30; y < canvas.height; y += 30) {
            canvasCtx.beginPath();
            canvasCtx.moveTo(0, y);
            canvasCtx.lineTo(canvas.width, y);
            canvasCtx.stroke();
        }

        const barWidth = (canvas.width / bufferLength) * 2.2;
        let x = 0;

        for (let i = 0; i < bufferLength; i++) {
            const barHeight = (dataArray[i] / 255.0) * canvas.height;

            const grad = canvasCtx.createLinearGradient(0, canvas.height, 0, canvas.height - barHeight);
            grad.addColorStop(0, '#00ff88');
            grad.addColorStop(0.7, '#00f0ff');
            grad.addColorStop(1, '#ff3366');

            canvasCtx.fillStyle = grad;
            canvasCtx.fillRect(x, canvas.height - barHeight, barWidth - 1, barHeight);

            x += barWidth;
        }
    }
    render();
}

// PTT Handling (Button + Keyboard SPACE)
function handlePTTStart(channelID, pttType) {
    if (isPTTActive) return;
    initWebAudio();
    isPTTActive = true;
    activePTTChannel = channelID;

    fetch('/api/radio/ptt?id=' + encodeURIComponent(channelID) + '&action=start&type=' + (pttType || 'normal'), { method: 'POST' });

    const btn = document.getElementById('ptt-btn-' + channelID);
    if (btn) btn.classList.add('active');
}

function handlePTTStop(channelID) {
    if (!isPTTActive) return;
    isPTTActive = false;

    fetch('/api/radio/ptt?id=' + encodeURIComponent(channelID) + '&action=stop', { method: 'POST' });

    const btn = document.getElementById('ptt-btn-' + channelID);
    if (btn) btn.classList.remove('active');
    activePTTChannel = null;
}

function triggerVoiceTransmission(channelID) {
    initWebAudio();
    const btn = document.getElementById('voice-btn-' + channelID);
    if (btn) {
        btn.innerText = '🗣️ Speaking (ATC Voice)...';
        btn.classList.add('active');
        btn.disabled = true;
    }
    fetch('/api/radio/send-voice?id=' + encodeURIComponent(channelID), { method: 'POST' })
        .then(() => {
            setTimeout(() => {
                if (btn) {
                    btn.innerText = '🗣️ Send ATC Voice (Speech TX)';
                    btn.classList.remove('active');
                    btn.disabled = false;
                }
            }, 4500);
        })
        .catch(err => {
            console.error('Failed to send voice TX:', err);
            if (btn) {
                btn.innerText = '🗣️ Send ATC Voice (Speech TX)';
                btn.classList.remove('active');
                btn.disabled = false;
            }
        });
}

function setupKeyboardPTT() {
    window.addEventListener('keydown', (e) => {
        if (e.code === 'Space' && !e.repeat && !isPTTActive) {
            // Find first connected channel
            const connectedBtn = document.querySelector('.btn-ptt:not(:disabled)');
            if (connectedBtn) {
                const chId = connectedBtn.getAttribute('data-channel');
                e.preventDefault();
                handlePTTStart(chId, 'normal');
            }
        }
    });

    window.addEventListener('keyup', (e) => {
        if (e.code === 'Space' && isPTTActive && activePTTChannel) {
            e.preventDefault();
            handlePTTStop(activePTTChannel);
        }
    });
}

// PCAP Upload Handler
async function handlePCAPUpload(input) {
    const file = input.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('pcap', file);

    const resDiv = document.getElementById('pcap-results');
    resDiv.style.display = 'block';
    resDiv.innerHTML = '<div class="loading-spinner">Analyzing PCAP with Pure Go Engine...</div>';

    try {
        const resp = await fetch('/api/pcap/upload', { method: 'POST', body: formData });
        const report = await resp.json();

        if (resp.status !== 200) {
            resDiv.innerHTML = '<div class="alert danger">Error: ' + (report.error || 'Failed to analyze PCAP') + '</div>';
            return;
        }

        renderPCAPReport(report);
    } catch (err) {
        resDiv.innerHTML = '<div class="alert danger">Upload failed: ' + err.message + '</div>';
    }
}

function renderPCAPReport(report) {
    const resDiv = document.getElementById('pcap-results');
    let html = '<div class="card glass" style="margin-bottom: 1.5rem;">' +
        '<div class="card-header">' +
            '<h3>PCAP Analysis Report: ' + report.file_name + '</h3>' +
        '</div>' +
        '<div class="card-body">' +
            '<div class="stats-box" style="grid-template-columns: repeat(4, 1fr); margin-bottom: 1.25rem;">' +
                '<div>Total Packets: <span class="val">' + report.total_packets + '</span></div>' +
                '<div>RTP Audio: <span class="val">' + report.rtp_packets + '</span></div>' +
                '<div>SIP Packets: <span class="val">' + report.sip_packets + '</span></div>' +
                '<div>Duration: <span class="val">' + report.duration_str + '</span></div>' +
                '<div>Codec: <span class="val">' + report.codec_name + '</span></div>' +
                '<div>Avg Jitter: <span class="val">' + report.avg_jitter_ms.toFixed(2) + ' ms</span></div>' +
                '<div>Loss Rate: <span class="val">' + report.loss_rate_pct.toFixed(2) + '%</span></div>' +
                '<div>Avg SQI: <span class="val">' + report.avg_sqi.toFixed(1) + ' / 100</span></div>' +
            '</div>' +
            '<div style="margin-bottom: 1.5rem;">' +
                '<h4>Extracted Audio Playback (From RTP Stream)</h4>' +
                '<audio controls src="/api/pcap/wav" style="width: 100%; margin-top: 0.5rem;"></audio>' +
            '</div>' +
            '<h4>ED-137 Dissected Packets (First 50)</h4>' +
            '<div style="overflow-x: auto;">' +
                '<table class="data-table">' +
                    '<thead>' +
                        '<tr>' +
                            '<th>#</th>' +
                            '<th>Time</th>' +
                            '<th>Proto</th>' +
                            '<th>Source &rarr; Dest</th>' +
                            '<th>Seq</th>' +
                            '<th>PTT</th>' +
                            '<th>SQU</th>' +
                            '<th>SQI</th>' +
                            '<th>Jitter</th>' +
                            '<th>Info</th>' +
                        '</tr>' +
                    '</thead>' +
                    '<tbody>';

    for (let p of report.packets) {
        html += '<tr>' +
            '<td>' + p.index + '</td>' +
            '<td>+' + p.time_offset_s.toFixed(3) + 's</td>' +
            '<td><span class="tag">' + p.protocol + '</span></td>' +
            '<td>' + p.src_addr + ' &rarr; ' + p.dst_addr + '</td>' +
            '<td>' + (p.seq || '-') + '</td>' +
            '<td style="color:' + (p.ptt !== 'OFF' ? 'var(--color-red)' : 'inherit') + '">' + p.ptt + '</td>' +
            '<td style="color:' + (p.squelch ? 'var(--color-green)' : 'inherit') + '">' + (p.squelch ? 'ON' : 'OFF') + '</td>' +
            '<td>' + (p.sqi || '-') + '</td>' +
            '<td>' + (p.jitter_ms ? p.jitter_ms.toFixed(2) + 'ms' : '-') + '</td>' +
            '<td>' + p.info + '</td>' +
        '</tr>';
    }

    html += '</tbody></table></div></div></div>';
    resDiv.innerHTML = html;
}

function toggleManualModal() {
    const modal = document.getElementById('modal-manual');
    if (!modal) return;
    if (modal.style.display === 'none' || modal.style.display === '') {
        modal.style.display = 'flex';
    } else {
        modal.style.display = 'none';
    }
}

// Comm Logs Management
let allCommLogs = [];
let currentLogFilter = 'ALL';

async function loadCommLogs() {
    try {
        const resp = await fetch('/api/logs');
        if (resp.ok) {
            const data = await resp.json();
            if (Array.isArray(data)) {
                allCommLogs = data;
                renderLogs();
                if (allCommLogs.length > 0) {
                    updateTicker(allCommLogs[allCommLogs.length - 1]);
                }
            }
        }
    } catch (e) {
        console.error('Failed to load logs:', e);
    }
}

function appendLogEntry(entry) {
    if (!allCommLogs) allCommLogs = [];
    allCommLogs.push(entry);
    if (allCommLogs.length > 300) {
        allCommLogs.shift();
    }
    updateTicker(entry);

    const logsTab = document.getElementById('tab-logs');
    if (!logsTab || !logsTab.classList.contains('active')) {
        unreadLogCount++;
        const badge = document.getElementById('log-unread-badge');
        if (badge) {
            badge.innerText = unreadLogCount > 99 ? '99+' : unreadLogCount;
            badge.style.display = 'inline-block';
        }
    }

    if (currentLogFilter === 'ALL' || entry.protocol === currentLogFilter) {
        const terminal = document.getElementById('vcs-log-terminal');
        if (terminal) {
            if (terminal.querySelector('.muted')) {
                terminal.innerHTML = '';
            }
            const row = createLogRowElement(entry);
            terminal.appendChild(row);
            const chk = document.getElementById('chk-auto-scroll');
            if (chk && chk.checked) {
                terminal.scrollTop = terminal.scrollHeight;
            }
        }
    }
}

function updateTicker(entry) {
    const el = document.getElementById('ticker-text');
    if (!el) return;
    const dirColor = entry.direction === 'TX' ? '#f87171' : entry.direction === 'RX' ? '#4ade80' : '#9ca3af';
    el.innerHTML = '<span style="color:#6b7280;">[' + entry.timestamp + ']</span> ' +
                   '<strong style="color:var(--color-cyan);">[' + entry.protocol + ']</strong> ' +
                   '<span style="color:' + dirColor + ';">[' + entry.direction + ']</span> ' +
                   escapeHtml(entry.message);
}

function createLogRowElement(entry) {
    const div = document.createElement('div');
    div.className = 'log-row log-proto-' + entry.protocol + ' log-level-' + entry.level;
    div.innerHTML = '<span class="log-time">[' + entry.timestamp + ']</span>' +
                    '<span class="log-badge badge-' + entry.protocol + '">' + entry.protocol + '</span>' +
                    '<span class="log-dir dir-' + entry.direction + '">[' + entry.direction + ']</span>' +
                    '<span class="log-msg">' + escapeHtml(entry.message) + '</span>';
    return div;
}

function renderLogs() {
    const terminal = document.getElementById('vcs-log-terminal');
    if (!terminal) return;
    terminal.innerHTML = '';

    if (!allCommLogs || allCommLogs.length === 0) {
        terminal.innerHTML = '<div class="muted" style="padding: 1rem;">No communication logs recorded yet.</div>';
        return;
    }

    const filtered = allCommLogs.filter(l => currentLogFilter === 'ALL' || l.protocol === currentLogFilter);
    if (filtered.length === 0) {
        terminal.innerHTML = '<div class="muted" style="padding: 1rem;">No ' + currentLogFilter + ' communication logs.</div>';
        return;
    }

    filtered.forEach(entry => {
        terminal.appendChild(createLogRowElement(entry));
    });

    const chk = document.getElementById('chk-auto-scroll');
    if (chk && chk.checked) {
        terminal.scrollTop = terminal.scrollHeight;
    }
}

function filterLogs(proto) {
    currentLogFilter = proto;
    document.querySelectorAll('.btn-filter').forEach(btn => {
        if (btn.innerText.toUpperCase() === proto) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
    renderLogs();
}

async function clearLogs() {
    try {
        await fetch('/api/logs/clear', { method: 'POST' });
        allCommLogs = [];
        renderLogs();
        const ticker = document.getElementById('ticker-text');
        if (ticker) ticker.innerText = 'Logs cleared.';
    } catch (e) {
        console.error('Failed to clear logs:', e);
    }
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
`
