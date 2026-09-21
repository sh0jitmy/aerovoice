#!/usr/bin/env python3
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
"""
Aerovoice UI Snapshot Capture Tool
Captures clean, high-resolution snapshots of all VCS tabs and GRS console
for documentation and manuals.
"""

import os
import shutil
import subprocess
import sys
import time
import urllib.request


def find_chrome():
    candidates = [
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        shutil.which("google-chrome"),
        shutil.which("chromium"),
        shutil.which("chromium-browser"),
    ]
    for c in candidates:
        if c and os.path.exists(c):
            return c
    return None


def main():
    chrome = find_chrome()
    if not chrome:
        print("Error: Chrome binary not found.")
        sys.exit(1)

    vcs_port = int(os.environ.get("VCS_PORT", 28082))
    grs_port = int(os.environ.get("GRS_PORT", 28081))
    vcs_url = f"http://127.0.0.1:{vcs_port}"
    grs_url = f"http://127.0.0.1:{grs_port}"

    out_dir = os.path.join("docs", "images")
    os.makedirs(out_dir, exist_ok=True)

    targets = [
        # VCS Tabs
        ("VCS Main Dashboard", f"{vcs_url}/", os.path.join(out_dir, "vcs_htmx_dashboard.png")),
        ("VCS Radio Console", f"{vcs_url}/?tab=radio", os.path.join(out_dir, "vcs_tab_radio.png")),
        ("VCS Telephony DA", f"{vcs_url}/?tab=telephony", os.path.join(out_dir, "vcs_tab_telephony.png")),
        ("VCS Recordings", f"{vcs_url}/?tab=recordings", os.path.join(out_dir, "vcs_tab_recordings.png")),
        ("VCS Supervision", f"{vcs_url}/?tab=supervision", os.path.join(out_dir, "vcs_tab_supervision.png")),
        ("VCS PCAP Analyzer", f"{vcs_url}/?tab=pcap", os.path.join(out_dir, "vcs_tab_pcap.png")),
        ("VCS Comm Logs", f"{vcs_url}/?tab=logs", os.path.join(out_dir, "vcs_tab_logs.png")),
        # GRS Tabs
        ("GRS Main Dashboard", f"{grs_url}/", os.path.join(out_dir, "grs_dashboard.png")),
        ("GRS Radio Transceiver", f"{grs_url}/?tab=radio", os.path.join(out_dir, "grs_tab_radio.png")),
        ("GRS Telephony Test", f"{grs_url}/?tab=telephony", os.path.join(out_dir, "grs_tab_telephony.png")),
        ("GRS Supervision & Faults", f"{grs_url}/?tab=supervision", os.path.join(out_dir, "grs_tab_supervision.png")),
        ("GRS Protocol Logs", f"{grs_url}/?tab=logs", os.path.join(out_dir, "grs_tab_logs.png")),
    ]

    print("Capturing UI snapshots with Headless Chrome...")
    for label, url, dest in targets:
        cmd = [
            chrome,
            "--headless=new",
            "--disable-gpu",
            "--no-sandbox",
            "--hide-scrollbars",
            "--window-size=1440,900",
            f"--screenshot={dest}",
            url,
        ]
        try:
            subprocess.run(cmd, check=True, timeout=15, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            size_kb = os.path.getsize(dest) // 1024
            print(f"  ✅ {label:20} -> {dest} ({size_kb} KB)")
        except Exception as e:
            print(f"  ❌ {label:20} -> Failed: {e}")

    print("Snapshot capture completed.")


if __name__ == "__main__":
    main()
