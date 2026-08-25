package testproxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	socksVersion5 = 0x05

	socksMethodNoAuth   = 0x00
	socksMethodUserPass = 0x02
	socksMethodNone     = 0xFF

	socksCmdConnect = 0x01

	socksATYPIPv4 = 0x01
	socksATYPFQDN = 0x03
	socksATYPIPv6 = 0x04

	socksRepSuccess         = 0x00
	socksRepGeneralFailure  = 0x01
	socksRepConnRefused     = 0x05
	socksRepCmdUnsupported  = 0x07
	socksRepATYPUnsupported = 0x08

	authVersion1 = 0x01
	authSuccess  = 0x00
	authFailure  = 0x01

	// tunnelByteCap bounds the per-direction byte dump recorded from a
	// tunnelled connection so a runaway transfer cannot eat memory.
	tunnelByteCap = 1 << 20
)

// SOCKSOption configures a SOCKS5 proxy before it starts.
type SOCKSOption func(*socksConfig)

type socksConfig struct {
	user string
	pass string
}

// WithUserPass requires RFC 1929 username/password authentication and
// rejects any other credentials.
func WithUserPass(user, pass string) SOCKSOption {
	return func(c *socksConfig) {
		c.user = user
		c.pass = pass
	}
}

// SOCKSServer is a running recording SOCKS5 proxy bound to a loopback
// listener.
type SOCKSServer struct {
	addr   string
	ln     net.Listener
	rec    *Recorder
	cfg    socksConfig
	wg     sync.WaitGroup
	closed bool

	connMu sync.Mutex
	conns  map[net.Conn]bool
}

// Addr returns the host:port of the proxy.
func (s *SOCKSServer) Addr() string { return s.addr }

// URL returns the proxy URL without credentials,
// e.g. "socks5://127.0.0.1:41234".
func (s *SOCKSServer) URL() string { return "socks5://" + s.addr }

// URLWithAuth returns the proxy URL with embedded credentials,
// e.g. "socks5://user:pass@127.0.0.1:41234".
func (s *SOCKSServer) URLWithAuth(user, pass string) string {
	return "socks5://" + user + ":" + pass + "@" + s.addr
}

// Close stops accepting connections, terminates open tunnels, and
// waits for the handlers to finish; normally reached via t.Cleanup.
func (s *SOCKSServer) Close() {
	if !s.closed {
		s.closed = true
		_ = s.ln.Close()
		s.connMu.Lock()
		for c := range s.conns {
			_ = c.Close()
		}
		s.connMu.Unlock()
		s.wg.Wait()
	}
}

func (s *SOCKSServer) trackConn(c net.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conns == nil {
		s.conns = make(map[net.Conn]bool)
	}
	s.conns[c] = true
}

func (s *SOCKSServer) untrackConn(c net.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	delete(s.conns, c)
}

// NewSOCKS5Proxy starts a recording SOCKS5 proxy (RFC 1928, CONNECT
// only) on a kernel-assigned loopback port and returns it with its
// recorder. The handshake and, when negotiated, the RFC 1929
// credentials are recorded; every tunneled byte in both directions is
// captured up to tunnelByteCap per direction.
func NewSOCKS5Proxy(t *testing.T, opts ...SOCKSOption) (*SOCKSServer, *Recorder) {
	t.Helper()
	s := &SOCKSServer{rec: &Recorder{}}
	for _, o := range opts {
		o(&s.cfg)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testproxy: listen: %v", err)
	}
	s.addr = ln.Addr().String()
	s.ln = ln
	go s.acceptLoop()
	t.Cleanup(s.Close)
	return s, s.rec
}

func (s *SOCKSServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.trackConn(c)
			defer s.untrackConn(c)
			s.handleConn(c)
		}(conn)
	}
}

func (s *SOCKSServer) handleConn(client net.Conn) {
	defer func() { _ = client.Close() }()
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))

	h := Handshake{}

	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil {
		return
	}
	if header[0] != socksVersion5 {
		return
	}
	h.Version = header[0]
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	h.OfferedMethods = methods

	selected := byte(socksMethodNoAuth)
	if s.cfg.user != "" {
		if !hasMethod(methods, socksMethodUserPass) {
			_, _ = client.Write([]byte{socksVersion5, socksMethodNone})
			return
		}
		selected = socksMethodUserPass
	}
	h.SelectedMethod = selected
	if _, err := client.Write([]byte{socksVersion5, selected}); err != nil {
		return
	}

	if selected == socksMethodUserPass {
		user, pass, err := readUserPassAuth(client)
		if err != nil {
			return
		}
		h.Username = user
		h.Password = pass
		if user != s.cfg.user || pass != s.cfg.pass {
			_, _ = client.Write([]byte{authVersion1, authFailure})
			return
		}
		if _, err := client.Write([]byte{authVersion1, authSuccess}); err != nil {
			return
		}
	}

	req := make([]byte, 4)
	if _, err := io.ReadFull(client, req); err != nil {
		return
	}
	if req[1] != socksCmdConnect {
		_ = writeSocksReply(client, socksRepCmdUnsupported)
		return
	}

	host, port, err := readSocksAddress(client, req[3])
	if err != nil {
		_ = writeSocksReply(client, socksRepATYPUnsupported)
		return
	}
	h.TargetHost = host
	h.TargetPort = port
	s.rec.AddHandshake(h)

	target, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))), 5*time.Second)
	if err != nil {
		_ = writeSocksReply(client, socksRepConnRefused)
		return
	}
	s.trackConn(target)
	defer func() {
		_ = target.Close()
		s.untrackConn(target)
	}()

	tunnel := s.rec.StartTunnel(net.JoinHostPort(host, strconv.Itoa(int(port))), client.RemoteAddr().String())

	if err := writeSocksReply(client, socksRepSuccess); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})

	s.splice(client, target, tunnel)
}

// readUserPassAuth performs the RFC 1929 subnegotiation and returns
// the offered credentials.
func readUserPassAuth(client net.Conn) (string, string, error) {
	ver := make([]byte, 1)
	if _, err := io.ReadFull(client, ver); err != nil {
		return "", "", err
	}
	if ver[0] != authVersion1 {
		return "", "", fmt.Errorf("unexpected auth version %d", ver[0])
	}
	user, err := readLengthPrefixed(client)
	if err != nil {
		return "", "", err
	}
	pass, err := readLengthPrefixed(client)
	if err != nil {
		return "", "", err
	}
	return user, pass, nil
}

func readLengthPrefixed(r io.Reader) (string, error) {
	lenBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return "", err
	}
	buf := make([]byte, int(lenBuf[0]))
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readSocksAddress decodes the connect target per ATYP.
func readSocksAddress(client net.Conn, atyp byte) (string, uint16, error) {
	switch atyp {
	case socksATYPIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(client, ip); err != nil {
			return "", 0, err
		}
		port, err := readPort(client)
		if err != nil {
			return "", 0, err
		}
		return net.IP(ip).String(), port, nil
	case socksATYPIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(client, ip); err != nil {
			return "", 0, err
		}
		port, err := readPort(client)
		if err != nil {
			return "", 0, err
		}
		return net.IP(ip).String(), port, nil
	case socksATYPFQDN:
		host, err := readLengthPrefixed(client)
		if err != nil {
			return "", 0, err
		}
		port, err := readPort(client)
		if err != nil {
			return "", 0, err
		}
		return host, port, nil
	default:
		return "", 0, fmt.Errorf("unsupported ATYP %d", atyp)
	}
}

func readPort(client net.Conn) (uint16, error) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(client, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf), nil
}

// writeSocksReply sends the bind reply with a zero bound address.
func writeSocksReply(client net.Conn, rep byte) error {
	_, err := client.Write([]byte{socksVersion5, rep, 0x00, socksATYPIPv4, 0, 0, 0, 0, 0, 0})
	return err
}

func hasMethod(methods []byte, m byte) bool {
	return slices.Contains(methods, m)
}

// capWriter stores up to cap bytes of everything written through it.
type capWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (c *capWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if remain := tunnelByteCap - len(c.buf); remain > 0 {
		if len(p) > remain {
			p = p[:remain]
		}
		c.buf = append(c.buf, p...)
	}
	return len(p), nil
}

func (c *capWriter) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, len(c.buf))
	copy(out, c.buf)
	return out
}

// splice copies both tunnel directions while recording every byte
// into the tunnel's live byte dumps.
func (s *SOCKSServer) splice(client, target net.Conn, tunnel *Tunnel) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(target, tunnel.clientBytes), client)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(client, tunnel.targetBytes), target)
	}()
	wg.Wait()
}
