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
		t.Fatal("ein 73-Byte-Passwort muss newAuth fehlschlagen lassen, nicht auth deaktivieren")
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
		VarINI:        "/nicht/vorhanden/var.ini",
		DisksINI:      "/nicht/vorhanden/disks.ini",
	})
	if err == nil {
		t.Fatal("New() muss fehlschlagen statt mit deaktiviertem Login zu starten")
	}
}

func TestSweepExpiredRemovesOldSessionsAndStaleLimiters(t *testing.T) {
	a, err := newAuth("admin", "geheim")
	if err != nil {
		t.Fatalf("newAuth: %v", err)
	}

	a.mu.Lock()
	a.sessions["abgelaufen"] = time.Now().Add(-time.Hour)
	a.sessions["gueltig"] = time.Now().Add(time.Hour)
	a.mu.Unlock()

	gate := a.loginLimiterFor("203.0.113.1")
	gate.mu.Lock()
	gate.next = time.Now().Add(-2 * staleLoginLimiter)
	gate.mu.Unlock()
	a.loginLimiterFor("203.0.113.2") // fresh, must survive the sweep

	a.sweepExpired()

	a.mu.Lock()
	_, expiredStillThere := a.sessions["abgelaufen"]
	_, validStillThere := a.sessions["gueltig"]
	a.mu.Unlock()
	if expiredStillThere {
		t.Error("abgelaufene Session wurde nicht entfernt")
	}
	if !validStillThere {
		t.Error("gültige Session wurde faelschlich entfernt")
	}

	a.loginLimitersMu.Lock()
	_, staleStillThere := a.loginLimiters["203.0.113.1"]
	_, freshStillThere := a.loginLimiters["203.0.113.2"]
	a.loginLimitersMu.Unlock()
	if staleStillThere {
		t.Error("veralteter Login-Rate-Gate wurde nicht entfernt")
	}
	if !freshStillThere {
		t.Error("frischer Login-Rate-Gate wurde faelschlich entfernt")
	}
}

func TestRequestIsSecure(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	if requestIsSecure(plain) {
		t.Error("einfacher HTTP-Request sollte nicht als sicher gelten")
	}

	viaProxy := httptest.NewRequest(http.MethodGet, "/", nil)
	viaProxy.Header.Set("X-Forwarded-Proto", "https")
	if !requestIsSecure(viaProxy) {
		t.Error("X-Forwarded-Proto: https sollte als sicher gelten")
	}

	viaTLS := httptest.NewRequest(http.MethodGet, "/", nil)
	viaTLS.TLS = &tls.ConnectionState{}
	if !requestIsSecure(viaTLS) {
		t.Error("direktes TLS sollte als sicher gelten")
	}
}

func TestClientIPStripsPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.10:54321"
	if got := clientIP(r); got != "192.0.2.10" {
		t.Fatalf("clientIP = %q, erwartet 192.0.2.10", got)
	}
}

// Regression: a single shared login rate gate let one client's attempts
// lock every other client out of logging in, including the real admin.
func TestLoginRateLimitIsPerClientIP(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()

	first := httptest.NewRequest(http.MethodPost, "/login", nil)
	first.RemoteAddr = "192.0.2.1:1111"
	first.SetBasicAuth("admin", "falsch")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, first)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("erster Client, falsches Passwort = %d, erwartet 401", recorder.Code)
	}

	// Immediately after, a different client IP must not be rate-limited by
	// the first client's attempt.
	second := httptest.NewRequest(http.MethodPost, "/login", nil)
	second.RemoteAddr = "192.0.2.2:2222"
	second.SetBasicAuth("admin", "geheim")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, second)
	if recorder.Code != http.StatusOK {
		t.Fatalf("zweiter Client (andere IP) = %d, erwartet 200", recorder.Code)
	}
}
