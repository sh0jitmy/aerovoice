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

package sip

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// ActiveCall represents an established SIP dialog.
type ActiveCall struct {
	CallID    string
	TargetURI string
	Dialog    *sipgo.DialogClientSession
	node      *SIPNode
}

// Hangup terminates the call with a BYE request.
func (c *ActiveCall) Hangup(ctx context.Context) error {
	if c.Dialog != nil {
		return c.Dialog.Bye(ctx)
	}
	return nil
}

// SIPNodeConfig holds configuration for a local SIP endpoint.
type SIPNodeConfig struct {
	Host             string
	Port             int
	UserAgentName    string
	OnInvite         func(caller string, callID string, sdpOffer []byte) ([]byte, error)
	OnBye            func(callID string)
	OnOptions        func(caller string) bool // true to respond 200 OK, false for silent drop
}

// SIPNode encapsulates SIP UAS server and UAC client.
type SIPNode struct {
	cfg        SIPNodeConfig
	ua         *sipgo.UserAgent
	client     *sipgo.Client
	server     *sipgo.Server
	contactHdr sip.ContactHeader
	dialogs    *sipgo.DialogClientCache

	mu              sync.RWMutex
	silentDrop      bool // For fault simulation
	optionsCount    uint64
	lastOptionsTime time.Time
	activeCalls     map[string]*ActiveCall

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSIPNode initializes a SIPNode on the specified host and port.
func NewSIPNode(cfg SIPNodeConfig) (*SIPNode, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.UserAgentName == "" {
		cfg.UserAgentName = "Aerovoice-ED137/1.0"
	}

	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent(cfg.UserAgentName),
		sipgo.WithUserAgentHostname(cfg.Host),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP UserAgent: %w", err)
	}

	client, err := sipgo.NewClient(ua)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP client: %w", err)
	}

	server, err := sipgo.NewServer(ua)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP server: %w", err)
	}

	contactHdr := sip.ContactHeader{
		Address: sip.Uri{
			User: "radio",
			Host: cfg.Host,
			Port: cfg.Port,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	node := &SIPNode{
		cfg:         cfg,
		ua:          ua,
		client:      client,
		server:      server,
		contactHdr:  contactHdr,
		dialogs:     sipgo.NewDialogClientCache(client, contactHdr),
		activeCalls: make(map[string]*ActiveCall),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Register server request handlers
	server.OnInvite(node.handleInvite)
	server.OnBye(node.handleBye)
	server.OnAck(node.handleAck)
	server.OnOptions(node.handleOptions)

	// Start listening UDP
	listenAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	go func() {
		if err := server.ListenAndServe(ctx, "udp", listenAddr); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Debug("SIP server stopped", "addr", listenAddr, "error", err)
		}
	}()

	return node, nil
}

// Call sends SIP INVITE with SDP offer, awaits 200 OK + SDP answer, and returns ActiveCall.
func (n *SIPNode) Call(ctx context.Context, targetURIStr string, sdpOffer []byte) (*ActiveCall, []byte, error) {
	var targetURI sip.Uri
	if err := sip.ParseUri(targetURIStr, &targetURI); err != nil {
		return nil, nil, fmt.Errorf("invalid target URI %s: %w", targetURIStr, err)
	}

	dialog, err := n.dialogs.Invite(ctx, targetURI, sdpOffer)
	if err != nil {
		return nil, nil, fmt.Errorf("SIP INVITE failed to %s: %w", targetURIStr, err)
	}

	// Wait for 200 OK answer
	if err := dialog.WaitAnswer(ctx, sipgo.AnswerOptions{}); err != nil {
		return nil, nil, fmt.Errorf("failed to receive 200 OK: %w", err)
	}

	// Send ACK to establish dialog
	if err := dialog.Ack(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to send ACK: %w", err)
	}

	var sdpAnswer []byte
	if dialog.InviteResponse != nil {
		sdpAnswer = dialog.InviteResponse.Body()
	}
	callID := dialog.ID

	activeCall := &ActiveCall{
		CallID:    callID,
		TargetURI: targetURIStr,
		Dialog:    dialog,
		node:      n,
	}

	n.mu.Lock()
	n.activeCalls[callID] = activeCall
	n.mu.Unlock()

	return activeCall, sdpAnswer, nil
}

// Ping sends SIP OPTIONS to targetURI and measures round-trip time (RTT).
func (n *SIPNode) Ping(ctx context.Context, targetURIStr string) (rtt time.Duration, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("OPTIONS panic recovered: %v", r)
		}
	}()

	var targetURI sip.Uri
	if err := sip.ParseUri(targetURIStr, &targetURI); err != nil {
		return 0, fmt.Errorf("invalid ping URI: %w", err)
	}

	req := sip.NewRequest(sip.OPTIONS, targetURI)
	req.AppendHeader(sip.NewHeader("Contact", n.contactHdr.Value()))

	start := time.Now()
	tx, err := n.client.TransactionRequest(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("failed to send OPTIONS: %w", err)
	}

	select {
	case res := <-tx.Responses():
		if res != nil && res.StatusCode == sip.StatusOK {
			return time.Since(start), nil
		}
		if res != nil {
			return 0, fmt.Errorf("OPTIONS responded with %d %s", res.StatusCode, res.Reason)
		}
		return 0, errors.New("OPTIONS response was nil")
	case <-tx.Done():
		return 0, errors.New("OPTIONS transaction done")
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// SetSilentDrop sets simulation mode to ignore OPTIONS requests (Supervision fault test).
func (n *SIPNode) SetSilentDrop(drop bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.silentDrop = drop
}

// GetOptionsStats returns count and time of last answered OPTIONS.
func (n *SIPNode) GetOptionsStats() (uint64, time.Time) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.optionsCount, n.lastOptionsTime
}

// Close gracefully stops the SIP node.
func (n *SIPNode) Close() error {
	n.cancel()
	_ = n.server.Close()
	_ = n.client.Close()
	return nil
}

func (n *SIPNode) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	caller := req.From().Address.String()
	sdpOffer := req.Body()

	var sdpAnswer []byte
	var err error

	if n.cfg.OnInvite != nil {
		sdpAnswer, err = n.cfg.OnInvite(caller, callID, sdpOffer)
	}

	if err != nil {
		res := sip.NewResponseFromRequest(req, sip.StatusServiceUnavailable, "Service Unavailable", nil)
		_ = tx.Respond(res)
		return
	}

	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", sdpAnswer)
	res.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	res.AppendHeader(sip.NewHeader("Contact", n.contactHdr.Value()))
	_ = tx.Respond(res)
}

func (n *SIPNode) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	if n.cfg.OnBye != nil {
		n.cfg.OnBye(callID)
	}

	n.mu.Lock()
	delete(n.activeCalls, callID)
	n.mu.Unlock()

	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	_ = tx.Respond(res)
}

func (n *SIPNode) handleAck(req *sip.Request, tx sip.ServerTransaction) {
	// ACK received; dialog is now confirmed
}

func (n *SIPNode) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	n.mu.Lock()
	silent := n.silentDrop
	n.optionsCount++
	n.lastOptionsTime = time.Now()
	n.mu.Unlock()

	if silent {
		return // Drop silently to simulate network timeout / crash
	}

	if n.cfg.OnOptions != nil && !n.cfg.OnOptions(req.From().Address.String()) {
		return
	}

	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	res.AppendHeader(sip.NewHeader("Contact", n.contactHdr.Value()))
	_ = tx.Respond(res)
}
