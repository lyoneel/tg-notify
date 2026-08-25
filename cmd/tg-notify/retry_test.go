package main

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
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

func TestRetryWithBackoff(t *testing.T) {
	apiErr := func(code, retryAfter int) error {
		return &telegram.APIError{Code: code, Description: "x", RetryAfter: retryAfter}
	}

	tests := []struct {
		name           string
		noRetry        bool
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
			name:           "429 does not retry when noRetry",
			noRetry:        true,
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
			name:           "noRetry makes exactly one attempt",
			noRetry:        true,
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
			origSleep := sleep
			sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
			defer func() { sleep = origSleep }()

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

			id, err := retryWithBackoff(call, tt.noRetry, tt.maxRetries, tt.baseWait)
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
		{name: "429 not transient", err: &telegram.APIError{Code: 429}, want: false},
		{name: "499 not transient", err: &telegram.APIError{Code: 499}, want: false},
		{name: "500 transient", err: &telegram.APIError{Code: 500}, want: true},
		{name: "502 transient", err: &telegram.APIError{Code: 502}, want: true},
		{name: "599 transient", err: &telegram.APIError{Code: 599}, want: true},
		{name: "404 not transient", err: &telegram.APIError{Code: 404}, want: false},
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

func TestRunFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "negative retries rejected",
			args:    []string{"--retries", "-1", "hello"},
			wantErr: "--retries must be >= 0",
		},
		{
			name:    "zero base-wait rejected",
			args:    []string{"--base-wait", "0s", "hello"},
			wantErr: "--base-wait must be > 0",
		},
		{
			name:    "negative base-wait rejected",
			args:    []string{"--base-wait", "-1s", "hello"},
			wantErr: "--base-wait must be > 0",
		},
		{
			name:    "negative reply-to rejected",
			args:    []string{"--reply-to", "-1", "hello"},
			wantErr: "--reply-to must be >= 0",
		},
		{
			name:    "negative offset rejected",
			args:    []string{"--discover-chat-id", "--offset", "-1"},
			wantErr: "--offset must be >= 0",
		},
		{
			name:    "offset without discover rejected",
			args:    []string{"--offset", "5", "hello"},
			wantErr: "--offset is only valid with --discover-chat-id",
		},
		{
			name:    "whoami with message rejected",
			args:    []string{"--whoami", "hello"},
			wantErr: "--whoami cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "whoami with reply-to rejected",
			args:    []string{"--whoami", "--reply-to", "1"},
			wantErr: "--whoami cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "discover with message rejected",
			args:    []string{"--discover-chat-id", "hello"},
			wantErr: "--discover-chat-id cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "discover with file rejected",
			args:    []string{"--discover-chat-id", "-f", "a.jpg"},
			wantErr: "--discover-chat-id cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "no-trim on positional input rejected",
			args:    []string{"--no-trim", "hello"},
			wantErr: "--no-trim only applies to a message read from stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil {
				t.Fatalf("run(%v) = nil, want error %q", tt.args, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("run(%v) = %q, want %q", tt.args, err.Error(), tt.wantErr)
			}
		})
	}
}
