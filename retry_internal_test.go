package tgnotify

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// connRefused is a net.Error that is not a timeout.
type connRefused struct{}

func (connRefused) Error() string   { return "connection refused" }
func (connRefused) Timeout() bool   { return false }
func (connRefused) Temporary() bool { return true }

// netTimeout is a net.Error that reports Timeout() == true.
type netTimeout struct{}

func (netTimeout) Error() string   { return "context deadline exceeded" }
func (netTimeout) Timeout() bool   { return true }
func (netTimeout) Temporary() bool { return true }

var _ net.Error = connRefused{}
var _ net.Error = netTimeout{}

func TestRetry(t *testing.T) {
	apiErr := func(code, retryAfter int) error {
		return &APIError{Code: code, Description: "x", RetryAfter: retryAfter}
	}

	tests := []struct {
		name           string
		disabled       bool
		maxRetries     int
		baseWait       time.Duration
		results        []error // error per attempt; extra attempts reuse the last entry
		wantErr        bool
		wantAtmpt      int
		wantSleeps     []time.Duration // exact expected sleeps (429 only, not jittered); nil = skip
		wantSleepCount int             // number of sleeps expected; -1 = skip
	}{
		{
			name:           "429 with retry_after 0 waits default 5s then succeeds",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(429, 0), nil},
			wantAtmpt:      2,
			wantSleeps:     []time.Duration{5 * time.Second},
			wantSleepCount: 1,
		},
		{
			name:           "429 honours retry_after",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(429, 7), nil},
			wantAtmpt:      2,
			wantSleeps:     []time.Duration{7 * time.Second},
			wantSleepCount: 1,
		},
		{
			name:           "429 does not retry when disabled",
			disabled:       true,
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(429, 7)},
			wantErr:        true,
			wantAtmpt:      1,
			wantSleepCount: 0,
		},
		{
			name:           "502 succeeds on third attempt with backoff sleeps",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(502, 0), apiErr(502, 0), nil},
			wantAtmpt:      3,
			wantSleepCount: 2,
		},
		{
			name:           "net timeout succeeds on later attempt",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{netTimeout{}, netTimeout{}, nil},
			wantAtmpt:      3,
			wantSleepCount: -1,
		},
		{
			name:           "connection refused succeeds on later attempt",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{connRefused{}, nil},
			wantAtmpt:      2,
			wantSleepCount: -1,
		},
		{
			name:           "400 never retries",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(400, 0)},
			wantErr:        true,
			wantAtmpt:      1,
			wantSleepCount: -1,
		},
		{
			name:           "plain non-network error never retries",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{errors.New("boom")},
			wantErr:        true,
			wantAtmpt:      1,
			wantSleepCount: -1,
		},
		{
			name:           "disabled makes exactly one attempt",
			disabled:       true,
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(502, 0)},
			wantErr:        true,
			wantAtmpt:      1,
			wantSleepCount: -1,
		},
		{
			name:           "persistent transient exhausts budget returns last error",
			maxRetries:     3,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(502, 0), apiErr(503, 0)},
			wantErr:        true,
			wantAtmpt:      4, // 1 initial + 3 retries
			wantSleepCount: 3,
		},
		{
			name:           "maxRetries zero disables transient retries",
			maxRetries:     0,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(502, 0)},
			wantErr:        true,
			wantAtmpt:      1,
			wantSleepCount: 0,
		},
		{
			name:           "overridden maxRetries and baseWait drive sequence",
			maxRetries:     3,
			baseWait:       500 * time.Millisecond,
			results:        []error{apiErr(502, 0), apiErr(502, 0), apiErr(502, 0), nil},
			wantAtmpt:      4,
			wantSleepCount: 3,
		},
		{
			name:           "transient then 429 mid-loop retries once then succeeds",
			maxRetries:     60,
			baseWait:       2 * time.Second,
			results:        []error{apiErr(502, 0), apiErr(429, 3), nil},
			wantAtmpt:      3,
			wantSleepCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts int
			var sleeps []time.Duration
			var logged []string
			origSleep := sleep
			sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
			defer func() { sleep = origSleep }()

			policy := RetryPolicy{
				Disabled:   tt.disabled,
				MaxRetries: tt.maxRetries,
				BaseWait:   tt.baseWait,
				Logger: func(format string, args ...any) {
					logged = append(logged, format)
				},
			}

			call := func() (int64, error) {
				idx := attempts
				if idx >= len(tt.results) {
					idx = len(tt.results) - 1
				}
				attempts++
				err := tt.results[idx]
				if err == nil {
					return 42, nil
				}
				return 0, err
			}

			id, err := retry(policy, call)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && id != 42 {
				t.Errorf("id = %d, want 42", id)
			}
			if attempts != tt.wantAtmpt {
				t.Errorf("attempts = %d, want %d", attempts, tt.wantAtmpt)
			}
			if tt.wantSleeps != nil {
				if len(sleeps) != len(tt.wantSleeps) {
					t.Fatalf("sleeps = %v, want %v", sleeps, tt.wantSleeps)
				}
				for i := range sleeps {
					if sleeps[i] != tt.wantSleeps[i] {
						t.Errorf("sleep[%d] = %v, want %v", i, sleeps[i], tt.wantSleeps[i])
					}
				}
			}
			if tt.wantSleepCount >= 0 && len(sleeps) != tt.wantSleepCount {
				t.Errorf("sleep count = %d, want %d", len(sleeps), tt.wantSleepCount)
			}
			if len(logged) != len(sleeps) {
				t.Errorf("log lines = %d, sleeps = %d, want equal", len(logged), len(sleeps))
			}
		})
	}
}

func TestUncappedWait(t *testing.T) {
	tests := []struct {
		attempt int
		base    time.Duration
		want    time.Duration
	}{
		{1, 2 * time.Second, 2 * time.Second},
		{2, 2 * time.Second, 4 * time.Second},
		{3, 2 * time.Second, 8 * time.Second},
		{4, 2 * time.Second, 16 * time.Second},
		{5, 2 * time.Second, 32 * time.Second},
		{6, 2 * time.Second, 60 * time.Second}, // capped from 64s
		{7, 2 * time.Second, 60 * time.Second},
		{10, 2 * time.Second, 60 * time.Second},
		{1, 500 * time.Millisecond, 500 * time.Millisecond},
		{3, 500 * time.Millisecond, 2 * time.Second},
	}
	for _, tt := range tests {
		if got := uncappedWait(tt.attempt, tt.base); got != tt.want {
			t.Errorf("uncappedWait(%d, %v) = %v, want %v", tt.attempt, tt.base, got, tt.want)
		}
	}
}

func TestBackoffWaitJitterBounds(t *testing.T) {
	const base = 2 * time.Second
	for attempt := 1; attempt <= 12; attempt++ {
		uncapped := uncappedWait(attempt, base)
		for i := 0; i < 100; i++ {
			got := backoffWait(attempt, base)
			lo := float64(uncapped) * (1 - jitterFraction)
			hi := float64(uncapped) * (1 + jitterFraction)
			if float64(got) < lo || float64(got) > hi {
				t.Fatalf("attempt %d: backoffWait = %v, want within [%v, %v]", attempt, got, time.Duration(lo), time.Duration(hi))
			}
		}
	}
}

func TestTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "429 not transient", err: &APIError{Code: 429}, want: false},
		{name: "499 not transient", err: &APIError{Code: 499}, want: false},
		{name: "500 transient", err: &APIError{Code: 500}, want: true},
		{name: "502 transient", err: &APIError{Code: 502}, want: true},
		{name: "599 transient", err: &APIError{Code: 599}, want: true},
		{name: "404 not transient", err: &APIError{Code: 404}, want: false},
		{name: "connection refused transient", err: connRefused{}, want: true},
		{name: "timeout transient", err: netTimeout{}, want: true},
		{name: "plain error not transient", err: errors.New("boom"), want: false},
		{name: "missing file not transient", err: func() error { _, e := os.Open("/definitely/missing/file.jpg"); return e }(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transient(tt.err); got != tt.want {
				t.Errorf("transient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDefaultRetryPolicyMatchesCLIDefaults(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.Disabled || p.MaxRetries != 60 || p.BaseWait != 2*time.Second || p.Logger != nil {
		t.Fatalf("DefaultRetryPolicy = %+v, want enabled, 60 retries, 2s, silent", p)
	}
	bot := New("t")
	got := bot.RetryPolicy()
	if got.Disabled != p.Disabled || got.MaxRetries != p.MaxRetries || got.BaseWait != p.BaseWait {
		t.Fatalf("New bot policy = %+v, want %+v", got, p)
	}
}

func TestApplyRetryRejectsInvalidPolicy(t *testing.T) {
	policy := RetryPolicy{MaxRetries: 3, BaseWait: 0}
	if _, err := applyRetry(policy, func() (int64, error) { return 1, nil }); err == nil {
		t.Fatal("zero base wait with retries enabled must fail fast")
	}
}

func TestScrubTokenErrNoMatch(t *testing.T) {
	plain := errors.New("plain error")
	if scrubTokenErr(plain) != plain {
		t.Fatal("errors without tokens must pass through unchanged")
	}
	if scrubTokenErr(nil) != nil {
		t.Fatal("nil must pass through")
	}
	scrubbed := scrubTokenErr(errors.New("POST https://api.telegram.org/botSECRET/sendMessage: refused"))
	if strings.Contains(scrubbed.Error(), "SECRET") {
		t.Fatalf("scrubbed error still carries the token: %s", scrubbed)
	}
}

func TestCallJSONMarshalError(t *testing.T) {
	bot := New("t")
	if _, err := bot.callJSON(context.Background(), bot.jsonClient, "sendMessage", make(chan int)); err == nil {
		t.Fatal("unmarshalable payload must fail")
	}
}

func TestSendMessageOptsTextValidationRunsBeforeRetry(t *testing.T) {
	bot := New("t")
	bot.SetRetryPolicy(RetryPolicy{Disabled: false, MaxRetries: 3, BaseWait: time.Second})
	if _, err := bot.SendMessageOpts(context.Background(), "1", "", nil); err == nil {
		t.Fatal("empty message must fail client-side even with retries enabled")
	}
}

func TestWriteMultipartBrokenPipe(t *testing.T) {
	pr, pw := io.Pipe()
	_ = pr.Close()
	mw := multipart.NewWriter(pw)
	err := writeMultipart(mw, "document", "x.txt", strings.NewReader("d"), "1", &SendOptions{
		Caption: "c", ParseMode: "HTML", ReplyTo: 2, Silent: true,
	})
	if err == nil {
		t.Fatal("writing fields to a closed pipe must fail")
	}
}

func TestGetMeTransportError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	bot := New("t")
	bot.SetBaseURL("http://" + addr)
	bot.SetRetryPolicy(RetryPolicy{Disabled: true})
	if _, err := bot.GetMe(context.Background()); err == nil {
		t.Fatal("connection refused must fail GetMe")
	}
}
