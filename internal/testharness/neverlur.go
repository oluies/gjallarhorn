// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/oluies/neverlur/config"
	ncoordinator "github.com/oluies/neverlur/coordinator"
	"github.com/oluies/neverlur/edtls"
	nlog "github.com/oluies/neverlur/log"
	"github.com/oluies/neverlur/mock"
	"github.com/oluies/neverlur/pkg"
)

// startNeverlurSide stands up the full Neverlur-side service set
// (PKG, CDN, Mixchain, AddFriend + Dialing coordinators) and
// populates the corresponding Harness fields. Pattern follows
// neverlur/alpenhorn_test.go createAlpenhornUniverse which is the
// canonical in-process bringup for this side.
//
// The harness's per-component cleanups slice is appended to so
// Close() can tear everything down in reverse order.
func (h *Harness) startNeverlurSide(tmpDir string) error {
	// === Config server (HTTP server + config.Client we hand to coordinators) ===
	cfgServer, err := config.CreateServer(filepath.Join(tmpDir, "config-server-state"))
	if err != nil {
		return wrap("config.CreateServer", err)
	}
	// Plain net.Listen (no TLS) so config.Client (which uses
	// http://) can talk to it. Production deploys this behind a
	// reverse proxy / TLS terminator; in-process tests don't need
	// the auth layer.
	cfgListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return wrap("config-server net.Listen", err)
	}
	cfgHTTP := &http.Server{Handler: cfgServer}
	go func() {
		_ = cfgHTTP.Serve(cfgListener)
	}()
	h.cleanups = append(h.cleanups, func() { _ = cfgHTTP.Close() })

	cfgClient := &config.Client{
		ConfigServerURL: "http://" + cfgListener.Addr().String(),
	}
	// Stash the *config.Server so startGjallarhornSide can publish
	// the Convo config through the same in-memory server.
	h.neverlurCfgServer = cfgServer

	// === Coordinator key (shared between AddFriend + Dialing on Neverlur side) ===
	coordPub, coordPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return wrap("coordinator key", err)
	}
	coordListener, err := edtls.Listen("tcp", "localhost:0", coordPriv)
	if err != nil {
		return wrap("coordinator edtls.Listen", err)
	}
	coordAddr := coordListener.Addr().String()

	// === CDN ===
	cdnInst := mock.LaunchCDN(tmpDir, coordPub)
	h.NeverlurCDN = NeverlurCDNHandle{CDN: cdnInst}
	h.cleanups = append(h.cleanups, func() { _ = cdnInst.HTTPServer.Close() })

	// === Mixchain (3 mixers wired for AddFriend + Dialing) ===
	mixchain := mock.LaunchMixchain(h.opts.neverlurMixers, coordPub)
	h.NeverlurMixers = []NeverlurMixchainHandle{{Mixchain: mixchain}}
	h.cleanups = append(h.cleanups, func() { _ = mixchain.Close() })

	// === PKG (single instance for the harness; production has multiple) ===
	pkgInst, err := mock.LaunchPKG(coordPub, func(_, _ string) error { return nil })
	if err != nil {
		return wrap("mock.LaunchPKG", err)
	}
	h.NeverlurPKG = NeverlurPKGHandle{PKG: pkgInst}
	h.cleanups = append(h.cleanups, func() { _ = pkgInst.Close() })

	// === Sign the AddFriend SignedConfig with real component endpoints ===
	addFriendConf := &config.SignedConfig{
		Version:          config.SignedConfigVersion,
		MinClientVersion: config.SignedConfigVersion,
		Created:          time.Now().UTC().Round(0),
		Expires:          time.Now().UTC().Add(1 * time.Hour).Round(0),
		Service:          "AddFriend",
		Inner: &config.AddFriendConfig{
			Version: config.AddFriendConfigVersion,
			Coordinator: config.CoordinatorConfig{
				Key:     coordPub,
				Address: coordAddr,
			},
			PKGServers: []pkg.PublicServerConfig{pkgInst.PublicServerConfig},
			MixServers: mixchain.Servers,
			CDNServer: config.CDNServerConfig{
				Key:     cdnInst.PublicKey,
				Address: cdnInst.Addr,
			},
			Registrar: config.RegistrarConfig{
				Key:     coordPub,
				Address: coordAddr,
			},
		},
	}
	if err := h.signConfigInPlace(addFriendConf); err != nil {
		return wrap("sign AddFriend config", err)
	}
	if err := cfgServer.SetCurrentConfig(addFriendConf); err != nil {
		return wrap("set AddFriend config", err)
	}
	h.AddFriendConfig = addFriendConf

	// === Sign the Dialing SignedConfig ===
	dialingConf := &config.SignedConfig{
		Version:          config.SignedConfigVersion,
		MinClientVersion: config.SignedConfigVersion,
		Created:          time.Now().UTC().Round(0),
		Expires:          time.Now().UTC().Add(1 * time.Hour).Round(0),
		Service:          "Dialing",
		Inner: &config.DialingConfig{
			Version: config.DialingConfigVersion,
			Coordinator: config.CoordinatorConfig{
				Key:     coordPub,
				Address: coordAddr,
			},
			MixServers: mixchain.Servers,
			CDNServer: config.CDNServerConfig{
				Key:     cdnInst.PublicKey,
				Address: cdnInst.Addr,
			},
		},
	}
	if err := h.signConfigInPlace(dialingConf); err != nil {
		return wrap("sign Dialing config", err)
	}
	if err := cfgServer.SetCurrentConfig(dialingConf); err != nil {
		return wrap("set Dialing config", err)
	}
	h.DialingConfig = dialingConf

	// === AddFriend coordinator ===
	addFriendSrv := &ncoordinator.Server{
		Service:      "AddFriend",
		PrivateKey:   coordPriv,
		Log:          quietLogger(),
		ConfigClient: cfgClient,
		PKGWait:      1 * time.Second,
		MixWait:      1 * time.Second,
		RoundWait:    2 * time.Second,
		NumMailboxes: 1,
		PersistPath:  filepath.Join(tmpDir, "addfriend-coordinator-state"),
	}
	if err := addFriendSrv.Persist(); err != nil {
		return wrap("addFriend Persist", err)
	}
	if err := addFriendSrv.LoadPersistedState(); err != nil {
		return wrap("addFriend LoadPersistedState", err)
	}
	if err := addFriendSrv.Run(); err != nil {
		return wrap("addFriend Run", err)
	}
	h.NeverlurCoordinator = addFriendSrv

	// === Dialing coordinator ===
	dialingSrv := &ncoordinator.Server{
		Service:      "Dialing",
		PrivateKey:   coordPriv,
		Log:          quietLogger(),
		ConfigClient: cfgClient,
		MixWait:      1 * time.Second,
		RoundWait:    2 * time.Second,
		NumMailboxes: 1,
		PersistPath:  filepath.Join(tmpDir, "dialing-coordinator-state"),
	}
	if err := dialingSrv.Persist(); err != nil {
		return wrap("dialing Persist", err)
	}
	if err := dialingSrv.LoadPersistedState(); err != nil {
		return wrap("dialing LoadPersistedState", err)
	}
	if err := dialingSrv.Run(); err != nil {
		return wrap("dialing Run", err)
	}
	h.cleanups = append(h.cleanups, func() { _ = dialingSrv.Close() })
	h.cleanups = append(h.cleanups, func() { _ = addFriendSrv.Close() })

	// === HTTP mux for both coordinators ===
	mux := http.NewServeMux()
	mux.Handle("/addfriend/", http.StripPrefix("/addfriend", addFriendSrv))
	mux.Handle("/dialing/", http.StripPrefix("/dialing", dialingSrv))
	coordHTTP := &http.Server{Handler: mux}
	go func() {
		_ = coordHTTP.Serve(coordListener)
	}()
	h.cleanups = append(h.cleanups, func() { _ = coordHTTP.Close() })

	// Remember keys/addrs internally so ClientFor can wire clients.
	h.neverlurCoordKey = coordPub
	h.neverlurCoordAddr = coordAddr
	h.neverlurCfgClient = cfgClient

	return nil
}

func quietLogger() *nlog.Logger {
	return &nlog.Logger{
		Level:        nlog.ErrorLevel,
		EntryHandler: &nlog.OutputText{Out: nlog.Stderr},
	}
}
