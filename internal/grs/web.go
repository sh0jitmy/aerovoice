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

package grs

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media/codec"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local emulator
	},
}

// WebServer provides the GRS emulator testbench console UI.
type WebServer struct {
	svc       *Service
	server    *http.Server
	wsClients map[*websocket.Conn]bool
	wsMu      sync.Mutex
}

// NewWebServer initializes the GRS testbench Web console.
func NewWebServer(svc *Service, host string, port int) (*WebServer, error) {
	ws := &WebServer{
		svc:       svc,
		wsClients: make(map[*websocket.Conn]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/api/snapshot", ws.handleSnapshot)
	mux.HandleFunc("/api/squelch", ws.handleSquelch)
	mux.HandleFunc("/api/source", ws.handleSource)
	mux.HandleFunc("/api/impairment", ws.handleImpairment)
	mux.HandleFunc("/api/silent_drop", ws.handleSilentDrop)
	mux.HandleFunc("/api/call", ws.handleCall)
	mux.HandleFunc("/api/logs/clear", ws.handleClearLogs)
	mux.HandleFunc("/ws/audio", ws.handleWSAudio)

	ws.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start audio broadcast worker to WebSockets
	go ws.broadcastAudioWorker()

	return ws, nil
}

// Start launches the GRS HTTP server in a goroutine.
func (w *WebServer) Start() error {
	ln, err := net.Listen("tcp", w.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind GRS Web Console to %s: %w", w.server.Addr, err)
	}
	w.server.Addr = ln.Addr().String()

	slog.Info("Starting GRS Testbench Web Console", "addr", w.server.Addr)
	go func() {
		if err := w.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("GRS Web server failed", "error", err)
		}
	}()
	return nil
}

// Addr returns the network address that the web server is listening on.
func (w *WebServer) Addr() string {
	if w.server != nil {
		return w.server.Addr
	}
	return ""
}

// Close terminates the web server.
func (w *WebServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	w.wsMu.Lock()
	for conn := range w.wsClients {
		_ = conn.Close()
	}
	w.wsMu.Unlock()

	return w.server.Shutdown(ctx)
}

func (w *WebServer) handleIndex(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	snap := w.svc.GetSnapshot()
	_ = grsTemplate.Execute(rw, snap)
}

func (w *WebServer) handleSnapshot(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	snap := w.svc.GetSnapshot()
	_ = json.NewEncoder(rw).Encode(snap)
}

func (w *WebServer) handleSquelch(rw http.ResponseWriter, r *http.Request) {
	onStr := r.URL.Query().Get("on")
	on := onStr == "true" || onStr == "1"
	w.svc.SetSquelch(on)
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleSource(rw http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	if src != "" {
		w.svc.SetAudioSource(src)
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleImpairment(rw http.ResponseWriter, r *http.Request) {
	jitterStr := r.URL.Query().Get("jitter")
	lossStr := r.URL.Query().Get("loss")

	jitter, _ := strconv.Atoi(jitterStr)
	loss, _ := strconv.Atoi(lossStr)
	w.svc.SetImpairment(jitter, loss)
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleSilentDrop(rw http.ResponseWriter, r *http.Request) {
	dropStr := r.URL.Query().Get("drop")
	drop := dropStr == "true" || dropStr == "1"
	w.svc.SetSilentDrop(drop)
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleCall(rw http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		target = w.svc.Config().GRS.VCSSIPURI
		if target == "" {
			target = "sip:101@127.0.0.1:5060"
		}
	}
	err := w.svc.CallVCS(target)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleWSAudio(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	w.wsMu.Lock()
	w.wsClients[conn] = true
	w.wsMu.Unlock()

	// Read loop to handle ping/pong and close
	go func() {
		defer func() {
			w.wsMu.Lock()
			delete(w.wsClients, conn)
			w.wsMu.Unlock()
			_ = conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

func (w *WebServer) handleClearLogs(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.svc.ClearLogs()
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) broadcastAudioWorker() {
	ch := w.svc.SubscribeAudio()
	for samples := range ch {
		pcmBytes := codec.Int16ToPCMBytes(samples)

		w.wsMu.Lock()
		for conn := range w.wsClients {
			_ = conn.WriteMessage(websocket.BinaryMessage, pcmBytes)
		}
		w.wsMu.Unlock()
	}
}

// grsTemplate is the self-contained HTML/CSS/JS for GRS Emulator Console.
var grsTemplate = template.Must(template.New("grs").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>GRS-Emulator Testbench Console (:8081)</title>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --text: #c9d1d9;
      --accent: #58a6ff;
      --green: #2ea043;
      --red: #f85149;
      --yellow: #d29922;
    }
    body {
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace;
      margin: 0;
      padding: 16px;
    }
    .header {
      display: flex;
      justify-content: space-between;
      border-bottom: 1px solid var(--border);
      padding-bottom: 10px;
      margin-bottom: 16px;
    }
    .badge {
      padding: 4px 8px;
      border-radius: 4px;
      font-weight: bold;
      font-size: 12px;
    }
    .badge-online { background: var(--green); color: #fff; }
    .badge-offline { background: var(--red); color: #fff; }
    .badge-idle { background: #484f58; color: #fff; }
    .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 16px;
    }
    h2 { margin-top: 0; color: var(--accent); font-size: 16px; border-bottom: 1px solid var(--border); padding-bottom: 6px; }
    .meter-container {
      background: #21262d;
      border-radius: 4px;
      height: 16px;
      overflow: hidden;
      margin: 8px 0;
    }
    .meter-bar {
      background: linear-gradient(90deg, #2ea043, #d29922, #f85149);
      height: 100%;
      width: 0%;
      transition: width 0.1s;
    }
    button {
      background: #21262d;
      border: 1px solid var(--border);
      color: var(--text);
      padding: 6px 12px;
      border-radius: 4px;
      cursor: pointer;
      font-weight: bold;
    }
    button:hover { background: #30363d; }
    button.active { background: var(--green); color: #fff; }
    button.danger { background: var(--red); color: #fff; }
    .ptt-box {
      padding: 12px;
      border-radius: 6px;
      text-align: center;
      font-size: 18px;
      font-weight: bold;
      margin: 8px 0;
      border: 1px solid var(--border);
      background: #21262d;
    }
    .ptt-active { background: var(--red); color: #fff; animation: pulse 1s infinite; }
    @keyframes pulse { 0% { opacity: 0.9; } 50% { opacity: 1; } 100% { opacity: 0.9; } }
    .log-box {
      background: #06090f;
      border: 1px solid var(--border);
      border-radius: 4px;
      padding: 8px;
      height: 200px;
      overflow-y: auto;
      font-size: 12px;
      font-family: monospace;
      line-height: 1.5;
    }
    .log-entry { margin-bottom: 4px; display: flex; gap: 6px; align-items: baseline; }
    .log-entry:hover { background: rgba(255,255,255,0.03); }
    .badge-proto { padding: 1px 5px; border-radius: 3px; font-weight: bold; font-size: 10px; text-transform: uppercase; }
    .proto-SIP { background: #1d4ed8; color: #93c5fd; }
    .proto-ED-137 { background: #0e7490; color: #67e8f9; }
    .proto-RTP { background: #15803d; color: #86efac; }
    .proto-SYS { background: #6b21a8; color: #d8b4fe; }
    .dir-tag { font-weight: bold; font-size: 11px; }
    .dir-TX { color: #f85149; }
    .dir-RX { color: #2ea043; }
    .dir-INT { color: #8b949e; }
    .log-INFO { color: #c9d1d9; }
    .log-WARN { color: #d29922; }
    .log-ERROR { color: #f85149; }
    .slider-row { display: flex; align-items: center; gap: 10px; margin: 8px 0; }
    .tab-nav {
      display: flex;
      gap: 8px;
      margin-bottom: 16px;
      border-bottom: 1px solid var(--border);
      padding-bottom: 10px;
    }
    .tab-btn {
      background: #161b22;
      border: 1px solid var(--border);
      color: var(--text);
      padding: 8px 16px;
      border-radius: 6px;
      cursor: pointer;
      font-weight: 600;
      font-size: 13px;
      transition: all 0.2s ease;
    }
    .tab-btn:hover {
      background: #21262d;
      border-color: var(--accent);
      color: #fff;
    }
    .tab-btn.active {
      background: #1f6feb;
      border-color: #388bfd;
      color: #fff;
      box-shadow: 0 0 10px rgba(31, 111, 235, 0.4);
    }
    .tab-pane {
      display: none;
    }
    .tab-pane.active {
      display: block;
    }
  </style>
</head>
<body>
  <div class="header">
    <div>
      <h1 style="margin:0; font-size:20px; color:#fff;">📡 GRS-EMULATOR TESTBENCH CONSOLE</h1>
      <small>Ground Radio Station Simulator (Port 8081) - ED-137C Radio & Voice</small>
    </div>
    <div>
      <span class="badge badge-online">STATION: {{.StationName}} ({{.Frequency}})</span>
      <span id="session-badge" class="badge {{if .SessionConnected}}badge-online{{else}}badge-idle{{end}}">
        {{if .SessionConnected}}VCS CONNECTED{{else}}STANDBY (No SIP Call){{end}}
      </span>
    </div>
  </div>

  <!-- Tab Navigation Bar -->
  <div class="tab-nav">
    <button class="tab-btn active" id="tab-btn-radio" onclick="showTab('radio')">📻 Radio Transceiver</button>
    <button class="tab-btn" id="tab-btn-telephony" onclick="showTab('telephony')">📞 Telephony Test</button>
    <button class="tab-btn" id="tab-btn-supervision" onclick="showTab('supervision')">🩺 Supervision & Faults</button>
    <button class="tab-btn" id="tab-btn-logs" onclick="showTab('logs')">📋 Protocol Event Logs</button>
  </div>

  <!-- TAB 1: RADIO TRANSCEIVER (Uplink & Downlink) -->
  <div id="pane-radio" class="tab-pane active">
    <div class="grid">
      <!-- Uplink Receiver (VCS -> GRS) -->
      <div class="card">
        <h2>📥 Uplink Receiver (From VCS)</h2>
        <div id="ptt-indicator" class="ptt-box {{if .RxPTTActive}}ptt-active{{end}}">
          PTT: <span id="ptt-text">{{if .RxPTTActive}}ON (Type: {{.RxPTTType}}, ID: {{.RxPTTID}}){{else}}OFF (Idle){{end}}</span>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span>Live Speaker Audio Output:</span>
          <button id="btn-speaker" onclick="toggleSpeaker()">🔇 Speaker: OFF</button>
        </div>

        <div style="margin-top:10px;">
          <label>Rx Audio Level: <span id="rx-db">{{.RxLevelDB}}</span> dBFS</label>
          <div class="meter-container">
            <div id="rx-meter" class="meter-bar" style="width: 0%;"></div>
          </div>
        </div>

        <p style="font-size:12px;">
          Rx Jitter: <strong id="rx-jitter">{{.Stats.RxJitterMs}} ms</strong> |
          Peak: <span id="peak-jitter">{{.Stats.PeakRxJitterMs}} ms</span> |
          Loss: <span id="rx-loss">{{.Stats.LossRate}}%</span> |
          Packets: <span id="rx-pkts">{{.Stats.PacketsReceived}}</span>
        </p>
      </div>

      <!-- Downlink Transmitter (GRS -> VCS) -->
      <div class="card">
        <h2>📤 Downlink Transmitter (To VCS)</h2>
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
          <span>Squelch (SQU) Control:</span>
          <button id="btn-squelch" class="{{if .TxSQUActive}}active{{end}}" onclick="toggleSquelch()">
            {{if .TxSQUActive}}🔘 SQUELCH: ON (Transmitting){{else}}⚪ SQUELCH: OFF{{end}}
          </button>
        </div>

        <div style="margin-bottom:12px;">
          <label style="display:block; margin-bottom:6px; font-weight:bold;">Audio Signal Generator (送信音声):</label>
          <label><input type="radio" name="audiosrc" value="pilot_voice" {{if or (eq .AudioSource "pilot_voice") (eq .AudioSource "")}}checked{{end}} onchange="changeSource(this.value)"> 🧑‍✈️ <strong>パイロット音声 (テスト、テスト。本日は晴天なり、本日は晴天なり。)</strong></label><br>
          <label><input type="radio" name="audiosrc" value="telephony_voice" {{if eq .AudioSource "telephony_voice"}}checked{{end}} onchange="changeSource(this.value)"> 📞 <strong>電話音声 (テスト、テスト。本日は晴天なり、本日は晴天なり。)</strong></label><br>
          <label><input type="radio" name="audiosrc" value="tone_1khz" {{if eq .AudioSource "tone_1khz"}}checked{{end}} onchange="changeSource(this.value)"> 1 kHz Sine Tone</label><br>
          <label><input type="radio" name="audiosrc" value="beep_400hz" {{if eq .AudioSource "beep_400hz"}}checked{{end}} onchange="changeSource(this.value)"> 400 Hz ATC Beep</label><br>
          <label><input type="radio" name="audiosrc" value="simulated_voice" {{if eq .AudioSource "simulated_voice"}}checked{{end}} onchange="changeSource(this.value)"> Simulated Speech Formants</label><br>
          <label><input type="radio" name="audiosrc" value="loopback" {{if eq .AudioSource "loopback"}}checked{{end}} onchange="changeSource(this.value)"> 🔁 Loopback Echo (VCS Audio)</label>
        </div>

        <hr style="border:0; border-top:1px solid var(--border);">
        <h3 style="font-size:13px; color:var(--yellow); margin:6px 0;">⚡ Network Impairment Injection (人工障害注入)</h3>
        <div class="slider-row">
          <label style="width:140px;">Injected Jitter: <strong id="lbl-jitter">{{.InjJitterMs}}</strong> ms</label>
          <input type="range" id="rng-jitter" min="0" max="50" value="{{.InjJitterMs}}" onchange="updateImpairment()">
        </div>
        <div class="slider-row">
          <label style="width:140px;">Injected Loss: <strong id="lbl-loss">{{.InjLossPct}}</strong> %</label>
          <input type="range" id="rng-loss" min="0" max="30" value="{{.InjLossPct}}" onchange="updateImpairment()">
        </div>
      </div>
    </div>
  </div>

  <!-- TAB 2: TELEPHONY TEST -->
  <div id="pane-telephony" class="tab-pane">
    <div class="card" style="max-width: 600px;">
      <h2>📞 Telephony Test Panel</h2>
      <p style="font-size:13px;">Auto-Answer Mode: <strong>1 kHz Tone Responder</strong></p>
      <p style="font-size:12px; color:#8b949e; line-height:1.5;">
        Simulate direct access (DA) ground-to-ground calls to the VCS controller station. 
        Pressing the button initiates an ED-137 SIP INVITE to VCS Station 101 with G.711 μ-law audio stream.
      </p>
      <div style="margin-top:16px;">
        <button onclick="callVCS()" style="padding:10px 18px; font-size:14px; background:#1f6feb; border-color:#388bfd; color:#fff;">📞 Call VCS Station (101)</button>
      </div>
    </div>
  </div>

  <!-- TAB 3: SUPERVISION & FAULT SIMULATION -->
  <div id="pane-supervision" class="tab-pane">
    <div class="card" style="max-width: 600px;">
      <h2>🩺 Supervision & Fault Simulation</h2>
      <p style="font-size:13px;">SIP OPTIONS Heartbeat: Responds 200 OK</p>
      <p style="font-size:12px; color:#8b949e; line-height:1.5;">
        Simulate unexpected network outage or radio station blackout. 
        When Silent Drop is active, the GRS silently discards incoming SIP OPTIONS keep-alive packets, triggering VCS fault detection alarms.
      </p>
      <div style="margin-top:16px;">
        <button id="btn-silent-drop" class="{{if .SilentDrop}}danger{{end}}" onclick="toggleSilentDrop()" style="padding:10px 18px; font-size:14px;">
          {{if .SilentDrop}}⚠️ Silent Drop: ACTIVE (Simulating Offline){{else}}Simulate Silent Drop (Offline){{end}}
        </button>
      </div>
    </div>
  </div>

  <!-- TAB 4: PROTOCOL EVENT LOGS -->
  <div id="pane-logs" class="tab-pane">
    <div class="card">
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:10px;">
        <h2 style="margin:0; border-bottom:none; padding-bottom:0;">📋 GRS Protocol & Event Log (Real-time)</h2>
        <div style="display:flex; gap:8px; align-items:center;">
          <label style="font-size:12px; cursor:pointer;"><input type="checkbox" id="chk-grs-scroll" checked> Auto-scroll</label>
          <button onclick="clearGRSLogs()" style="padding:4px 10px; font-size:12px; background:#21262d; border:1px solid #30363d; color:#c9d1d9; border-radius:4px;">Clear Logs</button>
        </div>
      </div>
      <div id="log-box" class="log-box" style="height: 480px;">
        {{range .RecentLogs}}
        <div class="log-entry log-{{.Level}}">
          <span style="color:#8b949e;">[{{.Timestamp}}]</span>
          <span class="badge-proto proto-{{.Protocol}}">{{.Protocol}}</span>
          <span class="dir-tag dir-{{.Direction}}">[{{.Direction}}]</span>
          <span>{{.Message}}</span>
        </div>
        {{end}}
      </div>
    </div>
  </div>

  <script>
    let isSpeakerOn = false;
    let audioCtx = null;
    let ws = null;
    let nextPlayTime = 0;

    function initAudio() {
      if (!audioCtx) {
        audioCtx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 8000 });
      }
      if (!ws) {
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(proto + '//' + window.location.host + '/ws/audio');
        ws.binaryType = 'arraybuffer';
        ws.onmessage = function(e) {
          if (!isSpeakerOn || !audioCtx) return;
          const int16Array = new Int16Array(e.data);
          const float32Array = new Float32Array(int16Array.length);
          for (let i = 0; i < int16Array.length; i++) {
            float32Array[i] = int16Array[i] / 32768.0;
          }
          const audioBuffer = audioCtx.createBuffer(1, float32Array.length, 8000);
          audioBuffer.copyToChannel(float32Array, 0);

          const now = audioCtx.currentTime;
          const targetBuffer = 0.020; // 20ms target jitter buffer
          const maxBuffer = 0.050;    // 50ms maximum buffer cap (strict real-time constraint)
          if (nextPlayTime < now || nextPlayTime > now + maxBuffer) {
            nextPlayTime = now + targetBuffer;
          }
          const src = audioCtx.createBufferSource();
          src.buffer = audioBuffer;
          src.connect(audioCtx.destination);
          src.start(nextPlayTime);
          nextPlayTime += audioBuffer.duration;
        };
      }
    }

    function toggleSpeaker() {
      isSpeakerOn = !isSpeakerOn;
      const btn = document.getElementById('btn-speaker');
      if (isSpeakerOn) {
        initAudio();
        if (audioCtx && audioCtx.state === 'suspended') audioCtx.resume();
        btn.innerText = '🔊 Speaker: ON';
        btn.classList.add('active');
      } else {
        if (audioCtx) {
          try {
            audioCtx.close();
          } catch (e) {}
          audioCtx = null;
        }
        nextPlayTime = 0;
        btn.innerText = '🔇 Speaker: OFF';
        btn.classList.remove('active');
      }
    }

    let txSquelch = {{.TxSQUActive}};
    function toggleSquelch() {
      txSquelch = !txSquelch;
      fetch('/api/squelch?on=' + txSquelch, { method: 'POST' });
    }

    function changeSource(src) {
      fetch('/api/source?src=' + encodeURIComponent(src), { method: 'POST' });
    }

    function updateImpairment() {
      const jitter = document.getElementById('rng-jitter').value;
      const loss = document.getElementById('rng-loss').value;
      document.getElementById('lbl-jitter').innerText = jitter;
      document.getElementById('lbl-loss').innerText = loss;
      fetch('/api/impairment?jitter=' + jitter + '&loss=' + loss, { method: 'POST' });
    }

    let silentDrop = {{.SilentDrop}};
    function toggleSilentDrop() {
      silentDrop = !silentDrop;
      fetch('/api/silent_drop?drop=' + silentDrop, { method: 'POST' });
    }

    function callVCS() {
      const target = encodeURIComponent('{{.VCSSIPURI}}');
      fetch('/api/call?target=' + target, { method: 'POST' });
    }

    // Polling Snapshot for Live Status Updates
    setInterval(async () => {
      try {
        const res = await fetch('/api/snapshot');
        if (!res.ok) return;
        const snap = await res.json();

        // PTT Indicator
        const pttBox = document.getElementById('ptt-indicator');
        const pttText = document.getElementById('ptt-text');
        if (snap.rx_ptt_active) {
          pttBox.classList.add('ptt-active');
          pttText.innerText = 'ON (Type: ' + snap.rx_ptt_type + ', ID: ' + snap.rx_ptt_id + ')';
        } else {
          pttBox.classList.remove('ptt-active');
          pttText.innerText = 'OFF (Idle)';
        }

        // Rx Audio Meter
        document.getElementById('rx-db').innerText = snap.rx_level_db;
        const pct = Math.max(0, Math.min(100, (snap.rx_level_db + 60) * 1.66));
        document.getElementById('rx-meter').style.width = pct + '%';

        // Stats
        document.getElementById('rx-jitter').innerText = snap.stats.rx_jitter_ms + ' ms';
        document.getElementById('peak-jitter').innerText = snap.stats.peak_rx_jitter_ms + ' ms';
        document.getElementById('rx-loss').innerText = snap.stats.loss_rate + '%';
        document.getElementById('rx-pkts').innerText = snap.stats.packets_received;

        // Squelch button state
        const squBtn = document.getElementById('btn-squelch');
        txSquelch = snap.tx_squ_active;
        if (txSquelch) {
          squBtn.innerText = '🔘 SQUELCH: ON (Transmitting)';
          squBtn.classList.add('active');
        } else {
          squBtn.innerText = '⚪ SQUELCH: OFF';
          squBtn.classList.remove('active');
        }

        // Silent drop button state
        const dropBtn = document.getElementById('btn-silent-drop');
        silentDrop = snap.silent_drop;
        if (silentDrop) {
          dropBtn.innerText = '⚠️ Silent Drop: ACTIVE (Simulating Offline)';
          dropBtn.classList.add('danger');
        } else {
          dropBtn.innerText = 'Simulate Silent Drop (Offline)';
          dropBtn.classList.remove('danger');
        }

        // Session badge
        const sessBadge = document.getElementById('session-badge');
        if (snap.session_connected) {
          sessBadge.className = 'badge badge-online';
          sessBadge.innerText = 'VCS CONNECTED (' + snap.client_addr + ')';
        } else {
          sessBadge.className = 'badge badge-idle';
          sessBadge.innerText = 'STANDBY (No SIP Call)';
        }

        // Logs
        const logBox = document.getElementById('log-box');
        if (snap.recent_logs && snap.recent_logs.length > 0) {
          logBox.innerHTML = snap.recent_logs.map(l => {
            const proto = l.protocol || 'SYS';
            const dir = l.direction || 'INT';
            return '<div class="log-entry log-' + l.level + '">' +
              '<span style="color:#8b949e;">[' + l.timestamp + ']</span> ' +
              '<span class="badge-proto proto-' + proto + '">' + proto + '</span> ' +
              '<span class="dir-tag dir-' + dir + '">[' + dir + ']</span> ' +
              '<span>' + escapeHtml(l.message) + '</span>' +
              '</div>';
          }).join('');
          const chk = document.getElementById('chk-grs-scroll');
          if (chk && chk.checked) {
            logBox.scrollTop = logBox.scrollHeight;
          }
        }
      } catch (err) {
        // Network error
      }
    }, 500);

    async function clearGRSLogs() {
      try {
        await fetch('/api/logs/clear', { method: 'POST' });
        const logBox = document.getElementById('log-box');
        if (logBox) logBox.innerHTML = '<div class="log-entry" style="color:#8b949e;">Logs cleared.</div>';
      } catch (e) {
        console.error('Failed to clear GRS logs:', e);
      }
    }

    function escapeHtml(str) {
      if (!str) return '';
      return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    function showTab(tabId) {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

      const btn = document.getElementById('tab-btn-' + tabId);
      if (btn) btn.classList.add('active');

      const pane = document.getElementById('pane-' + tabId);
      if (pane) pane.classList.add('active');
    }

    document.addEventListener('DOMContentLoaded', () => {
      const urlParams = new URLSearchParams(window.location.search);
      const initialTab = urlParams.get('tab') || window.location.hash.replace('#', '');
      if (initialTab) {
        showTab(initialTab);
      }
    });
  </script>
</body>
</html>`))
