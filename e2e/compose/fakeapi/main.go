// Command fakeapi is a standalone fake Telegram Bot API server for
// the Docker Compose proxy smoke tests. It answers the envelopes the
// tg-notify client decodes and logs every request as one JSON line on
// stdout, so `docker logs` is the assertion surface. It has no state
// and no dependencies beyond the standard library.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	addr := os.Getenv("FAKEAPI_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)

	log.Printf("fakeapi listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	entry := struct {
		Method  string `json:"method"`
		Path    string `json:"path"`
		BodyB64 string `json:"body_b64"`
		Remote  string `json:"remote"`
	}{
		Method:  r.Method,
		Path:    r.URL.Path,
		BodyB64: base64.StdEncoding.EncodeToString(body),
		Remote:  r.RemoteAddr,
	}
	line, _ := json.Marshal(entry)
	fmt.Println(string(line))

	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/sendMessage"):
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	case strings.HasSuffix(r.URL.Path, "/sendMediaGroup"):
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`))
	case strings.HasSuffix(r.URL.Path, "/getMe"):
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"TestBot","username":"testbot"}}`))
	case strings.HasSuffix(r.URL.Path, "/getUpdates"):
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"message":{"chat":{"id":987}}}]}`))
	case strings.HasSuffix(r.URL.Path, "/sendPhoto"),
		strings.HasSuffix(r.URL.Path, "/sendDocument"):
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	default:
		http.NotFound(w, r)
	}
}
