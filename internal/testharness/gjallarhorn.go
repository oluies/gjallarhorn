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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	vrand "vuvuzela.io/crypto/rand"

	"github.com/oluies/gjallarhorn/convo"
	gcoordinator "github.com/oluies/gjallarhorn/coordinator"
	gmixnet "github.com/oluies/gjallarhorn/mixnet"
	"github.com/oluies/gjallarhorn/mixnet/convopb"

	"github.com/oluies/neverlur/config"
	"github.com/oluies/neverlur/edtls"
)

// startGjallarhornSide stands up the Convo-side servers (Convo
// coordinator + Convo mixchain). Mirrors startNeverlurSide but for
// the Gjallarhorn conversation mixnet.
func (h *Harness) startGjallarhornSide(tmpDir string) error {
	// === Coordinator key ===
	coordPub, coordPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return wrap("gjallarhorn coordinator key", err)
	}
	coordListener, err := edtls.Listen("tcp", "localhost:0", coordPriv)
	if err != nil {
		return wrap("gjallarhorn coordinator edtls.Listen", err)
	}
	coordAddr := coordListener.Addr().String()

	// === Convo mixchain (N mixers configured with the Convo service).
	// neverlur/mock.LaunchMixchain only wires AddFriend + Dialing
	// services, so the Gjallarhorn-specific equivalent lives here.
	convoChain, err := launchConvoMixchain(h.opts.gjallarhornMixers, coordPub)
	if err != nil {
		return wrap("launchConvoMixchain", err)
	}
	h.GjallarhornConvoMixers = convoChain
	h.cleanups = append(h.cleanups, func() { _ = convoChain.Close() })

	// === Convo SignedConfig ===
	convoConf := &config.SignedConfig{
		Version:          config.SignedConfigVersion,
		MinClientVersion: config.SignedConfigVersion,
		Created:          time.Now().UTC().Round(0),
		Expires:          time.Now().UTC().Add(1 * time.Hour).Round(0),
		Service:          "Convo",
		Inner: &convo.ConvoConfig{
			Version: convo.ConvoConfigVersion,
			Coordinator: convo.CoordinatorConfig{
				Key:     coordPub,
				Address: coordAddr,
			},
			MixServers: convoChain.Servers,
		},
	}
	if err := h.signConfigInPlace(convoConf); err != nil {
		return wrap("sign Convo config", err)
	}
	if h.neverlurCfgServer == nil {
		return wrap("publish Convo config", errNeverlurNotStarted)
	}
	if err := h.neverlurCfgServer.SetCurrentConfig(convoConf); err != nil {
		return wrap("publish Convo config", err)
	}
	h.ConvoConfig = convoConf

	// === Convo coordinator ===
	convoSrv := &gcoordinator.Server{
		Service:      "Convo",
		PrivateKey:   coordPriv,
		ConfigClient: h.neverlurCfgClient,
		RoundDelay:   2 * time.Second,
		PersistPath:  filepath.Join(tmpDir, "convo-coordinator-state"),
	}
	if err := convoSrv.Persist(); err != nil {
		return wrap("convo Persist", err)
	}
	if err := convoSrv.LoadPersistedState(); err != nil {
		return wrap("convo LoadPersistedState", err)
	}
	if err := convoSrv.Run(); err != nil {
		return wrap("convo Run", err)
	}
	h.GjallarhornCoordinator = convoSrv
	h.cleanups = append(h.cleanups, func() { _ = convoSrv.Close() })

	// === HTTP mux for the Convo coordinator ===
	mux := http.NewServeMux()
	mux.Handle("/convo/", http.StripPrefix("/convo", convoSrv))
	coordHTTP := &http.Server{Handler: mux}
	go func() {
		_ = coordHTTP.Serve(coordListener)
	}()
	h.cleanups = append(h.cleanups, func() { _ = coordHTTP.Close() })

	return nil
}

// launchConvoMixchain stands up N in-process Gjallarhorn mixers
// configured with the Convo service.
func launchConvoMixchain(length int, coordinatorKey ed25519.PublicKey) (*GjallarhornConvoMixchainHandle, error) {
	publicKeys := make([]ed25519.PublicKey, length)
	privateKeys := make([]ed25519.PrivateKey, length)
	listeners := make([]net.Listener, length)
	addrs := make([]string, length)
	for i := 0; i < length; i++ {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, wrap("gjallarhorn mixer keygen", err)
		}
		publicKeys[i], privateKeys[i] = pub, priv
		l, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, wrap("gjallarhorn mixer net.Listen", err)
		}
		listeners[i] = l
		addrs[i] = l.Addr().String()
	}

	mixServers := make([]*gmixnet.Server, length)
	rpcServers := make([]*grpc.Server, length)
	for pos := length - 1; pos >= 0; pos-- {
		mixer := &gmixnet.Server{
			SigningKey:     privateKeys[pos],
			CoordinatorKey: coordinatorKey,
			Services: map[string]gmixnet.MixService{
				"Convo": &convo.ConvoService{
					Laplace: vrand.Laplace{Mu: 100, B: 3.0},
				},
			},
		}
		creds := credentials.NewTLS(edtls.NewTLSServerConfig(privateKeys[pos]))
		grpcServer := grpc.NewServer(grpc.Creds(creds))
		convopb.RegisterMixnetServer(grpcServer, mixer)
		mixServers[pos] = mixer
		rpcServers[pos] = grpcServer

		go func(pos int) {
			_ = grpcServer.Serve(listeners[pos])
		}(pos)
	}

	servers := make([]gmixnet.PublicServerConfig, length)
	for i, mixer := range mixServers {
		servers[i] = gmixnet.PublicServerConfig{
			Key:     mixer.SigningKey.Public().(ed25519.PublicKey),
			Address: addrs[i],
		}
	}

	closers := make([]func() error, length)
	for i, srv := range rpcServers {
		s := srv
		closers[i] = func() error { s.Stop(); return nil }
	}

	return &GjallarhornConvoMixchainHandle{
		Servers:    servers,
		mixServers: mixServers,
		closers:    closers,
	}, nil
}
