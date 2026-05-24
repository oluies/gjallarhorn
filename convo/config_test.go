// Copyright 2017 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package convo

import (
	"crypto/ed25519"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/davidlazar/go-crypto/encoding/base32"

	"github.com/oluies/gjallarhorn/mixnet"
	"github.com/oluies/neverlur/config"
	"github.com/oluies/neverlur/hybrid"
	"github.com/oluies/neverlur/pqsig"
)

func TestMarshalConvoConfig(t *testing.T) {
	guardian, err := hybrid.GenerateHybridIdentity()
	if err != nil {
		t.Fatal(err)
	}

	conf := &config.SignedConfig{
		Version:          config.SignedConfigVersion,
		MinClientVersion: config.SignedConfigVersion,

		// UTC strips the *time.Location pointer (defaults to time.Local,
		// which reflect.DeepEqual treats as different from the fixed-zone
		// Location that JSON round-tripping produces). Round(0) drops the
		// monotonic clock. Mirrors the Neverlur-side fix in
		// neverlur/config/config_test.go.
		Created: time.Now().UTC().Round(0),
		Expires: time.Now().UTC().Round(0),

		Guardians: []config.Guardian{
			{
				Username: "david",
				Key:      guardian.EdPub,
				PQKey:    pqsig.PackPublicKey(guardian.PQPub),
			},
		},

		Service: "Convo",
		Inner: &ConvoConfig{
			Version: ConvoConfigVersion,

			Coordinator: CoordinatorConfig{
				Key:     guardian.EdPub,
				Address: "localhost:8080",
			},
			MixServers: []mixnet.PublicServerConfig{
				{
					Key:     guardian.EdPub,
					Address: "localhost:1234",
				},
			},
		},
	}
	msg := conf.SigningMessage()
	sigEd := ed25519.Sign(guardian.EdPriv, msg)
	sigPQ, err := pqsig.Sign(guardian.PQPriv, msg)
	if err != nil {
		t.Fatal(err)
	}
	var hs config.HybridSignature
	copy(hs.Ed[:], sigEd)
	copy(hs.PQ[:], sigPQ)
	conf.Signatures = map[string][]byte{
		base32.EncodeToString(guardian.EdPub): hs.Bytes(),
	}
	if err := conf.Verify(); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(conf)
	if err != nil {
		t.Fatal(err)
	}

	conf2 := new(config.SignedConfig)
	err = json.Unmarshal(data, conf2)
	if err != nil {
		t.Fatal(err)
	}

	if conf.Hash() != conf2.Hash() {
		t.Fatalf("round-trip failed:\nbefore=%#v\nafter=%#v\n", *conf, *conf2)
	}
	if !reflect.DeepEqual(conf, conf2) {
		t.Fatalf("round-trip failed:\nbefore=%#v\nafter=%#v\n", *conf, *conf2)
	}
}
