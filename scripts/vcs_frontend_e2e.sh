#!/usr/bin/env bash
# Copyright 2026 [Copyright Holder]
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Author: [YOUR_NAME]

set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}   Aerovoice VCS HTMX Frontend E2E Test Suite                   ${NC}"
echo -e "${CYAN}================================================================${NC}"

WORK_DIR=$(mktemp -d -t aerovoice-vcs-e2e-XXXXXX)
RECORDINGS_DIR="$WORK_DIR/recordings"
mkdir -p "$RECORDINGS_DIR"

REPORT_DIR="$(pwd)/test_reports"
mkdir -p "$REPORT_DIR"
mkdir -p "docs/images"

# Isolated ports for standalone E2E execution
VCS_SIP_PORT=25060
VCS_RTP_PORT=25100
VCS_WEB_PORT=28082

GRS_SIP_PORT=25070
GRS_RTP_PORT=25200
GRS_WEB_PORT=28081

VCS_PID=""
GRS_PID=""

cleanup() {
    echo -e "\n${YELLOW}===> [VCS Frontend E2E Cleanup] Stopping background servers...${NC}"
    if [ -n "$VCS_PID" ] && kill -0 "$VCS_PID" > /dev/null 2>&1; then
        kill "$VCS_PID" > /dev/null 2>&1 || true
    fi
    if [ -n "$GRS_PID" ] && kill -0 "$GRS_PID" > /dev/null 2>&1; then
        kill "$GRS_PID" > /dev/null 2>&1 || true
    fi
    rm -rf "$WORK_DIR"
    echo -e "${GREEN}===> [VCS Frontend E2E Cleanup] Completed.${NC}"
}
trap cleanup EXIT INT TERM

# 1. Build Binaries
echo -e "\n${YELLOW}[Step 1/5] Building VCS Console & GRS Emulator binaries...${NC}"
go build -o "$WORK_DIR/vcs" ./cmd/vcs
go build -o "$WORK_DIR/grs-emulator" ./cmd/grs-emulator
echo -e "${GREEN}Binaries successfully compiled.${NC}"

# 2. Generate isolated temporary configs
cat <<EOF > "$WORK_DIR/grs.yaml"
grs:
  station_name: "E2E-GRS-Station"
  frequency: "118.100 MHz"
  sip_host: "127.0.0.1"
  sip_port: ${GRS_SIP_PORT}
  rtp_host: "127.0.0.1"
  rtp_port: ${GRS_RTP_PORT}
  web_host: "127.0.0.1"
  web_port: ${GRS_WEB_PORT}
  default_ptime: 10
  loopback_echo: true
  audio_source: "pilot_voice"
  telephone:
    auto_answer: true
    auto_answer_mode: "speech"
EOF

cat <<EOF > "$WORK_DIR/vcs.yaml"
vcs:
  sip_host: "127.0.0.1"
  sip_port: ${VCS_SIP_PORT}
  rtp_host: "127.0.0.1"
  rtp_port_start: ${VCS_RTP_PORT}
  web_host: "127.0.0.1"
  web_port: ${VCS_WEB_PORT}
  default_ptime: 10
  default_jitter_buffer_ms: 40

channels:
  - id: "ch-twr"
    name: "TWR Main"
    frequency: "118.100 MHz"
    grs_sip_uri: "sip:radio@127.0.0.1:${GRS_SIP_PORT}"
    role: "Main"
    ptime: 10
    jitter_buffer_ms: 40

telephony:
  direct_access:
    - id: "da-twr"
      name: "TWR Tower"
      target_sip_uri: "sip:101@127.0.0.1:${GRS_SIP_PORT}"
    - id: "da-speech-test"
      name: "Speech Test Call"
      target_sip_uri: "sip:speech-test@127.0.0.1:${GRS_SIP_PORT}"
EOF

# 3. Start GRS Emulator
echo -e "\n${YELLOW}[Step 2/5] Starting GRS Emulator on Web port ${GRS_WEB_PORT}...${NC}"
"$WORK_DIR/grs-emulator" -config "$WORK_DIR/grs.yaml" > "$WORK_DIR/grs.log" 2>&1 &
GRS_PID=$!

for i in {1..20}; do
    if curl -sf "http://127.0.0.1:${GRS_WEB_PORT}/api/snapshot" > /dev/null 2>&1; then
        echo -e "${GREEN}GRS Emulator is ONLINE!${NC}"
        break
    fi
    if [ "$i" -eq 20 ]; then
        echo -e "${RED}GRS Emulator failed to start in time. Logs:${NC}"
        cat "$WORK_DIR/grs.log"
        exit 1
    fi
    sleep 0.5
done

# 4. Start VCS Console
echo -e "\n${YELLOW}[Step 3/5] Starting VCS Console on Web port ${VCS_WEB_PORT}...${NC}"
"$WORK_DIR/vcs" -config "$WORK_DIR/vcs.yaml" > "$WORK_DIR/vcs.log" 2>&1 &
VCS_PID=$!

for i in {1..20}; do
    if curl -sf "http://127.0.0.1:${VCS_WEB_PORT}/" > /dev/null 2>&1; then
        echo -e "${GREEN}VCS Console is ONLINE!${NC}"
        break
    fi
    if [ "$i" -eq 20 ]; then
        echo -e "${RED}VCS Console failed to start in time. Logs:${NC}"
        cat "$WORK_DIR/vcs.log"
        exit 1
    fi
    sleep 0.5
done

# Trigger channel connection & quick voice transmission to populate HTMX state
echo -e "\n${YELLOW}[Step 4/5] Pre-populating HTMX components (Radio connect & test transmission)...${NC}"
curl -sf -X POST "http://127.0.0.1:${VCS_WEB_PORT}/api/radio/connect?id=ch-twr" > /dev/null 2>&1 || true
sleep 0.5
curl -sf -X POST "http://127.0.0.1:${VCS_WEB_PORT}/api/radio/send-voice?id=ch-twr" > /dev/null 2>&1 || true
sleep 1.0

# 5. Execute Python HTMX & Headless Chrome Verification
echo -e "\n${YELLOW}[Step 5/5] Running Headless Chrome E2E Verification & Visual Report Suite...${NC}"
VCS_URL="http://127.0.0.1:${VCS_WEB_PORT}" GRS_URL="http://127.0.0.1:${GRS_WEB_PORT}" \
    python3 scripts/test_vcs_frontend_ui.py

echo -e "\n${GREEN}========================================================================${NC}"
echo -e "${GREEN} ✅ ALL VCS HTMX FRONTEND E2E TESTS PASSED SUCCESSFULLY!                ${NC}"
echo -e "${GREEN}    - Main Console & HTMX tabs: 100% Validated (English & Japanese)     ${NC}"
echo -e "${GREEN}    - Radio Channels component: Frequency, PTT/SQU lamps, controls OK  ${NC}"
echo -e "${GREEN}    - Telephony subsystem: Direct Access, test buttons validated        ${NC}"
echo -e "${GREEN}    - Node Supervision: Live RTT & SIP health monitoring OK             ${NC}"
echo -e "${GREEN}    - Audio Recordings: ED-137 Volume 4 WAV playback table verified    ${NC}"
echo -e "${GREEN}    - Signaling Terminal: SIP & ED-137 log row rendering OK             ${NC}"
echo -e "${GREEN}    - Headless Chrome Snapshots: Multi-screen snapshots captured        ${NC}"
echo -e "${GREEN}    - English Report: test_reports/vcs_frontend_e2e_report_en.html      ${NC}"
echo -e "${GREEN}    - Japanese Report: test_reports/vcs_frontend_e2e_report_ja.html     ${NC}"
echo -e "${GREEN}    - GitHub Pages Portal: test_reports/index.html                      ${NC}"
echo -e "${GREEN}========================================================================${NC}"
