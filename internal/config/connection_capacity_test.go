package config

import (
	"errors"
	"github.com/13rac1/teep/internal/tlsct"
	"net/http"
	"testing"
)

func TestRetryTransportRejectsCapacityWithoutRetry(t *testing.T) {
	calls := 0
	rt := &RetryTransport{Base: rtFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, tlsct.ErrConnectionCapacity })}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, tlsct.ErrConnectionCapacity) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
