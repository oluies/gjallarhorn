// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import "errors"

// Sentinel errors returned by the harness and its test clients.
// Stable across the package so tests can match on errors.Is.

// errEmptyMessage is returned by TestClient.SendMessage when the body
// is zero-length.
var errEmptyMessage = errors.New("message body is empty")

// errInvalidUTF8 is returned by TestClient.SendMessage when the body
// is not valid UTF-8.
var errInvalidUTF8 = errors.New("message body is not valid UTF-8")

// errUnexpectedSigningKey is pushed onto a TestClient's errCh when
// the underlying neverlur.EventHandler.UnexpectedSigningKey callback
// fires (a friend's identity key changed between requests).
var errUnexpectedSigningKey = errors.New("friend signing key changed unexpectedly")

// errMessageTooLarge is returned by TestClient.SendMessage when the
// body exceeds convo.SizeMessageBody bytes (the on-wire ceiling
// inside one DeadDropMessage).
var errMessageTooLarge = errors.New("message body exceeds convo.SizeMessageBody")

// errNoConvoState is returned by TestClient.SendMessage before the
// AddFriend + dial handshakes have bootstrapped a ConvoState.
var errNoConvoState = errors.New("no active conversation state (call SendCall first)")
