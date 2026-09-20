#!/bin/bash
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

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "================================================================="
echo "  Aerovoice ED-137C Radio & Telephony Interactive Demo Suite"
echo "================================================================="
echo ""
echo "Starting components in background:"
echo "  1. GRS Ground Radio Station Testbench -> http://127.0.0.1:8081"
echo "  2. VCS Air Traffic Control Console    -> http://127.0.0.1:8082"
echo ""

# Build binaries if not exist
cd "${ROOT_DIR}"
if [ ! -f "bin/grs-emulator" ] || [ ! -f "bin/vcs" ]; then
    echo "==> Building bin/grs-emulator and bin/vcs..."
    go build -o bin/grs-emulator ./cmd/grs-emulator
    go build -o bin/vcs ./cmd/vcs
fi

# Cleanup function
cleanup() {
    echo ""
    echo "==> Shutting down demo servers..."
    if [ -n "${GRS_PID}" ]; then
        kill "${GRS_PID}" 2>/dev/null || true
    fi
    if [ -n "${VCS_PID}" ]; then
        kill "${VCS_PID}" 2>/dev/null || true
    fi
    echo "Demo servers stopped."
    exit 0
}

trap cleanup SIGINT SIGTERM EXIT

# Start GRS Emulator
"${ROOT_DIR}/bin/grs-emulator" -config "${ROOT_DIR}/configs/grs.yaml" &
GRS_PID=$!

sleep 1

# Start VCS Console
"${ROOT_DIR}/bin/vcs" -config "${ROOT_DIR}/configs/vcs.yaml" &
VCS_PID=$!

echo ""
echo "================================================================="
echo "  Servers are now RUNNING!"
echo "  - Open VCS Console in Browser:   http://127.0.0.1:8082"
echo "  - Open GRS Testbench in Browser: http://127.0.0.1:8081"
echo ""
echo "  Quick Test Guide:"
echo "  1. In VCS (8082), click 'Connect to GRS' on 'TWR Main'."
echo "  2. Hold the 'PUSH TO TALK (PTT)' button or press SPACEBAR."
echo "     -> Observe GRS (8081) VU Meter and PTT ON event log."
echo "  3. In GRS (8081), toggle 'Squelch Downlink' to ON."
echo "     -> Observe VCS (8082) SQU indicator light up and audio FFT peak."
echo "  4. In VCS (8082) 'Telephony' tab, click '1kHz Tone Test Call'."
echo "     -> Observe full-duplex call, jitter metrics, and WAV log creation."
echo "================================================================="
echo "Press Ctrl+C to terminate both servers."

wait
