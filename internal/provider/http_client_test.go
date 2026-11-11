package provider

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryingHTTPClient_RetriesOnlyTooManyRequests(t *testing.T) {
	tests := []struct {
		name         string
		statuses     []int
		wantStatus   int
		wantAttempts int32
	}{
		{name: "429 then success", statuses: []int{429, 429, 200}, wantStatus: 200, wantAttempts: 3},
		{name: "429 exhausts retries and returns last response", statuses: []int{429, 429, 429, 429}, wantStatus: 429, wantAttempts: 3},
		{name: "5xx is not retried", statuses: []int{500, 200}, wantStatus: 500, wantAttempts: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n := attempts.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(tt.statuses[n-1])
			}))
			t.Cleanup(srv.Close)

			c := newRetryingHTTPClient(rateLimitRetryPolicy{maxRetries: 2, minWait: time.Millisecond, maxWait: 10 * time.Millisecond})
			resp, err := c.Post(srv.URL, "application/json", nil)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := attempts.Load(); got != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got, tt.wantAttempts)
			}
		})
	}
}

func TestRateLimitBackoff_HonoursRetryAfterWithinMaxWait(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"5"}}}
	if got := rateLimitBackoff(time.Second, time.Minute, 0, resp); got != 5*time.Second {
		t.Errorf("wait = %s, want 5s", got)
	}
	if got := rateLimitBackoff(time.Second, 3*time.Second, 0, resp); got != 3*time.Second {
		t.Errorf("wait = %s, want capped 3s", got)
	}
	if got := rateLimitBackoff(time.Second, time.Minute, 2, &http.Response{}); got != 4*time.Second {
		t.Errorf("wait = %s, want exponential 4s", got)
	}
}
