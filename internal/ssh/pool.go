// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

// PoolConfig configures an SSHTransport's connection pooling, authentication, and
// timeouts.
type PoolConfig struct {
	// GlobalMaxConnections bounds the number of simultaneous SSH connections (across
	// all hosts) the transport will keep open at once.
	GlobalMaxConnections int

	// PerHostMaxSessions bounds the number of simultaneous command sessions
	// multiplexed over a single host's connection.
	PerHostMaxSessions int

	// ConnectTimeout bounds how long dialing and the SSH handshake may take.
	ConnectTimeout time.Duration

	// IdleTimeout closes a pooled connection that has not been used for this long.
	// Zero disables idle expiry.
	IdleTimeout time.Duration

	// KeepAliveInterval sends a keepalive request on this interval to detect dead
	// connections. Zero disables keepalives.
	KeepAliveInterval time.Duration

	// HostKeyCallback verifies the remote host key. It is mandatory: callers that
	// want to disable verification must do so explicitly with
	// xssh.InsecureIgnoreHostKey(), which OpsRun only wires up behind an explicit,
	// loudly-warned CLI flag.
	HostKeyCallback xssh.HostKeyCallback

	// AuthForHost returns the authentication methods to use for host.
	AuthForHost func(ctx context.Context, host inventory.Host) ([]xssh.AuthMethod, error)

	// Retry overrides the default connection retry policy. The zero value uses
	// defaultRetryConfig().
	Retry retryConfig
}

func (c *PoolConfig) setDefaults() {
	if c.GlobalMaxConnections <= 0 {
		c.GlobalMaxConnections = 50
	}
	if c.PerHostMaxSessions <= 0 {
		c.PerHostMaxSessions = 2
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.Retry.MaxAttempts == 0 {
		c.Retry = defaultRetryConfig()
	}
}

// SSHTransport implements Transport over real SSH connections, pooling one connection
// per host and multiplexing sessions over it, within the configured limits. The name
// "stutters" as ssh.SSHTransport, but the unqualified name Transport is already taken by
// this package's interface; LocalTransport and FakeTransport follow the same naming
// convention for the other two implementations.
//
//nolint:revive // see above
type SSHTransport struct {
	cfg       PoolConfig
	globalSem semaphore

	mu      sync.Mutex
	clients map[string]*pooledClient
	dialing map[string]*dialCall
}

// dialCall represents a single in-flight dial for a host: every concurrent Open call for
// that host waits on done instead of starting a redundant dial, so a burst of concurrent
// callers for a host that has no connection yet still produces exactly one connection.
type dialCall struct {
	done chan struct{}
	pc   *pooledClient
	err  error
}

// NewSSHTransport builds an SSHTransport. HostKeyCallback and AuthForHost must be set.
func NewSSHTransport(cfg PoolConfig) (*SSHTransport, error) {
	if cfg.HostKeyCallback == nil {
		return nil, errors.New("PoolConfig.HostKeyCallback is required")
	}
	if cfg.AuthForHost == nil {
		return nil, errors.New("PoolConfig.AuthForHost is required")
	}
	cfg.setDefaults()
	return &SSHTransport{
		cfg:       cfg,
		globalSem: newSemaphore(cfg.GlobalMaxConnections),
		clients:   make(map[string]*pooledClient),
		dialing:   make(map[string]*dialCall),
	}, nil
}

// Open implements Transport.
func (t *SSHTransport) Open(ctx context.Context, host inventory.Host) (Session, error) {
	pc, err := t.getOrDial(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := pc.sessions.Acquire(ctx); err != nil {
		return nil, &TransportError{Class: ErrorClassCanceled, Host: host.Name, Err: err}
	}
	sess, err := pc.client.NewSession()
	if err != nil {
		pc.sessions.Release()
		t.dropClient(host.Name, pc)
		return nil, &TransportError{Class: ErrorClassConnection, Host: host.Name, Err: err}
	}
	return &sshSession{session: sess, host: host.Name, release: pc.sessions.Release}, nil
}

// Close implements Transport, closing every pooled connection.
func (t *SSHTransport) Close() error {
	t.mu.Lock()
	clients := t.clients
	t.clients = make(map[string]*pooledClient)
	t.mu.Unlock()

	var firstErr error
	for _, pc := range clients {
		if err := pc.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// getOrDial returns a pooled connection for host, dialing at most once even when many
// callers race to open the same host concurrently: the first caller claims the dialCall
// slot and dials; every other concurrent caller waits on that single in-flight dial
// instead of starting a redundant connection (which would otherwise leak connections and
// exhaust the global semaphore under concurrent load).
func (t *SSHTransport) getOrDial(ctx context.Context, host inventory.Host) (*pooledClient, error) {
	for {
		t.mu.Lock()
		if pc, ok := t.clients[host.Name]; ok {
			if !pc.isStale(t.cfg.IdleTimeout) {
				pc.touch()
				t.mu.Unlock()
				return pc, nil
			}
			delete(t.clients, host.Name)
			t.mu.Unlock()
			go pc.close() //nolint:errcheck
			continue
		}

		if dc, ok := t.dialing[host.Name]; ok {
			t.mu.Unlock()
			select {
			case <-dc.done:
				if dc.err != nil {
					return nil, dc.err
				}
				dc.pc.touch()
				return dc.pc, nil
			case <-ctx.Done():
				return nil, &TransportError{Class: ErrorClassCanceled, Host: host.Name, Err: ctx.Err()}
			}
		}

		dc := &dialCall{done: make(chan struct{})}
		t.dialing[host.Name] = dc
		t.mu.Unlock()

		pc, err := t.dialAndRegister(ctx, host, dc)
		return pc, err
	}
}

func (t *SSHTransport) dialAndRegister(ctx context.Context, host inventory.Host, dc *dialCall) (*pooledClient, error) {
	defer func() {
		t.mu.Lock()
		delete(t.dialing, host.Name)
		t.mu.Unlock()
		close(dc.done)
	}()

	if err := t.globalSem.Acquire(ctx); err != nil {
		dc.err = &TransportError{Class: ErrorClassCanceled, Host: host.Name, Err: err}
		return nil, dc.err
	}

	var client *xssh.Client
	dialErr := withRetry(ctx, t.cfg.Retry, func() error {
		c, err := t.dial(ctx, host)
		if err != nil {
			return err
		}
		client = c
		return nil
	})
	if dialErr != nil {
		t.globalSem.Release()
		dc.err = dialErr
		return nil, dialErr
	}

	pc := &pooledClient{
		client:        client,
		sessions:      newSemaphore(t.cfg.PerHostMaxSessions),
		lastUsed:      time.Now(),
		releaseGlobal: t.globalSem.Release,
	}
	pc.startKeepalive(t.cfg.KeepAliveInterval)

	t.mu.Lock()
	t.clients[host.Name] = pc
	t.mu.Unlock()

	dc.pc = pc
	return pc, nil
}

func (t *SSHTransport) dropClient(name string, pc *pooledClient) {
	t.mu.Lock()
	if t.clients[name] == pc {
		delete(t.clients, name)
	}
	t.mu.Unlock()
	go pc.close() //nolint:errcheck
}

func (t *SSHTransport) dial(ctx context.Context, host inventory.Host) (*xssh.Client, error) {
	auths, err := t.cfg.AuthForHost(ctx, host)
	if err != nil {
		return nil, &TransportError{Class: ErrorClassAuthn, Host: host.Name, Err: err}
	}

	config := &xssh.ClientConfig{
		User:            host.User,
		Auth:            auths,
		HostKeyCallback: t.cfg.HostKeyCallback,
		Timeout:         t.cfg.ConnectTimeout,
	}

	addr := net.JoinHostPort(host.Address, portOrDefault(host.Port))

	if host.ProxyJump == "" {
		client, err := xssh.Dial("tcp", addr, config)
		if err != nil {
			return nil, classifyDialError(host.Name, err)
		}
		return client, nil
	}

	jumpAddress, jumpPort, err := net.SplitHostPort(host.ProxyJump)
	if err != nil {
		jumpAddress, jumpPort = host.ProxyJump, "22"
	}
	jumpHost := inventory.Host{Name: host.ProxyJump, Address: jumpAddress, User: host.User}
	jumpAuths, err := t.cfg.AuthForHost(ctx, jumpHost)
	if err != nil {
		return nil, &TransportError{Class: ErrorClassAuthn, Host: host.ProxyJump, Err: err}
	}
	jumpConfig := &xssh.ClientConfig{
		User:            jumpHost.User,
		Auth:            jumpAuths,
		HostKeyCallback: t.cfg.HostKeyCallback,
		Timeout:         t.cfg.ConnectTimeout,
	}
	jumpClient, err := xssh.Dial("tcp", net.JoinHostPort(jumpAddress, jumpPort), jumpConfig)
	if err != nil {
		return nil, classifyDialError(jumpHost.Name, err)
	}

	conn, err := jumpClient.Dial("tcp", addr)
	if err != nil {
		jumpClient.Close() //nolint:errcheck
		return nil, &TransportError{Class: ErrorClassConnection, Host: host.Name, Err: fmt.Errorf("dial via jump host %s: %w", host.ProxyJump, err)}
	}
	ncc, chans, reqs, err := xssh.NewClientConn(conn, addr, config)
	if err != nil {
		jumpClient.Close() //nolint:errcheck
		return nil, classifyDialError(host.Name, err)
	}
	return xssh.NewClient(ncc, chans, reqs), nil
}

func portOrDefault(port int) string {
	if port <= 0 {
		port = 22
	}
	return fmt.Sprintf("%d", port)
}

func classifyDialError(host string, err error) *TransportError {
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) {
		return &TransportError{Class: ErrorClassHostKey, Host: host, Err: err}
	}
	var revokedErr *knownhosts.RevokedError
	if errors.As(err, &revokedErr) {
		return &TransportError{Class: ErrorClassHostKey, Host: host, Err: err}
	}
	if strings.Contains(err.Error(), "host key") {
		return &TransportError{Class: ErrorClassHostKey, Host: host, Err: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TransportError{Class: ErrorClassTimeout, Host: host, Err: err}
	}
	if _, ok := err.(*xssh.ExitError); ok {
		return &TransportError{Class: ErrorClassConnection, Host: host, Err: err}
	}
	if isAuthError(err) {
		return &TransportError{Class: ErrorClassAuthn, Host: host, Err: err}
	}
	return &TransportError{Class: ErrorClassConnection, Host: host, Err: err}
}

func isAuthError(err error) bool {
	msg := err.Error()
	for _, sub := range []string{"unable to authenticate", "no supported methods remain"} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

// pooledClient is one reused SSH connection to a host, with a per-host session
// semaphore and idle/keepalive bookkeeping.
type pooledClient struct {
	client        *xssh.Client
	sessions      semaphore
	releaseGlobal func()

	mu            sync.Mutex
	lastUsed      time.Time
	closed        bool
	stopKeepalive chan struct{}
}

func (pc *pooledClient) touch() {
	pc.mu.Lock()
	pc.lastUsed = time.Now()
	pc.mu.Unlock()
}

func (pc *pooledClient) isStale(idleTimeout time.Duration) bool {
	if idleTimeout <= 0 {
		return false
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.closed || time.Since(pc.lastUsed) > idleTimeout
}

func (pc *pooledClient) startKeepalive(interval time.Duration) {
	if interval <= 0 {
		return
	}
	pc.stopKeepalive = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-pc.stopKeepalive:
				return
			case <-ticker.C:
				if _, _, err := pc.client.SendRequest("keepalive@opsrun", true, nil); err != nil {
					pc.close() //nolint:errcheck
					return
				}
			}
		}
	}()
}

func (pc *pooledClient) close() error {
	pc.mu.Lock()
	if pc.closed {
		pc.mu.Unlock()
		return nil
	}
	pc.closed = true
	pc.mu.Unlock()

	if pc.stopKeepalive != nil {
		close(pc.stopKeepalive)
	}
	if pc.releaseGlobal != nil {
		pc.releaseGlobal()
	}
	return pc.client.Close()
}
