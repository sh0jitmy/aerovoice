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
	"syscall"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/grs"
	"github.com/shjtmy/go_sh0jitmy_template/internal/version"
)

func main() {
	configPath := flag.String("config", "configs/grs.yaml", "Path to GRS configuration YAML")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Aerovoice GRS Emulator Version %s (Commit: %s, Built: %s)\n",
			version.Version, version.Commit, version.Date)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Aerovoice GRS Emulator (ED-137C Radio Testbench)", "version", version.Version)

	cfg, err := config.LoadGRSConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load GRS configuration", "path", *configPath, "error", err)
		os.Exit(1)
	}

	svc, err := grs.NewService(cfg)
	if err != nil {
		slog.Error("Failed to initialize GRS service", "error", err)
		os.Exit(1)
	}
	defer func() { _ = svc.Close() }()

	webSvr, err := grs.NewWebServer(svc, cfg.GRS.WebHost, cfg.GRS.WebPort)
	if err != nil {
		slog.Error("Failed to start GRS Web Console", "error", err)
		os.Exit(1)
	}
	defer func() { _ = webSvr.Close() }()

	if err := webSvr.Start(); err != nil {
		slog.Error("Failed to launch GRS Web server", "error", err)
		os.Exit(1)
	}

	slog.Info("GRS Emulator operational",
		"station", cfg.GRS.StationName,
		"frequency", cfg.GRS.Frequency,
		"web_url", fmt.Sprintf("http://%s:%d", cfg.GRS.WebHost, cfg.GRS.WebPort),
		"sip_port", cfg.GRS.SIPPort,
		"rtp_port", cfg.GRS.RTPPort,
	)

	// Wait for OS shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutting down Aerovoice GRS Emulator...")
}
