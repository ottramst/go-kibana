package kibana

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Defaults of [RetryPolicy].
const (
	defaultRetryMaxAttempts = 4
	defaultRetryMinBackoff  = time.Second
	defaultRetryMaxBackoff  = 30 * time.Second
)

// defaultRetryOn are the status codes retried when RetryPolicy.RetryOn is nil.
var defaultRetryOn = []int{
	http.StatusTooManyRequests,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

// maxDrain is how much of a response body is read before it is discarded
// for a retry, so that the connection can be reused.
const maxDrain = 64 << 10

// RetryPolicy configures the retries of [WithRetry]. The zero value retries
// with the defaults.
type RetryPolicy struct {
	_ [0]func()

	// MaxAttempts is the number of attempts, including the first one. Zero
	// means 4, and 1 means no retries.
	MaxAttempts int

	// MinBackoff and MaxBackoff bound the wait before a retry: before
	// retry n (from 1), the client waits a random duration between
	// MinBackoff and min(MaxBackoff, MinBackoff×2^(n-1)), unless Kibana
	// sends a Retry-After header. Zero means 1 second and 30 seconds.
	MinBackoff time.Duration
	MaxBackoff time.Duration

	// RetryOn are the status codes that are retried. Nil means 429, 502,
	// 503 and 504. Codes must be between 400 and 599.
	RetryOn []int
}

// WithRetry retries requests that fail with a transient error, such as the
// 503 Kibana returns while it starts. Without it, the client does not retry.
//
// GET, HEAD, PUT, DELETE and OPTIONS requests are retried on the status
// codes of RetryOn and on network errors. Other requests, such as POST, are
// retried only when Kibana has not processed them: on 429, and on 503 with
// a Retry-After header, if RetryOn contains them. Requests are not retried
// after TLS errors, such as an invalid or rejected certificate, when their
// context is done, or when their body cannot be sent again, such as the
// streamed file of [SavedObjectsService.Import].
//
// A Retry-After header, in seconds or as an HTTP date, sets the wait, up to
// MaxBackoff. Kibana sends "Retry-After: 30" with the 503 responses it
// returns while it starts. If a wait would end after the context's
// deadline, the client returns the last response instead of waiting.
//
// The rules go by HTTP method, so read-only endpoints that use POST,
// such as [RolesService.Query] and [SavedObjectsService.BulkGet], are only
// retried on 429, and on 503 with a Retry-After header.
//
// The timeout of the HTTP client (see [WithHTTPClient]) and the context's
// deadline span all attempts. With the defaults, a request waits at most
// 7 seconds in total between its attempts (1, 2 and 4 seconds), or 90
// seconds if every response asks for a 30-second Retry-After. [StatusService.Get] waits through retried 503
// responses before it returns the last one. A retried PUT or DELETE can
// fail with [ErrConflict] or [ErrNotFound] if Kibana processed an earlier
// attempt whose response was lost.
//
// WithRetry wraps the transport of a copy of the HTTP client, so the
// client passed to WithHTTPClient is not changed, and the order of the
// options does not matter. Don't also use a retrying transport.
func WithRetry(policy RetryPolicy) ClientOption {
	return func(c *Client) error {
		if c.retry != nil {
			return errors.New("kibana: WithRetry must only be passed once")
		}
		p, err := policy.withDefaults()
		if err != nil {
			return err
		}
		c.retry = p
		return nil
	}
}

// withDefaults returns a copy of p with the defaults set, or an error if p
// is invalid.
func (p RetryPolicy) withDefaults() (*RetryPolicy, error) {
	if p.MaxAttempts < 0 {
		return nil, errors.New("kibana: RetryPolicy.MaxAttempts must not be negative")
	}
	if p.MinBackoff < 0 || p.MaxBackoff < 0 {
		return nil, errors.New("kibana: RetryPolicy backoffs must not be negative")
	}
	if p.MaxAttempts == 0 {
		p.MaxAttempts = defaultRetryMaxAttempts
	}
	if p.MinBackoff == 0 {
		p.MinBackoff = defaultRetryMinBackoff
	}
	if p.MaxBackoff == 0 {
		p.MaxBackoff = defaultRetryMaxBackoff
	}
	if p.MinBackoff > p.MaxBackoff {
		return nil, fmt.Errorf("kibana: RetryPolicy.MinBackoff (%v) must not be greater than MaxBackoff (%v)", p.MinBackoff, p.MaxBackoff)
	}
	if p.RetryOn == nil {
		p.RetryOn = defaultRetryOn
	} else {
		for _, code := range p.RetryOn {
			if code < 400 || code > 599 {
				return nil, fmt.Errorf("kibana: RetryPolicy.RetryOn has the status code %d, want 400 to 599", code)
			}
		}
		p.RetryOn = slices.Clone(p.RetryOn)
	}
	return &p, nil
}

// retryTransport retries requests according to a RetryPolicy.
type retryTransport struct {
	base   http.RoundTripper
	policy *RetryPolicy

	// sleep waits for d, or until ctx is done.
	sleep func(ctx context.Context, d time.Duration) error
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	attemptReq := req
	for attempt := 1; ; attempt++ {
		resp, err := t.base.RoundTrip(attemptReq)
		if attempt >= t.policy.MaxAttempts || !replayable || ctx.Err() != nil || !t.shouldRetry(req.Method, resp, err) {
			return resp, err
		}
		wait, ok := t.wait(ctx, attempt, resp)
		if !ok {
			return resp, err
		}
		if resp != nil {
			_, _ = io.CopyN(io.Discard, resp.Body, maxDrain)
			_ = resp.Body.Close()
		}
		if err := t.sleep(ctx, wait); err != nil {
			return nil, err
		}
		attemptReq = req.Clone(ctx)
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("kibana: rewinding the request body for a retry: %w", err)
			}
			attemptReq.Body = body
		}
	}
}

// shouldRetry reports whether a request with the given method that got
// resp or err is retried.
func (t *retryTransport) shouldRetry(method string, resp *http.Response, err error) bool {
	idempotent := false
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		idempotent = true
	}
	if err != nil {
		return idempotent && !isTLSError(err)
	}
	if !slices.Contains(t.policy.RetryOn, resp.StatusCode) {
		return false
	}
	if idempotent {
		return true
	}
	// Kibana has not processed the request.
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusServiceUnavailable:
		_, ok := retryAfter(resp.Header.Get("Retry-After"))
		return ok
	}
	return false
}

// wait returns how long to wait before the attempt after the given one, or
// false if the wait would end after the context's deadline.
func (t *retryTransport) wait(ctx context.Context, attempt int, resp *http.Response) (time.Duration, bool) {
	var d time.Duration
	if resp != nil {
		if ra, ok := retryAfter(resp.Header.Get("Retry-After")); ok {
			d = min(ra, t.policy.MaxBackoff)
		}
	}
	if d == 0 {
		d = t.backoff(attempt)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < d {
		return 0, false
	}
	return d, true
}

// backoff returns a random duration between MinBackoff and
// min(MaxBackoff, MinBackoff×2^(attempt-1)).
func (t *retryTransport) backoff(attempt int) time.Duration {
	lo, hi := t.policy.MinBackoff, t.policy.MaxBackoff
	ceil := lo
	for range attempt - 1 {
		if ceil >= hi/2 {
			ceil = hi
			break
		}
		ceil *= 2
	}
	ceil = min(ceil, hi)
	if ceil <= lo {
		return lo
	}
	return lo + rand.N(ceil-lo+1)
}

// maxRetryAfter caps parsed Retry-After values, to avoid overflows.
const maxRetryAfter = 24 * time.Hour

// retryAfter parses a Retry-After header, in seconds or as an HTTP date. It
// returns false if the header is missing or invalid, and 0 for a time that
// is not in the future.
func retryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if s, err := strconv.ParseInt(v, 10, 64); err == nil {
		switch {
		case s < 0:
			return 0, false
		case s > int64(maxRetryAfter/time.Second):
			return maxRetryAfter, true
		}
		return time.Duration(s) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	return min(max(time.Until(t), 0), maxRetryAfter), true
}

// isTLSError reports whether err is a TLS or certificate error, which a
// retry does not fix.
func isTLSError(err error) bool {
	var (
		unknownAuthority x509.UnknownAuthorityError
		invalid          x509.CertificateInvalidError
		hostname         x509.HostnameError
		verification     *tls.CertificateVerificationError
		recordHeader     tls.RecordHeaderError
	)
	if errors.As(err, &unknownAuthority) || errors.As(err, &invalid) || errors.As(err, &hostname) ||
		errors.As(err, &verification) || errors.As(err, &recordHeader) {
		return true
	}
	// A TLS alert from the server, such as a rejected client certificate.
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "remote error"
}

// sleepContext waits for d, or until ctx is done.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
