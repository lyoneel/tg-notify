package testproxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"testing"
	"time"
)

// NewHTTPSProxy starts a recording forward proxy behind TLS with a
// fresh self-signed certificate and returns it with its recorder and
// the PEM-encoded CA certificate a client must trust. The handler is
// identical to the plain HTTP proxy; only the listener differs. With
// a plain-HTTP target no CONNECT occurs, so the proxy still sees and
// records the full request and response.
func NewHTTPSProxy(t *testing.T) (*Server, *Recorder, []byte) {
	t.Helper()

	certPEM, keyPEM, err := SelfSignedCert("testproxy.localhost")
	if err != nil {
		t.Fatalf("testproxy: self-signed cert: %v", err)
	}
	keyPair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("testproxy: load key pair: %v", err)
	}

	s := &Server{scheme: "https", rec: &Recorder{}, certPEM: certPEM}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testproxy: listen: %v", err)
	}
	s.addr = ln.Addr().String()
	s.srv = &http.Server{
		Handler:           forwardHandler(s.rec, "", ""),
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{keyPair}, MinVersion: tls.VersionTLS12},
	}
	go func() { _ = s.srv.ServeTLS(ln, "", "") }()
	t.Cleanup(s.Close)
	return s, s.rec, certPEM
}
