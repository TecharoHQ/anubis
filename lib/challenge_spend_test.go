package lib

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/store"
)

type spendFailureStore struct{ store.Interface }

func (s spendFailureStore) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	var c challenge.Challenge
	if json.Unmarshal(value, &c) == nil && c.Spent {
		return store.ErrCantEncode
	}
	return s.Interface.Set(ctx, key, value, expiry)
}

func TestChallengeSpendIsExclusive(t *testing.T) {
	srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	t.Cleanup(ts.Close)
	c := makeChallenge(t, ts, httpClient(t))
	j := store.JSON[challenge.Challenge]{Underlying: srv.store}
	stored, err := j.Get(t.Context(), "challenge:"+c.ID)
	if err != nil {
		t.Fatal(err)
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for range 32 {
		wg.Go(func() {
			<-gate
			req := httptest.NewRequest("GET", "/?redir=/&id="+c.ID+"&nonce=0&elapsedTime=1&response="+internal.SHA256sum(stored.RandomData+"0"), nil)
			req.Header.Set("X-Real-IP", "192.0.2.1")
			req.AddCookie(&http.Cookie{Name: srv.cookieName(anubis.TestCookieName), Value: c.ID})
			rec := httptest.NewRecorder()
			srv.PassChallenge(rec, req)
			if authCookie(srv, rec.Result()) != nil {
				accepted.Add(1)
			}
		})
	}
	close(gate)
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("challenge minted %d cookies", got)
	}
}

func TestChallengeSpendWriteFailureRejectsClearance(t *testing.T) {
	srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	t.Cleanup(ts.Close)
	cli := httpClient(t)
	c := makeChallenge(t, ts, cli)
	srv.store = spendFailureStore{srv.store}
	resp := handleChallengeZeroDifficulty(t, ts, cli, c)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 400 || authCookie(srv, resp) != nil {
		t.Fatalf("failed spend accepted: %d", resp.StatusCode)
	}
}
