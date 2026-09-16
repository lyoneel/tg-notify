package tgnotify

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"net"
	"net/http"
	"time"
)

// Retry policy constants shared by the library and the CLI: the
// exponential factor, the per-wait cap, and the jitter fraction. The
// count and base wait come from the RetryPolicy.
const (
	waitFactor       = 2
	maxTransientWait = 60 * time.Second
	jitterFraction   = 0.25
)

// DefaultRetryWait is the wait applied on a 429 without retry_after.
const DefaultRetryWait = 5

// RetryPolicy configures automatic retries for the send and read
// methods. The default policy matches the CLI defaults: enabled, 60
// transient retries, 2s base wait.
//
// A 429 rate-limit error waits retry_after seconds (default 5) and
// retries exactly once. Transient errors (5xx APIError, net.Error)
// retry up to MaxRetries times with exponential backoff (BaseWait
// doubled per attempt, capped at 60s, ±25% jitter). Every other error
// returns immediately. Disabled makes a single attempt; MaxRetries == 0
// disables transient retries only.
type RetryPolicy struct {
	// Disabled turns every retry off: one attempt, no waits.
	Disabled bool
	// MaxRetries is the transient-retry budget. Zero disables
	// transient retries only.
	MaxRetries int
	// BaseWait is the first backoff wait; it must be positive when
	// retries are enabled.
	BaseWait time.Duration
	// Logger receives progress lines while waiting between attempts.
	// nil keeps retries silent. The CLI attaches a stderr writer
	// here; library callers can attach any logger.
	Logger func(format string, args ...any)
}

// DefaultRetryPolicy returns the CLI-default policy: enabled, 60
// transient retries, 2s base wait, silent.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 60, BaseWait: 2 * time.Second}
}

func (p RetryPolicy) logf(format string, args ...any) {
	if p.Logger != nil {
		p.Logger(format, args...)
	}
}

// sleep is indirection for time.Sleep so tests can replace it.
var sleep = time.Sleep

// retry runs send under the policy and returns the first successful
// result. It is the exact algorithm the CLI shipped before the
// library existed, ported verbatim.
func retry[T any](policy RetryPolicy, send func() (T, error)) (T, error) {
	val, err := send()
	if err == nil {
		return val, nil
	}
	if policy.Disabled {
		return val, err
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests {
		wait := apiErr.RetryAfter
		if wait <= 0 {
			wait = DefaultRetryWait
		}
		policy.logf("Rate limited. Retrying after %ds...\n", wait)
		sleep(time.Duration(wait) * time.Second)
		return send()
	}

	if !transient(err) {
		return val, err
	}

	for attempt := 1; attempt <= policy.MaxRetries; attempt++ {
		wait := backoffWait(attempt, policy.BaseWait)
		policy.logf("Transient error (%v). Retry %d/%d in %s...\n", err, attempt, policy.MaxRetries, wait)
		sleep(wait)
		val, err = send()
		if err == nil {
			return val, nil
		}
		var apiErr429 *APIError
		if errors.As(err, &apiErr429) && apiErr429.Code == http.StatusTooManyRequests {
			wait := apiErr429.RetryAfter
			if wait <= 0 {
				wait = DefaultRetryWait
			}
			policy.logf("Rate limited. Retrying after %ds...\n", wait)
			sleep(time.Duration(wait) * time.Second)
			val, err = send()
			if err == nil {
				return val, nil
			}
		}
		if !transient(err) {
			return val, err
		}
	}
	return val, err
}

// transient reports whether err is a retryable transient failure: an
// APIError with a 5xx code or a network-level error. Filesystem errors
// (missing files, permissions) are deliberately excluded so a bad local
// path fails fast instead of exhausting the retry budget.
func transient(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code >= 500 && apiErr.Code <= 599
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// backoffWait computes the jittered wait for a 1-based attempt number:
// the uncapped backoff is jittered by ±jitterFraction.
func backoffWait(attempt int, baseWait time.Duration) time.Duration {
	wait := uncappedWait(attempt, baseWait)
	jitter := float64(wait) * jitterFraction
	offset := rand.Float64()*2*jitter - jitter
	return time.Duration(float64(wait) + offset)
}

// uncappedWait computes the pre-jitter backoff for a 1-based attempt:
// baseWait * waitFactor^(attempt-1), capped at maxTransientWait.
func uncappedWait(attempt int, baseWait time.Duration) time.Duration {
	wait := baseWait
	for i := 1; i < attempt; i++ {
		wait *= waitFactor
		if wait >= maxTransientWait {
			wait = maxTransientWait
			break
		}
	}
	return wait
}

// applyRetry validates the policy and runs send under it. It is a
// free function because Go 1.24 does not allow generic methods.
func applyRetry[T any](policy RetryPolicy, send func() (T, error)) (T, error) {
	if !policy.Disabled && policy.BaseWait <= 0 && policy.MaxRetries > 0 {
		var zero T
		return zero, fmt.Errorf("retry policy: base wait must be positive (got %s)", policy.BaseWait)
	}
	return retry(policy, send)
}
