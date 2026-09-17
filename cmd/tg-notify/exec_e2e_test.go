package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/lyoneel/tg-notify/internal/testproxy"
)

var (
	buildOnce  sync.Once
	binaryPath string
	binaryErr  error
)

// buildBinary compiles the CLI once per test package into a temp dir
// so exec tests exercise the real process boundary.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		binaryPath = filepath.Join(os.TempDir(), "tg-notify-e2e", "tg-notify")
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
			binaryErr = err
			return
		}
		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		binaryErr = cmd.Run()
		if binaryErr != nil {
			binaryErr = &buildError{msg: out.String(), err: binaryErr}
		}
	})
	if binaryErr != nil {
		t.Fatalf("build tg-notify binary: %v", binaryErr)
	}
	return binaryPath
}

type buildError struct {
	msg string
	err error
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.msg }

// scrubbedOS returns os.Environ() without any proxy or Telegram
// variables, so a test process can never inherit ambient routing.
func scrubbedOS() []string {
	var out []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "TELEGRAM_PROXY",
			"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID", "TELEGRAM_BASE_URL":
			continue
		}
		out = append(out, kv)
	}
	return out
}

// runBinary executes the built CLI in workDir with extra env vars and
// returns its exit code and captured output. The command times out
// after 30 seconds.
func runBinary(t *testing.T, workDir string, env []string, args ...string) (int, string, string) {
	t.Helper()
	bin := buildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(scrubbedOS(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run binary: %v", err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

func TestExecMessageViaEnv(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	env := []string{
		"TELEGRAM_BOT_TOKEN=TESTTOKEN",
		"TELEGRAM_CHAT_ID=123",
		"TELEGRAM_BASE_URL=" + api.URL(),
		"TELEGRAM_PROXY=" + proxySrv.URL(),
	}
	code, stdout, stderr := runBinary(t, t.TempDir(), env, "--no-retry", "process smoke")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if stdout != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", stdout)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("proxy exchanges = %d, want 1 (env TELEGRAM_PROXY must route the real process)", rec.Count())
	}
	if ex := rec.All()[0]; !strings.Contains(string(ex.RequestBody), `"text":"process smoke"`) {
		t.Errorf("recorded request body %q missing message text", ex.RequestBody)
	}
}

func TestExecFlagOverridesEnv(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	envProxy, envRec := testproxy.NewHTTPProxy(t)
	flagProxy, flagRec := testproxy.NewHTTPProxy(t)

	env := []string{
		"TELEGRAM_BOT_TOKEN=TESTTOKEN",
		"TELEGRAM_CHAT_ID=123",
		"TELEGRAM_BASE_URL=" + api.URL(),
		"TELEGRAM_PROXY=" + envProxy.URL(),
	}
	code, stdout, stderr := runBinary(t, t.TempDir(), env,
		"--no-retry", "--proxy", flagProxy.URL(), "flag wins")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if stdout != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", stdout)
	}
	if err := flagRec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if flagRec.Count() != 1 {
		t.Errorf("flag proxy exchanges = %d, want 1", flagRec.Count())
	}
	if envRec.Count() != 0 {
		t.Errorf("env proxy exchanges = %d, want 0 (--proxy flag must win)", envRec.Count())
	}
}

func TestExecDotEnvLoading(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	workDir := t.TempDir()
	dotEnv := strings.Join([]string{
		"TELEGRAM_BOT_TOKEN=TESTTOKEN",
		"TELEGRAM_CHAT_ID=123",
		"TELEGRAM_BASE_URL=" + api.URL(),
		"TELEGRAM_PROXY=" + proxySrv.URL(),
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(workDir, ".env"), []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	code, stdout, stderr := runBinary(t, workDir, nil, "--no-retry", "dotenv smoke")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if stdout != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", stdout)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("proxy exchanges = %d, want 1 (.env TELEGRAM_PROXY must route)", rec.Count())
	}
}

func TestExecUnknownFlagExitsNonZero(t *testing.T) {
	code, stdout, stderr := runBinary(t, t.TempDir(), nil, "--bogus")
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero for an unknown flag")
	}
	if !strings.Contains(stderr, "flag provided but not defined") {
		t.Errorf("stderr = %q, want flag error message", stderr)
	}
	if strings.Contains(stdout, "Sent") {
		t.Errorf("stdout = %q, must not report success", stdout)
	}
}

func TestExecDeadProxyErrorScrubbed(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)

	const token = "TESTTOKEN"
	env := []string{
		"TELEGRAM_BOT_TOKEN=" + token,
		"TELEGRAM_CHAT_ID=123",
		"TELEGRAM_BASE_URL=" + api.URL(),
		"TELEGRAM_PROXY=http://127.0.0.1:1",
	}
	code, stdout, stderr := runBinary(t, t.TempDir(), env, "--no-retry", "unreachable proxy")
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero with a dead proxy")
	}
	if !strings.Contains(stderr, "Failed:") {
		t.Errorf("stderr = %q, want Failed: prefix", stderr)
	}
	if strings.Contains(stderr, token) {
		t.Errorf("stderr leaks the bot token: %q", stderr)
	}
	if strings.Contains(stdout, "Sent") {
		t.Errorf("stdout = %q, must not report success", stdout)
	}
	if api.Hits() != 0 {
		t.Errorf("fake API hit count = %d, want 0", api.Hits())
	}
}

func TestExecVersion(t *testing.T) {
	code, stdout, _ := runBinary(t, t.TempDir(), nil, "--version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Errorf("stdout empty, want a version string")
	}
}
