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

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/shjtmy/aerovoice/internal/config"
	"github.com/shjtmy/aerovoice/internal/media"
	"github.com/shjtmy/aerovoice/internal/vcs"
	"github.com/shjtmy/aerovoice/internal/version"
)

func main() {
	configPath := flag.String("config", "configs/vcs.yaml", "Path to VCS configuration YAML")
	recDir := flag.String("recordings", "data/recordings", "Directory to store audio recordings")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Aerovoice VCS Console Version %s (Commit: %s, Built: %s)\n",
			version.Version, version.Commit, version.Date)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Aerovoice VCS Console (ED-137C Radio Verification)", "version", version.Version)

	cfg, err := config.LoadVCSConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load VCS configuration", "path", *configPath, "error", err)
		os.Exit(1)
	}

	cleanRecDir := filepath.Clean(*recDir)
	recorder, err := media.NewRecorderWithConfig(media.RecorderConfig{
		OutputDir:          cleanRecDir,
		MaxRecordings:      cfg.Recording.MaxRecordings,
		MaxDurationSeconds: cfg.Recording.MaxDurationSeconds,
	})
	if err != nil {
		slog.Error("Failed to initialize audio recorder", "path", cleanRecDir, "error", err)
		os.Exit(1)
	}

	svc, err := vcs.NewVCSService(cfg, recorder)
	if err != nil {
		slog.Error("Failed to initialize VCS service", "error", err)
		os.Exit(1)
	}
	defer func() { _ = svc.Close() }()

	webCore := cfg.GetVCS()
	webSvr, err := vcs.NewWebServer(svc, recorder, webCore.WebHost, webCore.WebPort)
	if err != nil {
		slog.Error("Failed to start VCS Web Console", "error", err)
		os.Exit(1)
	}
	defer func() { _ = webSvr.Close() }()

	slog.Info("VCS System fully operational",
		"web_url", fmt.Sprintf("http://%s:%d", webCore.WebHost, webCore.WebPort),
		"sip_port", webCore.SIPPort,
		"rtp_port_start", webCore.RTPPortStart,
	)

	// Wait for OS shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutting down Aerovoice VCS...")
}
