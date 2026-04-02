package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestLoopDrainMiddleware(t *testing.T) {
	var onLimitCalls atomic.Int32
	onLimit := func() {
		onLimitCalls.Add(1)
	}

	h := loopDrainMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), 3, onLimit)

	for i := 0; i < 2; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
	if onLimitCalls.Load() != 0 {
		t.Fatalf("onLimit before 3rd request: %d", onLimitCalls.Load())
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if onLimitCalls.Load() != 1 {
		t.Fatalf("onLimit after 3rd: got %d, want 1", onLimitCalls.Load())
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if onLimitCalls.Load() != 1 {
		t.Fatalf("onLimit should not fire again after N; got %d", onLimitCalls.Load())
	}
}

func TestLoopDrainMiddlewareDisabled(t *testing.T) {
	called := false
	h := loopDrainMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), 0, func() {
		called = true
	})
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if called {
		t.Fatal("onLimit should not run when loopMax <= 0")
	}
}
