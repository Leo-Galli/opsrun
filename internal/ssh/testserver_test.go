// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	xssh "golang.org/x/crypto/ssh"
)

// testServer is a minimal in-process SSH server used to exercise SSHTransport without a
// real network or a real sshd. Commands are interpreted as small scripted scenarios so
// tests can simulate success, failure, slowness, and disconnects deterministically.
type testServer struct {
	listener  net.Listener
	hostKey   xssh.Signer
	rejectAll bool

	mu          sync.Mutex
	connections int
	sessions    int
}

func (ts *testServer) Connections() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.connections
}

func (ts *testServer) Sessions() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.sessions
}

func newTestServer(t testing.TB) *testServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := xssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("wrap host key: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ts := &testServer{listener: ln, hostKey: signer}
	go ts.serve()
	t.Cleanup(func() { ln.Close() })
	return ts
}

func (ts *testServer) Addr() string { return ts.listener.Addr().String() }

func (ts *testServer) HostKey() xssh.PublicKey { return ts.hostKey.PublicKey() }

func (ts *testServer) serve() {
	for {
		conn, err := ts.listener.Accept()
		if err != nil {
			return
		}
		go ts.handleConn(conn)
	}
}

func (ts *testServer) handleConn(conn net.Conn) {
	config := &xssh.ServerConfig{
		PublicKeyCallback: func(c xssh.ConnMetadata, pubKey xssh.PublicKey) (*xssh.Permissions, error) {
			if ts.rejectAll {
				return nil, errors.New("access denied")
			}
			return &xssh.Permissions{}, nil
		},
	}
	config.AddHostKey(ts.hostKey)

	sconn, chans, reqs, err := xssh.NewServerConn(conn, config)
	if err != nil {
		conn.Close()
		return
	}
	ts.mu.Lock()
	ts.connections++
	ts.mu.Unlock()
	defer sconn.Close()
	go xssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(xssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		ts.mu.Lock()
		ts.sessions++
		ts.mu.Unlock()
		go ts.handleSession(channel, requests)
	}
}

func (ts *testServer) handleSession(channel xssh.Channel, requests <-chan *xssh.Request) {
	defer channel.Close()
	for req := range requests {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			if err := xssh.Unmarshal(req.Payload, &payload); err != nil {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			ts.runScenario(channel, payload.Command)
			return
		case "pty-req", "env":
			req.Reply(true, nil)
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// runScenario interprets command as "scenario:argument" and drives the channel
// accordingly, then sends an exit-status (or, for the disconnect scenario, none at all).
func (ts *testServer) runScenario(channel xssh.Channel, command string) {
	scenario, arg, _ := strings.Cut(command, ":")
	switch scenario {
	case "echo":
		channel.Write([]byte(arg + "\n")) //nolint:errcheck
		sendExitStatus(channel, 0)
	case "stderr":
		channel.Stderr().Write([]byte(arg + "\n")) //nolint:errcheck
		sendExitStatus(channel, 1)
	case "exit":
		code, _ := strconv.Atoi(arg)
		sendExitStatus(channel, code)
	case "sleep":
		d, err := time.ParseDuration(arg)
		if err != nil {
			d = time.Second
		}
		time.Sleep(d)
		sendExitStatus(channel, 0)
	case "disconnect":
		// Abruptly close without any exit-status, simulating a dropped connection.
		return
	default:
		channel.Write([]byte(command + "\n")) //nolint:errcheck
		sendExitStatus(channel, 0)
	}
}

func sendExitStatus(channel xssh.Channel, code int) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(code))
	channel.SendRequest("exit-status", false, payload) //nolint:errcheck
}

// clientKeySigner returns a fresh ed25519 signer usable as a client auth key in tests.
func clientKeySigner(t testing.TB) xssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	signer, err := xssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("wrap client key: %v", err)
	}
	return signer
}
