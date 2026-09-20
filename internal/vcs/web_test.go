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
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebServer_Endpoints(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	recDir := filepath.Join(tempDir, "recordings")

	recorder, err := media.NewRecorder(recDir)
	require.NoError(t, err)

	cfg := &config.VCSConfig{
		VCS: config.VCSCoreConfig{
			SIPHost:               "127.0.0.1",
			SIPPort:               17060,
			RTPHost:               "127.0.0.1",
			RTPPortStart:          27000,
			DefaultPtime:          10,
			DefaultJitterBufferMs: 40,
		},
		Channels: []config.ChannelConfig{
			{
				ID:             "ch-web-test",
				Name:           "Tower Web",
				Frequency:      "118.100 MHz",
				GRSSIPURI:      "sip:radio@127.0.0.1:17070",
				Role:           "Main",
				Ptime:          10,
				JitterBufferMs: 40,
			},
		},
	}

	svc, err := NewVCSService(cfg, recorder)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	webSvr, err := NewWebServer(svc, recorder, "127.0.0.1", 18080)
	require.NoError(t, err)
	defer func() { _ = webSvr.Close() }()

	// 1. Test Index HTML
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Aerovoice")
	assert.Contains(t, w.Body.String(), "ED-137C")

	// 2. Test CSS
	req = httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "--color-cyan")

	// 3. Test App JS
	req = httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "AudioContext")

	// 4. Test UI Components
	req = httptest.NewRequest(http.MethodGet, "/ui/components/radio-channels", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ch-web-test")
	assert.Contains(t, w.Body.String(), "118.100 MHz")

	req = httptest.NewRequest(http.MethodGet, "/ui/components/telephony-panel", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/ui/components/recordings-table", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/ui/components/supervision-panel", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 5. Test Jitter Buffer API
	req = httptest.NewRequest(http.MethodPost, "/api/radio/jitter-buffer?id=ch-web-test&ms=60", nil)
	w = httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	snaps := svc.GetChannelSnapshots()
	assert.Equal(t, 60, snaps[0].JitterBufferMs)
}
