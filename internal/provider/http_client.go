package provider

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	httpClientTimeout       = 60 * time.Second
	httpMaxIdleConnsPerHost = 20
)

type rateLimitRetryPolicy struct {
	maxRetries int
	minWait    time.Duration
	maxWait    time.Duration
}

var defaultRateLimitRetryPolicy = rateLimitRetryPolicy{
	maxRetries: 5,
	minWait:    1 * time.Second,
	maxWait:    60 * time.Second,
}

func newHTTPClient() *http.Client {
	return newRetryingHTTPClient(defaultRateLimitRetryPolicy)
}

func newRetryingHTTPClient(policy rateLimitRetryPolicy) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	}
	transport = transport.Clone()
	transport.MaxIdleConnsPerHost = httpMaxIdleConnsPerHost

	rc := retryablehttp.NewClient()
	rc.HTTPClient = &http.Client{Transport: transport, Timeout: httpClientTimeout}
	// Why not the default logger: retries are reported through tflog in rateLimitBackoff.
	rc.Logger = nil
	rc.RetryMax = policy.maxRetries
	rc.RetryWaitMin = policy.minWait
	rc.RetryWaitMax = policy.maxWait
	rc.CheckRetry = retryOnTooManyRequests
	rc.Backoff = rateLimitBackoff
	// Why not the default error handler: the last 429 must reach the muumuu client so it surfaces as *muumuu.Error.
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler

	return rc.StandardClient()
}

// Why not retry transport errors or 5xx: a non-idempotent POST may already have been applied.
func retryOnTooManyRequests(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, err
	}
	return resp != nil && resp.StatusCode == http.StatusTooManyRequests, nil
}

func rateLimitBackoff(minWait, maxWait time.Duration, attemptNum int, resp *http.Response) time.Duration {
	var wait time.Duration
	if retryAfter, ok := retryAfterWait(resp); ok {
		wait = retryAfter + time.Duration(attemptNum*100)*time.Millisecond
	} else {
		wait = minWait * (1 << uint(attemptNum))
	}
	if wait > maxWait {
		wait = maxWait
	}

	if resp != nil && resp.Request != nil {
		tflog.Info(resp.Request.Context(), "Rate limited, retrying", map[string]any{
			"method":        resp.Request.Method,
			"path":          resp.Request.URL.Path,
			"retry_attempt": attemptNum + 1,
			"wait_seconds":  wait.Seconds(),
		})
	}

	return wait
}

func retryAfterWait(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}
	secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}
