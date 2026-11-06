// Package api is the only code that knows lidl.es: hosts, paths, the query
// parameters of the search API and the shape of every page and endpoint the
// CLI reads. Everything above it works with the types in this package, so
// when the site moves the fix lands here and nowhere else. It never touches
// the disk.
package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBase = "https://www.lidl.es"

type Client struct {
	HTTP *http.Client
	Base string
	UA   string
	// Debug, when set, gets one line per request and per response.
	Debug io.Writer
	// Gap is the minimum time between two requests from this client, so
	// multi-page sweeps stay polite without each caller sleeping.
	Gap time.Duration

	mu   sync.Mutex
	last time.Time
	// retryBase is the wait before the first transient retry (tests shrink it).
	retryBase time.Duration
}

// New builds a client. LIDL_BASE_URL points it at a fake server in tests.
func New(version string) *Client {
	base := DefaultBase
	if v := os.Getenv("LIDL_BASE_URL"); v != "" {
		base = strings.TrimRight(v, "/")
	}
	return &Client{
		HTTP: &http.Client{Timeout: 25 * time.Second},
		Base: base,
		// The CLI names itself; the site accepts that without a browser UA.
		UA:        "lidl-cli/" + version + " (+https://github.com/n0nuser/lidl-cli)",
		Gap:       500 * time.Millisecond,
		retryBase: defaultRetryBase,
	}
}

type request struct {
	path     string // starts with /, may carry its own query
	query    url.Values
	json     bool
	endpoint string // template for errors, never carries user input
}

type response struct {
	status int
	body   []byte
	url    *url.URL // after redirects
}

// Retry policy. One request gets at most two more attempts, and only for
// failures that a second try can plausibly fix.
const (
	retryAttempts    = 3                      // the first try plus two retries
	defaultRetryBase = 500 * time.Millisecond // doubles per attempt, up to retryCap
	retryCap         = 5 * time.Second
	// maxRetryAfter is the longest 429 wait the CLI sits through. A longer
	// one is reported with the wait, not retried.
	maxRetryAfter = 2 * time.Minute
)

// get performs one GET with a bounded retry for transient failures. 2xx and
// 404 come back as a response; everything else is classified into *Error.
//
// Retried: 5xx and transport errors (up to two more attempts, 500 ms then 1 s,
// capped at 5 s) and a 429 whose Retry-After is short (once, after the wait).
// Every attempt goes through wait, so the retries keep the request spacing.
func (c *Client) get(ctx context.Context, r request) (*response, error) {
	retriedAfter := false
	for attempt := 0; ; attempt++ {
		resp, err := c.do(ctx, r)
		if err == nil {
			return resp, nil
		}
		var e *Error
		if !errors.As(err, &e) || !e.Retryable {
			return nil, err
		}
		switch {
		case e.Kind == KindNetwork && attempt+1 < retryAttempts:
			if werr := sleepCtx(ctx, c.backoff(attempt)); werr != nil {
				return nil, werr
			}
		case e.Kind == KindBlocked && e.Status == http.StatusTooManyRequests &&
			!retriedAfter && e.RetryAfterSet && e.RetryAfter <= maxRetryAfter:
			retriedAfter = true
			if werr := sleepCtx(ctx, e.RetryAfter); werr != nil {
				return nil, werr
			}
		default:
			return nil, err
		}
	}
}

// do is one attempt: build, send, read, classify.
func (c *Client) do(ctx context.Context, r request) (*response, error) {
	u := c.Base + r.path
	if len(r.query) > 0 {
		sep := "?"
		if strings.Contains(r.path, "?") {
			sep = "&"
		}
		u += sep + r.query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UA)
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	if r.json {
		// Not application/json: the search API answers that with a 401
		// (regression test TestSearchAcceptHeader).
		req.Header.Set("Accept", "*/*")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	}

	c.wait(ctx)
	c.debugf("> GET %s", u)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, netError(r.endpoint, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, netError(r.endpoint, err)
	}
	c.debugf("< %d %s (%d bytes)", resp.StatusCode, resp.Request.URL, len(body))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		wait, known := retryAfter(resp.Header.Get("Retry-After"), time.Now())
		return nil, rateLimited(r.endpoint, wait, known)
	case resp.StatusCode == http.StatusForbidden:
		return nil, &Error{Kind: KindBlocked, Status: resp.StatusCode, Endpoint: r.endpoint,
			Msg: "lidl.es refused the request (HTTP 403). Wait a few minutes before retrying, and keep the request rate low"}
	case resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode == http.StatusNotFound:
		return &response{status: resp.StatusCode, body: body, url: resp.Request.URL}, nil
	case resp.StatusCode >= 500:
		return nil, &Error{Kind: KindNetwork, Status: resp.StatusCode, Endpoint: r.endpoint, Retryable: true,
			Msg: fmt.Sprintf("lidl.es is unavailable right now (HTTP %d on %s)", resp.StatusCode, r.endpoint)}
	}
	e := changed(r.endpoint, fmt.Sprintf("HTTP %d", resp.StatusCode), body)
	e.Status = resp.StatusCode
	return nil, e
}

// rateLimited classifies a 429. Retry-After may be absent; when it is present
// its wait goes into the message, whether the CLI will sit it out or not.
func rateLimited(endpoint string, wait time.Duration, known bool) *Error {
	e := &Error{Kind: KindBlocked, Status: http.StatusTooManyRequests, Endpoint: endpoint, Retryable: true}
	if !known {
		e.Msg = "lidl.es is rate limiting (HTTP 429). Wait a few minutes before retrying"
		return e
	}
	e.RetryAfter, e.RetryAfterSet = wait, true
	e.Msg = fmt.Sprintf("lidl.es is rate limiting (HTTP 429); retry after %s", wait.Round(time.Second))
	return e
}

// retryAfter reads a Retry-After header: delay-seconds or an HTTP-date. ok is
// false when the header is absent or unparseable, or the date already passed.
func retryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}

// backoff waits 500 ms, then 1 s, then 2 s… up to retryCap.
func (c *Client) backoff(attempt int) time.Duration {
	base := c.retryBase
	if base <= 0 {
		base = defaultRetryBase
	}
	d := base
	for i := 0; i < attempt && d < retryCap; i++ {
		d *= 2
	}
	if d > retryCap {
		return retryCap
	}
	return d
}

// sleepCtx waits d, or returns early when ctx ends.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func netError(endpoint string, err error) *Error {
	msg := "could not reach lidl.es: " + err.Error()
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		msg = "lidl.es did not answer in time"
	}
	return &Error{Kind: KindNetwork, Endpoint: endpoint, Retryable: true, Msg: msg}
}

// Reachable performs one GET of the search API and returns the classified
// error, nil when the site answered. It is what `doctor` uses.
func (c *Client) Reachable(ctx context.Context) error {
	_, err := c.get(ctx, request{path: "/q/api/search", query: searchBase("lidl", 1), json: true, endpoint: "GET /q/api/search"})
	return err
}

func (c *Client) wait(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.Gap - time.Since(c.last); !c.last.IsZero() && d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
		}
	}
	c.last = time.Now()
}

func (c *Client) debugf(format string, args ...any) {
	if c.Debug != nil {
		fmt.Fprintf(c.Debug, "lidl: "+format+"\n", args...)
	}
}

// abs turns a site path into a full URL on the public host, whatever base the
// client talks to.
func abs(path string) string {
	if path == "" || strings.HasPrefix(path, "http") {
		return path
	}
	return DefaultBase + path
}
