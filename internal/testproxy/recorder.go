// Package testproxy provides recording HTTP, HTTPS, and SOCKS5
// proxies for the tg-notify proxy end-to-end tests. Every server
// binds 127.0.0.1 on a kernel-assigned port and closes automatically
// via t.Cleanup, so the tests stay hermetic. Nothing here should
// ever listen on a public interface or serve real traffic.
package testproxy

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Exchange is one proxied request/response pair as recorded by an
// HTTP or HTTPS forward proxy, or one direction of a SOCKS5 tunnel.
type Exchange struct {
	Method string
	URL    string

	RequestHeader  http.Header
	RequestBody    []byte
	Status         int
	ResponseHeader http.Header
	ResponseBody   []byte

	RemoteAddr string
	At         time.Time
}

// Handshake is the decoded SOCKS5 greeting and request of one
// connection: the offered and selected auth methods, the username
// and password when RFC 1929 authentication was negotiated, and the
// connect target.
type Handshake struct {
	Version        byte
	OfferedMethods []byte
	SelectedMethod byte
	Username       string
	Password       string
	TargetHost     string
	TargetPort     uint16
}

// Tunnel is a live SOCKS5 tunnel: the byte dumps fill in as data
// flows, so callers can assert on tunnelled traffic without waiting
// for the connection to close (keep-alive clients hold tunnels open).
type Tunnel struct {
	Target     string
	RemoteAddr string
	At         time.Time

	clientBytes *capWriter
	targetBytes *capWriter
}

// ClientBytes returns the bytes the client sent through the tunnel so
// far.
func (t *Tunnel) ClientBytes() []byte { return t.clientBytes.bytes() }

// TargetBytes returns the bytes the target sent back so far.
func (t *Tunnel) TargetBytes() []byte { return t.targetBytes.bytes() }

// Recorder is a mutex-guarded store of exchanges, SOCKS5 handshakes,
// and tunnels, safe for use from proxy handler goroutines.
type Recorder struct {
	mu         sync.Mutex
	exchanges  []Exchange
	handshakes []Handshake
	tunnels    []*Tunnel
}

// Add records one exchange.
func (r *Recorder) Add(e Exchange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges = append(r.exchanges, e)
}

// AddHandshake records one SOCKS5 handshake.
func (r *Recorder) AddHandshake(h Handshake) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handshakes = append(r.handshakes, h)
}

// StartTunnel registers a new tunnel and returns it; the returned
// tunnel's byte dumps grow live as data flows.
func (r *Recorder) StartTunnel(target, remoteAddr string) *Tunnel {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := &Tunnel{
		Target:      target,
		RemoteAddr:  remoteAddr,
		At:          time.Now(),
		clientBytes: &capWriter{},
		targetBytes: &capWriter{},
	}
	r.tunnels = append(r.tunnels, t)
	return t
}

// Tunnels returns a copy of the tunnel list.
func (r *Recorder) Tunnels() []*Tunnel {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Tunnel, len(r.tunnels))
	copy(out, r.tunnels)
	return out
}

// TunnelCount returns the number of tunnels recorded.
func (r *Recorder) TunnelCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tunnels)
}

// WaitForTunnels polls until at least n tunnels are recorded or the
// timeout elapses.
func (r *Recorder) WaitForTunnels(n int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.TunnelCount() >= n {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("recorder has %d tunnels after %s, want %d", r.TunnelCount(), timeout, n)
}

// All returns a copy of the recorded exchanges.
func (r *Recorder) All() []Exchange {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Exchange, len(r.exchanges))
	copy(out, r.exchanges)
	return out
}

// Handshakes returns a copy of the recorded SOCKS5 handshakes.
func (r *Recorder) Handshakes() []Handshake {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Handshake, len(r.handshakes))
	copy(out, r.handshakes)
	return out
}

// Count returns the number of recorded exchanges.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.exchanges)
}

// WaitFor polls until at least n exchanges are recorded or the
// timeout elapses.
func (r *Recorder) WaitFor(n int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.Count() >= n {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := r.Count(); got < n {
		return fmt.Errorf("recorder has %d exchanges after %s, want %d", got, timeout, n)
	}
	return nil
}
