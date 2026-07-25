package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestApp(t *testing.T, token string) *App {
	t.Helper()
	service, err := New(Config{
		DataDir:    t.TempDir(),
		APIToken:   token,
		I2CAddress: "0x69",
		VarINI:     "/nicht/vorhanden/var.ini",
		DisksINI:   "/nicht/vorhanden/disks.ini",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return service
}

func do(t *testing.T, handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestReadEndpointsAreOpen(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()

	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, nil).Code; code != http.StatusOK {
			t.Errorf("GET %s = %d, erwartet 200", target, code)
		}
	}
	if code := do(t, handler, http.MethodGet, "/api/health", nil).Code; code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/health ohne Controller = %d, erwartet 503", code)
	}
}

func TestStaticAssets(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	index := do(t, handler, http.MethodGet, "/", nil)
	if index.Code != http.StatusOK {
		t.Fatalf("GET / = %d", index.Code)
	}
	if !strings.Contains(index.Body.String(), "/app.js") {
		t.Error("index verweist nicht auf /app.js")
	}
	for target, contentType := range map[string]string{"/app.css": "text/css", "/app.js": "text/javascript"} {
		response := do(t, handler, http.MethodGet, target, nil)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s = %d", target, response.Code)
			continue
		}
		if !strings.Contains(response.Header().Get("Content-Type"), contentType) {
			t.Errorf("GET %s hat Content-Type %q", target, response.Header().Get("Content-Type"))
		}
	}
}

// Regression: "GET /" war ein Catch-all und lieferte für jeden Pfad HTML.
func TestUnknownPathIsNotFound(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	if code := do(t, handler, http.MethodGet, "/gibt-es-nicht", nil).Code; code != http.StatusNotFound {
		t.Fatalf("unbekannter Pfad = %d, erwartet 404", code)
	}
}

func TestContentSecurityPolicyHasNoUnsafeInline(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	policy := do(t, handler, http.MethodGet, "/", nil).Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("Content-Security-Policy fehlt")
	}
	if strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("Policy erlaubt unsafe-inline: %q", policy)
	}
}

func TestWriteEndpointsRequireToken(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()

	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusUnauthorized {
		t.Errorf("ohne Token = %d, erwartet 401", code)
	}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", map[string]string{"X-API-Token": "falsch"}).Code; code != http.StatusUnauthorized {
		t.Errorf("falscher Token = %d, erwartet 401", code)
	}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", map[string]string{"X-API-Token": "geheim"}).Code; code != http.StatusAccepted {
		t.Errorf("richtiger Token = %d, erwartet 202", code)
	}
}

func TestWriteEndpointsBlockCrossSite(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	fremd := map[string]string{"Origin": "http://boese.example"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", fremd).Code; code != http.StatusForbidden {
		t.Errorf("fremder Origin = %d, erwartet 403", code)
	}
	fetch := map[string]string{"Sec-Fetch-Site": "cross-site"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", fetch).Code; code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site cross-site = %d, erwartet 403", code)
	}
	eigen := map[string]string{"Origin": "http://example.com", "Sec-Fetch-Site": "same-origin"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", eigen).Code; code != http.StatusAccepted {
		t.Errorf("eigener Origin = %d, erwartet 202", code)
	}
}

func TestManualPercentValidation(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	for _, target := range []string{"/api/fan/0", "/api/fan/101", "/api/fan/abc", "/api/test/0"} {
		if code := do(t, handler, http.MethodPost, target, nil).Code; code != http.StatusBadRequest {
			t.Errorf("POST %s = %d, erwartet 400", target, code)
		}
	}
	if code := do(t, handler, http.MethodPost, "/api/fan/75", nil).Code; code != http.StatusAccepted {
		t.Errorf("POST /api/fan/75 = %d, erwartet 202", code)
	}
}

func TestProfileSwitch(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	if code := do(t, handler, http.MethodPost, "/api/profile/silent", nil).Code; code != http.StatusAccepted {
		t.Fatalf("Profilwechsel = %d, erwartet 202", code)
	}
	service.mu.RLock()
	active := service.runtime.ActiveProfile
	service.mu.RUnlock()
	if active != "silent" {
		t.Fatalf("aktives Profil = %q", active)
	}
	if code := do(t, handler, http.MethodPost, "/api/profile/gibtsnicht", nil).Code; code != http.StatusNotFound {
		t.Fatalf("unbekanntes Profil = %d, erwartet 404", code)
	}
}

func TestOverridePersistsManualMode(t *testing.T) {
	service := newTestApp(t, "")
	service.setOverride(Override{Mode: ModeManual, Percent: 42})

	restored := service.loadOverride()
	if restored.Mode != ModeManual || restored.Percent != 42 {
		t.Fatalf("manueller Modus nicht gespeichert: %+v", restored)
	}

	service.setOverride(Override{})
	if cleared := service.loadOverride(); cleared.Mode != "" {
		t.Fatalf("Automatik nicht gespeichert: %+v", cleared)
	}
}
