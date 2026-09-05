package proxy

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/13rac1/teep/internal/attestation"
	"github.com/13rac1/teep/internal/config"
	"github.com/13rac1/teep/internal/provider"
	"github.com/13rac1/teep/internal/tlsct"
)

type capacityAttester struct {
	calls     atomic.Int32
	exhausted atomic.Bool
}

func (a *capacityAttester) FetchAttestation(context.Context, string, attestation.Nonce) (*attestation.RawAttestation, error) {
	a.calls.Add(1)
	if a.exhausted.Load() {
		return nil, tlsct.ErrConnectionCapacity
	}
	return nil, errors.New("test attestation unavailable")
}

func TestAuthorizationAttestationCapacityRecovery(t *testing.T) {
	s, err := New(&config.Config{Offline: true, Providers: map[string]*config.Provider{"neardirect": {Name: "neardirect", BaseURL: "https://test.near.ai"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	route, err := provider.NewResolvedRoute("https://test.near.ai", "")
	if err != nil {
		t.Fatal(err)
	}
	key, err := route.AuthorizationKey("neardirect", "test")
	if err != nil {
		t.Fatal(err)
	}
	attester := &capacityAttester{}
	attester.exhausted.Store(true)
	prov := s.providers["neardirect"]
	prov.ResolveRoute, prov.StaticRoute, prov.Attester = nil, route, attester
	input := &authorizedRequest{provider: prov, route: route, key: key}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			rec := newInferenceRecorder()
			s.handleAuthorizedEndpoint(t.Context(), rec, input)
			if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "1" {
				t.Errorf("capacity status=%d backoff=%q", rec.Code, rec.Header().Get("Retry-After"))
			}
		})
	}
	wg.Wait()
	if _, blocked := s.negCache.ActiveInfo(key.ProviderName(), key.SingleflightKey()); blocked {
		t.Fatal("capacity exhaustion entered negative cache")
	}
	before := attester.calls.Load()
	attester.exhausted.Store(false)
	_, _, err = s.loadAuthorization(t.Context(), prov, route, key)
	if err == nil || attester.calls.Load() != before+1 {
		t.Fatal("restored capacity did not permit new attestation")
	}
	if _, blocked := s.negCache.ActiveInfo(key.ProviderName(), key.SingleflightKey()); !blocked {
		t.Fatal("ordinary attestation failure did not enter negative cache")
	}
}
