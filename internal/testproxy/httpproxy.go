package testproxy

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"testing"
	"time"
)

// hopByHopHeaders are never forwarded by a proxy (RFC 7230 section
// 6.1, plus the proxy-specific credentials and challenge headers).
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Proxy-Authorization",
	"Proxy-Authenticate",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// copyHeaders returns a copy of h without the hop-by-hop headers.
func copyHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	maps.Copy(out, h)
	for _, k := range hopByHopHeaders {
		out.Del(k)
	}
	return out
}

// forwardHandler is the shared request/response recording handler
// used by both the HTTP and HTTPS forward proxies. It requires
// absolute-form request URIs, optionally enforces Basic proxy
// authentication, forwards the request with a plain client, and
// records the full exchange.
func forwardHandler(rec *Recorder, authUser, authPass string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "" {
			http.Error(w, "absolute-form request URI required", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read request body: %v", err), http.StatusBadGateway)
			return
		}

		if authUser != "" {
			if !proxyBasicAuthOK(r.Header.Get("Proxy-Authorization"), authUser, authPass) {
				rec.Add(Exchange{
					Method:        r.Method,
					URL:           r.URL.String(),
					RequestHeader: r.Header.Clone(),
					RequestBody:   body,
					Status:        http.StatusProxyAuthRequired,
					RemoteAddr:    r.RemoteAddr,
					At:            time.Now(),
				})
				w.Header().Set("Proxy-Authenticate", `Basic realm="testproxy"`)
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}
		}

		outReq, err := http.NewRequest(r.Method, r.URL.String(), bytes.NewReader(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("build forwarded request: %v", err), http.StatusBadGateway)
			return
		}
		outReq.Header = copyHeaders(r.Header)
		outReq.Host = r.URL.Host

		client := &http.Client{Transport: &http.Transport{}}
		resp, err := client.Do(outReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("forward request: %v", err), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read response body: %v", err), http.StatusBadGateway)
			return
		}

		rec.Add(Exchange{
			Method:         r.Method,
			URL:            r.URL.String(),
			RequestHeader:  r.Header.Clone(),
			RequestBody:    body,
			Status:         resp.StatusCode,
			ResponseHeader: resp.Header.Clone(),
			ResponseBody:   respBody,
			RemoteAddr:     r.RemoteAddr,
			At:             time.Now(),
		})

		maps.Copy(w.Header(), copyHeaders(resp.Header))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
	}
}

// proxyBasicAuthOK reports whether the Proxy-Authorization header
// carries valid Basic credentials for user/pass.
func proxyBasicAuthOK(header, user, pass string) bool {
	const prefix = "Basic "
	if len(header) < len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	gotUser, gotPass, ok := cutUserPass(string(decoded))
	return ok && gotUser == user && gotPass == pass
}

func cutUserPass(s string) (user, pass string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// Server is a running recording proxy bound to a loopback listener.
type Server struct {
	addr    string
	scheme  string
	srv     *http.Server
	rec     *Recorder
	user    string
	pass    string
	certPEM []byte
	closed  bool
}

// URL returns the proxy URL, e.g. "http://127.0.0.1:41234".
func (s *Server) URL() string {
	return s.scheme + "://" + s.addr
}

// Close shuts the proxy down; normally reached via t.Cleanup.
func (s *Server) Close() {
	if !s.closed {
		s.closed = true
		_ = s.srv.Close()
	}
}

// HTTPOption configures a forward proxy before it starts.
type HTTPOption func(*Server)

// WithBasicAuth makes the proxy reject requests without a valid
// Proxy-Authorization Basic header for the given credentials.
func WithBasicAuth(user, pass string) HTTPOption {
	return func(s *Server) {
		s.user = user
		s.pass = pass
	}
}

// NewHTTPProxy starts a recording HTTP forward proxy on a
// kernel-assigned loopback port and returns it with its recorder.
func NewHTTPProxy(t *testing.T, opts ...HTTPOption) (*Server, *Recorder) {
	t.Helper()
	s := &Server{scheme: "http", rec: &Recorder{}}
	for _, o := range opts {
		o(s)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testproxy: listen: %v", err)
	}
	s.addr = ln.Addr().String()
	s.srv = &http.Server{
		Handler:           forwardHandler(s.rec, s.user, s.pass),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = s.srv.Serve(ln) }()
	t.Cleanup(s.Close)
	return s, s.rec
}
