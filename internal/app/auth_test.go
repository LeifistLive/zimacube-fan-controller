package app

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Regression: bcrypt rejects a password over 72 bytes. newAuth used to catch
// that error and just leave auth disabled, i.e. the whole dashboard open —
// exactly backwards from what a broken ADMIN_PASSWORD should do.
func TestNewAuthFailsOnPasswordOver72Bytes(t *testing.T) {
	tooLong := strings.Repeat("a", 73)
	if _, err := newAuth("admin", tooLong); err == nil {
		t.Fatal("a 73-byte password must make newAuth fail, not disable auth")
	}
}

// New() must refuse to start rather than run with authentication silently
// disabled when ADMIN_PASSWORD cannot be hashed.
func TestNewFailsToStartOnPasswordOver72Bytes(t *testing.T) {
	tooLong := strings.Repeat("a", 73)
	_, err := New(Config{
		DataDir:       t.TempDir(),
		AdminUser:     "admin",
		AdminPassword: tooLong,
		I2CAddress:    "0x69",
		VarINI:        "/nonexistent/var.ini",
		DisksINI:      "/nonexistent/disks.ini",
	})
	if err == nil {
		t.Fatal("New() must fail rather than start with login disabled")
	}
}

func TestSweepExpiredRemovesOldSessionsAndStaleLimiters(t *testing.T) {
	a, err := newAuth("admin", "secret")
	if err != nil {
		t.Fatalf("newAuth: %v", err)
	}

	a.mu.Lock()
	a.sessions["expired"] = time.Now().Add(-time.Hour)
	a.sessions["valid"] = time.Now().Add(time.Hour)
	a.mu.Unlock()

	gate := a.loginLimiterFor("203.0.113.1")
	gate.mu.Lock()
	gate.next = time.Now().Add(-2 * staleLoginLimiter)
	gate.mu.Unlock()
	a.loginLimiterFor("203.0.113.2") // fresh, must survive the sweep

	a.sweepExpired()

	a.mu.Lock()
	_, expiredStillThere := a.sessions["expired"]
	_, validStillThere := a.sessions["valid"]
	a.mu.Unlock()
	if expiredStillThere {
		t.Error("expired session was not removed")
	}
	if !validStillThere {
		t.Error("valid session was wrongly removed")
	}

	a.loginLimitersMu.Lock()
	_, staleStillThere := a.loginLimiters["203.0.113.1"]
	_, freshStillThere := a.loginLimiters["203.0.113.2"]
	a.loginLimitersMu.Unlock()
	if staleStillThere {
		t.Error("stale login rate gate was not removed")
	}
	if !freshStillThere {
		t.Error("fresh login rate gate was wrongly removed")
	}
}

func TestRequestIsSecure(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	if requestIsSecure(plain) {
		t.Error("plain HTTP request should not be considered secure")
	}

	viaProxy := httptest.NewRequest(http.MethodGet, "/", nil)
	viaProxy.Header.Set("X-Forwarded-Proto", "https")
	if !requestIsSecure(viaProxy) {
		t.Error("X-Forwarded-Proto: https should be considered secure")
	}

	viaTLS := httptest.NewRequest(http.MethodGet, "/", nil)
	viaTLS.TLS = &tls.ConnectionState{}
	if !requestIsSecure(viaTLS) {
		t.Error("direct TLS should be considered secure")
	}
}

func TestClientIPStripsPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.10:54321"
	if got := clientIP(r); got != "192.0.2.10" {
		t.Fatalf("clientIP = %q, expected 192.0.2.10", got)
	}
}

// Regression: a single shared login rate gate let one client's attempts
// lock every other client out of logging in, including the real admin.
func TestLoginRateLimitIsPerClientIP(t *testing.T) {
	handler := newTestApp(t, "secret").Routes()

	first := httptest.NewRequest(http.MethodPost, "/login", nil)
	first.RemoteAddr = "192.0.2.1:1111"
	first.SetBasicAuth("admin", "wrong")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, first)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("first client, wrong password = %d, expected 401", recorder.Code)
	}

	// Immediately after, a different client IP must not be rate-limited by
	// the first client's attempt.
	second := httptest.NewRequest(http.MethodPost, "/login", nil)
	second.RemoteAddr = "192.0.2.2:2222"
	second.SetBasicAuth("admin", "secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, second)
	if recorder.Code != http.StatusOK {
		t.Fatalf("second client (different IP) = %d, expected 200", recorder.Code)
	}
}
