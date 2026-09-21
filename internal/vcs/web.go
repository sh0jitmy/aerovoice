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

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sh0jitmy/aerovoice/internal/ed137"
	"github.com/sh0jitmy/aerovoice/internal/media"
	"github.com/sh0jitmy/aerovoice/internal/pcap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow local origins for test & simulation
	},
}

// WebServer manages the VCS Web Dashboard and WebSocket connections.
type WebServer struct {
	svc         *VCSService
	recorder    *media.Recorder
	server      *http.Server
	wsClients   map[*websocket.Conn]bool
	wsMu        sync.Mutex
	lastPCAPWAV []byte
	lastPCAPMu  sync.RWMutex
}

// NewWebServer initializes and launches the VCS web console server on specified host/port.
func NewWebServer(svc *VCSService, recorder *media.Recorder, host string, port int) (*WebServer, error) {
	ws := &WebServer{
		svc:       svc,
		recorder:  recorder,
		wsClients: make(map[*websocket.Conn]bool),
	}

	svc.SetBroadcaster(ws)

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/static/style.css", ws.handleStyle)
	mux.HandleFunc("/static/app.js", ws.handleAppJS)
	mux.HandleFunc("/static/htmx.min.js", ws.handleHTMX)
	mux.HandleFunc("/ws", ws.handleWebSocket)

	// HTMX Partial Components
	mux.HandleFunc("/ui/components/radio-channels", ws.handleUIRadioChannels)
	mux.HandleFunc("/ui/components/telephony-panel", ws.handleUITelephony)
	mux.HandleFunc("/ui/components/recordings-table", ws.handleUIRecordings)
	mux.HandleFunc("/ui/components/supervision-panel", ws.handleUISupervision)
	mux.HandleFunc("/ui/components/logs-terminal", ws.handleUILogsTerminal)

	// API Endpoints
	mux.HandleFunc("/api/radio/connect", ws.handleRadioConnect)
	mux.HandleFunc("/api/radio/disconnect", ws.handleRadioDisconnect)
	mux.HandleFunc("/api/radio/ptt", ws.handleRadioPTT)
	mux.HandleFunc("/api/radio/send-voice", ws.handleRadioSendVoice)
	mux.HandleFunc("/api/radio/jitter-buffer", ws.handleRadioJitterBuffer)
	mux.HandleFunc("/api/radio/volume", ws.handleRadioVolume)

	mux.HandleFunc("/api/telephony/dial", ws.handleTelephonyDial)
	mux.HandleFunc("/api/telephony/dial-uri", ws.handleTelephonyDialURI)
	mux.HandleFunc("/api/telephony/hangup", ws.handleTelephonyHangup)

	mux.HandleFunc("/api/recordings/audio", ws.handleRecordingAudio)
	mux.HandleFunc("/api/pcap/upload", ws.handlePCAPUpload)
	mux.HandleFunc("/api/pcap/wav", ws.handlePCAPWAV)
	mux.HandleFunc("/api/logs", ws.handleAPILogs)
	mux.HandleFunc("/api/logs/clear", ws.handleAPILogsClear)
	mux.HandleFunc("/docs/manual", ws.handleManual)

	// Forward live logs from VCSService to WebSocket clients
	logChan, _ := svc.SubscribeLogs()
	go func() {
		for entry := range logChan {
			ws.BroadcastEvent("log", entry)
		}
	}()

	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind VCS Web Console to %s: %w", addr, err)
	}

	actualAddr := ln.Addr().String()
	ws.server = &http.Server{
		Addr:              actualAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("VCS Web Console listening", "addr", fmt.Sprintf("http://%s", actualAddr))
		if err := ws.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("VCS Web Server error", "error", err)
		}
	}()

	return ws, nil
}

// Addr returns the network address that the web server is listening on.
func (w *WebServer) Addr() string {
	if w.server != nil {
		return w.server.Addr
	}
	return ""
}

// Close terminates the web server and closes active WebSockets.
func (w *WebServer) Close() error {
	w.wsMu.Lock()
	for c := range w.wsClients {
		_ = c.Close()
	}
	w.wsClients = make(map[*websocket.Conn]bool)
	w.wsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return w.server.Shutdown(ctx)
}

// BroadcastAudio sends PCM samples to all connected browser clients for Web Audio API playback & FFT.
func (w *WebServer) BroadcastAudio(channelID string, pcmSamples []int16, isRx bool) {
	w.wsMu.Lock()
	defer w.wsMu.Unlock()

	if len(w.wsClients) == 0 {
		return
	}

	buf := new(bytes.Buffer)
	for _, s := range pcmSamples {
		_ = binary.Write(buf, binary.LittleEndian, s)
	}
	data := buf.Bytes()

	for c := range w.wsClients {
		_ = c.WriteMessage(websocket.BinaryMessage, data)
	}
}

// BroadcastEvent sends JSON notifications to attached clients.
func (w *WebServer) BroadcastEvent(eventType string, data any) {
	w.wsMu.Lock()
	defer w.wsMu.Unlock()

	msg, err := json.Marshal(map[string]any{
		"type": eventType,
		"data": data,
	})
	if err != nil {
		return
	}

	for c := range w.wsClients {
		_ = c.WriteMessage(websocket.TextMessage, msg)
	}
}

func (w *WebServer) handleIndex(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write([]byte(IndexHTML))
}

func (w *WebServer) handleStyle(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = rw.Write([]byte(StyleCSS))
}

func (w *WebServer) handleAppJS(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = rw.Write([]byte(AppJS))
}

func (w *WebServer) handleHTMX(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	// Look up htmx.min.js in repository
	paths := []string{
		"internal/web/static/js/htmx.min.js",
		"../internal/web/static/js/htmx.min.js",
		"../../internal/web/static/js/htmx.min.js",
	}
	for _, p := range paths {
		//nolint:gosec // G304: static predefined paths
		if data, err := os.ReadFile(filepath.Clean(p)); err == nil {
			_, _ = rw.Write(data)
			return
		}
	}
	// Fallback CDN redirect
	http.Redirect(rw, r, "https://unpkg.com/htmx.org@1.9.12/dist/htmx.min.js", http.StatusTemporaryRedirect)
}

func (w *WebServer) handleWebSocket(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	w.wsMu.Lock()
	w.wsClients[conn] = true
	w.wsMu.Unlock()

	// Stream initial recent logs to newly connected client
	for _, entry := range w.svc.GetRecentLogs(50) {
		msg, _ := json.Marshal(map[string]any{
			"type": "log",
			"data": entry,
		})
		_ = conn.WriteMessage(websocket.TextMessage, msg)
	}

	defer func() {
		w.wsMu.Lock()
		delete(w.wsClients, conn)
		w.wsMu.Unlock()
		_ = conn.Close()
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// If client sends binary PCM 16-bit 8kHz microphone audio
		if msgType == websocket.BinaryMessage && len(data)%2 == 0 {
			samples := make([]int16, len(data)/2)
			for i := 0; i < len(samples); i++ {
				//nolint:gosec // G115: raw PCM sample conversion
				samples[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
			}
			w.svc.FeedMicAudio(samples)
		}
	}
}

// UI Components
func (w *WebServer) handleUIRadioChannels(rw http.ResponseWriter, r *http.Request) {
	snaps := w.svc.GetChannelSnapshots()
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmplStr := `
	{{range .}}
	<div class="channel-card {{if .PTTActive}}transmitting{{else if .SQUActive}}receiving{{end}}" id="card-{{.ID}}">
		<div class="ch-header">
			<div>
				<div class="ch-freq">{{.Frequency}}</div>
				<div class="ch-name">{{.Name}} ({{.Role}})</div>
			</div>
			<span class="state-badge state-{{.State}}">{{.State}}</span>
		</div>

		<div class="ch-indicators">
			<div class="indicator-light">
				<div class="lamp ptt {{if .PTTActive}}active{{end}}"></div>
				<span>PTT</span>
			</div>
			<div class="indicator-light">
				<div class="lamp squ {{if .SQUActive}}active{{end}}"></div>
				<span>SQU (SQI: {{.SQI}})</span>
			</div>
		</div>

		<div class="stats-box">
			<div>Rx Jitter: <span class="val">{{printf "%.2f" .RxJitterMs}} ms</span></div>
			<div>Tx Timing: <span class="val">{{printf "%.2f" .TxJitterMs}} ms</span></div>
			<div>Packets: <span class="val">Rx {{.RxPackets}} / Tx {{.TxPackets}}</span></div>
			<div>Loss Rate: <span class="val">{{printf "%.1f" .LossRatePct}}%</span></div>
		</div>

		<div class="control-row">
			<label>Jitter Buffer (ms):</label>
			<div class="slider-control">
				<input type="range" min="10" max="120" step="10" value="{{.JitterBufferMs}}"
					onchange="fetch('/api/radio/jitter-buffer?id={{.ID}}&ms=' + this.value, {method:'POST'})">
				<span class="val">{{.JitterBufferMs}}ms</span>
			</div>
		</div>

		<div class="ptt-button-container">
			{{if eq .State "disconnected"}}
				<button class="btn-neon-small" style="width:100%; padding: 0.75rem;" onclick="handleConnectChannel('{{.ID}}')">
					&#128225; Connect to GRS
				</button>
			{{else}}
				<button id="ptt-btn-{{.ID}}" data-channel="{{.ID}}" class="btn-ptt {{if .PTTActive}}active{{end}}"
					onmousedown="handlePTTStart('{{.ID}}', 'normal')"
					onmouseup="handlePTTStop('{{.ID}}')"
					ontouchstart="handlePTTStart('{{.ID}}', 'normal')"
					ontouchend="handlePTTStop('{{.ID}}')">
					PUSH TO TALK (PTT)
				</button>
				<button id="voice-btn-{{.ID}}" class="btn-primary" style="width:100%; margin-top: 0.4rem;" onclick="triggerVoiceTransmission('{{.ID}}')">
					🗣️ Send ATC Voice (Speech TX)
				</button>
				<button class="btn-secondary" style="width:100%; margin-top: 0.4rem;" onclick="fetch('/api/radio/disconnect?id={{.ID}}', {method:'POST'})">
					Disconnect
				</button>
			{{end}}
		</div>
	</div>
	{{end}}
	`
	t, _ := template.New("radio_channels").Parse(tmplStr)
	_ = t.Execute(rw, snaps)
}

func (w *WebServer) handleUITelephony(rw http.ResponseWriter, r *http.Request) {
	phoneSnap := w.svc.GetTelephonySnapshot()
	daList := w.svc.cfg.GetTelephony().DirectAccess
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := map[string]any{
		"Phone": phoneSnap,
		"DAs":   daList,
	}

	tmplStr := `
	<div class="card glass">
		<div class="card-header">
			<h3>Direct Access (Speed Dial)</h3>
		</div>
		<div class="card-body">
			<div class="da-grid">
				{{range .DAs}}
				<div class="da-button" onclick="handleDialDA('{{.ID}}', 'normal')">
					<div class="da-title">{{.Name}}</div>
					<div class="da-uri">{{.TargetSIPURI}}</div>
				</div>
				{{end}}
			</div>
		</div>
	</div>

	<div class="card glass">
		<div class="card-header">
			<h3>Active Telephony Call</h3>
			<span class="state-badge {{if .Phone.Active}}state-connected{{else}}state-disconnected{{end}}">
				{{.Phone.State}}
			</span>
		</div>
		<div class="card-body">
			{{if .Phone.Active}}
				<div style="margin-bottom: 1rem;">
					<div><strong>Target:</strong> {{.Phone.Target}}</div>
					<div><strong>Mode:</strong> <span class="tag-mono">{{.Phone.Mode}}</span></div>
					<div><strong>Duration:</strong> {{printf "%.1f" .Phone.DurationS}}s</div>
				</div>
				<div class="stats-box" style="margin-bottom: 1rem;">
					<div>Rx Jitter: <span class="val">{{printf "%.2f" .Phone.RxJitterMs}} ms</span></div>
					<div>Tx Timing: <span class="val">{{printf "%.2f" .Phone.TxJitterMs}} ms</span></div>
				</div>
				<button class="btn-danger" style="width:100%;" onclick="fetch('/api/telephony/hangup', {method:'POST'})">
					&#128222; Hangup Call
				</button>
			{{else}}
				<p class="muted" style="margin-bottom: 1rem;">No active phone call. Select a Direct Access contact or test mode to start a call.</p>
				<div style="display:flex; gap:0.5rem; flex-wrap:wrap;">
					<button class="btn-primary" onclick="handleDialDA('da-speech-test', 'speech')">
						🗣️ Human Speech Test Call
					</button>
					<button class="btn-secondary" onclick="handleDialDA('da-tone-test', 'tone')">
						1kHz Tone Test Call
					</button>
					<button class="btn-secondary" onclick="handleDialDA('da-echo-test', 'echo')">
						300ms Echo Loopback Test
					</button>
				</div>
			{{end}}
		</div>
	</div>
	`
	t, _ := template.New("telephony").Parse(tmplStr)
	_ = t.Execute(rw, data)
}

func (w *WebServer) handleUIRecordings(rw http.ResponseWriter, r *http.Request) {
	recs := w.recorder.GetRecordings()
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(recs) == 0 {
		_, _ = rw.Write([]byte("<p class='muted'>No audio recordings logged yet. Key PTT or receive audio to record.</p>"))
		return
	}

	tmplStr := `
	<div style="overflow-x: auto;">
		<table class="data-table">
			<thead>
				<tr>
					<th>Timestamp</th>
					<th>Type</th>
					<th>Channel / Target</th>
					<th>Duration</th>
					<th>Size</th>
					<th>Playback</th>
					<th>Action</th>
				</tr>
			</thead>
			<tbody>
				{{range .}}
				<tr>
					<td>{{.Timestamp.Format "2006-01-02 15:04:05"}}</td>
					<td><span class="tag">{{.Type}}</span></td>
					<td>{{.Channel}}</td>
					<td>{{.DurationStr}}</td>
					<td>{{printf "%.1f" (div .SizeBytes 1024)}} KB</td>
					<td>
						<audio controls src="/api/recordings/audio?file={{.FilePath}}" preload="none" style="height: 30px;"></audio>
					</td>
					<td>
						<a href="/api/recordings/audio?file={{.FilePath}}" download class="btn-neon-small">Download</a>
					</td>
				</tr>
				{{end}}
			</tbody>
		</table>
	</div>
	`
	funcMap := template.FuncMap{
		"div": func(a int64, b float64) float64 {
			return float64(a) / b
		},
	}
	t, _ := template.New("recordings").Funcs(funcMap).Parse(tmplStr)
	_ = t.Execute(rw, recs)
}

func (w *WebServer) handleUISupervision(rw http.ResponseWriter, r *http.Request) {
	stats := w.svc.GetSupervisionStatuses()
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmplStr := `
	<div class="da-grid">
		{{range .}}
		<div class="card" style="padding: 1rem; border-left: 4px solid {{if eq .Status "online"}}var(--color-green){{else if eq .Status "degraded"}}var(--color-amber){{else}}var(--color-red){{end}};">
			<div style="font-weight: 700; font-size: 0.95rem; margin-bottom: 0.25rem;">{{.TargetURI}}</div>
			<div style="display:flex; justify-content:space-between; font-size:0.75rem; margin-bottom: 0.5rem;">
				<span>Status: <strong>{{.Status}}</strong></span>
				<span>RTT: <strong>{{printf "%.2f" .RTTMs}} ms</strong></span>
			</div>
			<div class="stats-box">
				<div>Checks: <span class="val">{{.CheckCount}}</span></div>
				<div>Fails: <span class="val">{{.FailCount}}</span></div>
				<div style="grid-column: span 2;">Last: <span class="val">{{.LastCheck.Format "15:04:05"}}</span></div>
			</div>
		</div>
		{{end}}
	</div>
	`
	t, _ := template.New("supervision").Parse(tmplStr)
	_ = t.Execute(rw, stats)
}

// API Handlers
func (w *WebServer) handleRadioConnect(rw http.ResponseWriter, r *http.Request) {
	chID := r.URL.Query().Get("id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := w.svc.ConnectChannel(ctx, chID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRadioDisconnect(rw http.ResponseWriter, r *http.Request) {
	chID := r.URL.Query().Get("id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := w.svc.DisconnectChannel(ctx, chID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRadioPTT(rw http.ResponseWriter, r *http.Request) {
	chID := r.URL.Query().Get("id")
	action := r.URL.Query().Get("action")
	pttTypeStr := r.URL.Query().Get("type")

	var pttType ed137.PTTType
	switch pttTypeStr {
	case "priority":
		pttType = ed137.PTTPriority
	case "emergency":
		pttType = ed137.PTTEmergency
	default:
		pttType = ed137.PTTNormal
	}

	if action == "start" {
		if err := w.svc.StartPTT(chID, pttType, 1); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := w.svc.StopPTT(chID); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRadioSendVoice(rw http.ResponseWriter, r *http.Request) {
	chID := r.URL.Query().Get("id")
	if err := w.svc.SendVoiceTransmission(chID); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRadioJitterBuffer(rw http.ResponseWriter, r *http.Request) {
	chID := r.URL.Query().Get("id")
	msStr := r.URL.Query().Get("ms")
	ms, _ := strconv.Atoi(msStr)
	if ms >= 10 && ms <= 200 {
		_ = w.svc.SetChannelJitterBuffer(chID, ms)
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRadioVolume(rw http.ResponseWriter, r *http.Request) {
	// Handled client-side Web Audio or state
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleTelephonyDial(rw http.ResponseWriter, r *http.Request) {
	daID := r.URL.Query().Get("da")
	mode := r.URL.Query().Get("mode")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := w.svc.DialDA(ctx, daID, mode); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleTelephonyDialURI(rw http.ResponseWriter, r *http.Request) {
	targetURI := r.URL.Query().Get("uri")
	mode := r.URL.Query().Get("mode")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := w.svc.DialURI(ctx, targetURI, mode); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleTelephonyHangup(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	_ = w.svc.HangupPhone(ctx)
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleRecordingAudio(rw http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	cleanPath := filepath.Clean(filePath)

	//nolint:gosec // G304: user-requested audio recording file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		http.Error(rw, "file not found", http.StatusNotFound)
		return
	}

	rw.Header().Set("Content-Type", "audio/wav")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.Header().Set("Content-Length", strconv.Itoa(len(data)))
	//nolint:gosec // G705: raw binary WAV data
	_, _ = rw.Write(data)
}

func (w *WebServer) handlePCAPUpload(rw http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(rw, r.Body, 32<<20)
	//nolint:gosec // G120: request body bounded by http.MaxBytesReader
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("pcap")
	if err != nil {
		http.Error(rw, "missing pcap file", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	report, err := pcap.AnalyzePCAP(file, header.Filename)
	if err != nil {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.lastPCAPMu.Lock()
	w.lastPCAPWAV = report.RestoredWAVData
	w.lastPCAPMu.Unlock()

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(report)
}

func (w *WebServer) handlePCAPWAV(rw http.ResponseWriter, r *http.Request) {
	w.lastPCAPMu.RLock()
	data := w.lastPCAPWAV
	w.lastPCAPMu.RUnlock()

	if len(data) == 0 {
		http.Error(rw, "no restored WAV audio in current PCAP", http.StatusNotFound)
		return
	}

	rw.Header().Set("Content-Type", "audio/wav")
	rw.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = rw.Write(data)
}

func (w *WebServer) handleManual(rw http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("docs/manual.md")
	if err != nil {
		http.Error(rw, "Manual file not found", http.StatusNotFound)
		return
	}
	rw.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = rw.Write(data)
}

func (w *WebServer) handleAPILogs(rw http.ResponseWriter, r *http.Request) {
	logs := w.svc.GetRecentLogs(200)
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(logs)
}

func (w *WebServer) handleAPILogsClear(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.svc.ClearLogs()
	rw.WriteHeader(http.StatusOK)
}

func (w *WebServer) handleUILogsTerminal(rw http.ResponseWriter, r *http.Request) {
	logs := w.svc.GetRecentLogs(100)
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmplStr := `
	{{range .}}
	<div class="log-row log-proto-{{.Protocol}} log-level-{{.Level}}">
		<span class="log-time">[{{.Timestamp}}]</span>
		<span class="log-badge badge-{{.Protocol}}">{{.Protocol}}</span>
		<span class="log-dir dir-{{.Direction}}">[{{.Direction}}]</span>
		<span class="log-msg">{{.Message}}</span>
	</div>
	{{else}}
	<div class="muted" style="padding: 1rem;">No communication logs recorded yet.</div>
	{{end}}
	`
	tmpl, err := template.New("logs").Parse(tmplStr)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(rw, logs)
}
