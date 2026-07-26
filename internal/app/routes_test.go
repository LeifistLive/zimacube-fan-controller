package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestApp builds an App with auth disabled unless password is non-empty
// (empty ADMIN_PASSWORD means auth is off entirely, see auth.go), which is
// what almost every test below wants so it can exercise the route/handler
// logic without also having to log in first.
func newTestApp(t *testing.T, password string) *App {
	t.Helper()
	service, err := New(Config{
		DataDir:       t.TempDir(),
		AdminUser:     "admin",
		AdminPassword: password,
		I2CAddress:    "0x69",
		VarINI:        "/nonexistent/var.ini",
		DisksINI:      "/nonexistent/disks.ini",
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

// loginSession logs in with HTTP Basic Auth (the same mechanism the login
// page and fanctl use) and returns the resulting session cookie value.
func loginSession(t *testing.T, handler http.Handler, user, password string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	request.SetBasicAuth(user, password)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login = %d, expected 200", recorder.Code)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie.Value
		}
	}
	t.Fatal("login did not return a session cookie")
	return ""
}

func sessionHeaders(session string) map[string]string {
	return map[string]string{"Cookie": sessionCookieName + "=" + session}
}

func TestReadEndpointsAreOpenWhenAuthDisabled(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, nil).Code; code != http.StatusOK {
			t.Errorf("GET %s = %d, expected 200", target, code)
		}
	}
	if code := do(t, handler, http.MethodGet, "/api/health", nil).Code; code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/health without a controller = %d, expected 503", code)
	}
}

func TestReadEndpointsRequireSessionWhenAuthEnabled(t *testing.T) {
	service := newTestApp(t, "secret")
	handler := service.Routes()

	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, nil).Code; code != http.StatusUnauthorized {
			t.Errorf("GET %s without a session = %d, expected 401", target, code)
		}
	}

	session := loginSession(t, handler, "admin", "secret")
	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, sessionHeaders(session)).Code; code != http.StatusOK {
			t.Errorf("GET %s with a session = %d, expected 200", target, code)
		}
	}
}

// Regression: /api/health must stay reachable without a session even with
// login enabled, otherwise Docker healthcheck and external monitoring
// (e.g. Uptime Kuma) can no longer poll the service.
func TestHealthStaysOpenWhenAuthEnabled(t *testing.T) {
	handler := newTestApp(t, "secret").Routes()
	if code := do(t, handler, http.MethodGet, "/api/health", nil).Code; code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/health without a session = %d, expected 503 (not 401)", code)
	}
}

func TestIndexRedirectsToLoginWhenUnauthenticated(t *testing.T) {
	handler := newTestApp(t, "secret").Routes()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusFound {
		t.Fatalf("GET / without a session = %d, expected 302", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/login" {
		t.Fatalf("redirect target = %q, expected /login", location)
	}
}

func TestLoginRejectsWrongCredentials(t *testing.T) {
	// One fresh App/Routes pair per case: the login endpoint has its own,
	// stricter rate limit (loginRateInterval), which a second call on the
	// same instance would otherwise answer with 429 instead of the 401
	// response checked here.
	wrongPassword := httptest.NewRequest(http.MethodPost, "/login", nil)
	wrongPassword.SetBasicAuth("admin", "wrong")
	recorder := httptest.NewRecorder()
	newTestApp(t, "secret").Routes().ServeHTTP(recorder, wrongPassword)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, expected 401", recorder.Code)
	}

	wrongUser := httptest.NewRequest(http.MethodPost, "/login", nil)
	wrongUser.SetBasicAuth("someone-else", "secret")
	recorder = httptest.NewRecorder()
	newTestApp(t, "secret").Routes().ServeHTTP(recorder, wrongUser)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong username = %d, expected 401", recorder.Code)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	handler := newTestApp(t, "secret").Routes()
	session := loginSession(t, handler, "admin", "secret")

	if code := do(t, handler, http.MethodGet, "/api/status", sessionHeaders(session)).Code; code != http.StatusOK {
		t.Fatalf("session before logout = %d, expected 200", code)
	}

	logout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logout.Header.Set("Cookie", sessionCookieName+"="+session)
	handler.ServeHTTP(httptest.NewRecorder(), logout)

	if code := do(t, handler, http.MethodGet, "/api/status", sessionHeaders(session)).Code; code != http.StatusUnauthorized {
		t.Fatalf("session after logout = %d, expected 401", code)
	}
}

func TestStaticAssets(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	index := do(t, handler, http.MethodGet, "/", nil)
	if index.Code != http.StatusOK {
		t.Fatalf("GET / = %d", index.Code)
	}
	if !strings.Contains(index.Body.String(), "/app.js") {
		t.Error("index does not reference /app.js")
	}
	for target, contentType := range map[string]string{"/app.css": "text/css", "/app.js": "text/javascript"} {
		response := do(t, handler, http.MethodGet, target, nil)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s = %d", target, response.Code)
			continue
		}
		if !strings.Contains(response.Header().Get("Content-Type"), contentType) {
			t.Errorf("GET %s has Content-Type %q", target, response.Header().Get("Content-Type"))
		}
	}
}

// Regression: "GET /" was a catch-all and returned HTML for every path.
func TestUnknownPathIsNotFound(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	if code := do(t, handler, http.MethodGet, "/does-not-exist", nil).Code; code != http.StatusNotFound {
		t.Fatalf("unknown path = %d, expected 404", code)
	}
}

func TestContentSecurityPolicyHasNoUnsafeInline(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	policy := do(t, handler, http.MethodGet, "/", nil).Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("Content-Security-Policy is missing")
	}
	if strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("policy allows unsafe-inline: %q", policy)
	}
}

func TestWriteEndpointsRequireSessionWhenAuthEnabled(t *testing.T) {
	handler := newTestApp(t, "secret").Routes()

	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusUnauthorized {
		t.Errorf("without a session = %d, expected 401", code)
	}

	session := loginSession(t, handler, "admin", "secret")
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", sessionHeaders(session)).Code; code != http.StatusAccepted {
		t.Errorf("with a session = %d, expected 202", code)
	}
}

func TestWriteEndpointsBlockCrossSite(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	foreign := map[string]string{"Origin": "http://evil.example"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", foreign).Code; code != http.StatusForbidden {
		t.Errorf("foreign origin = %d, expected 403", code)
	}
	fetch := map[string]string{"Sec-Fetch-Site": "cross-site"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", fetch).Code; code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site cross-site = %d, expected 403", code)
	}
	own := map[string]string{"Origin": "http://example.com", "Sec-Fetch-Site": "same-origin"}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", own).Code; code != http.StatusAccepted {
		t.Errorf("own origin = %d, expected 202", code)
	}
}

func TestManualPercentValidation(t *testing.T) {
	// Each request gets its own App instance: since the rate limit is per
	// category (one write per second for "override"/"test"), a second call
	// on the same instance would otherwise return 429 instead of the
	// validation response checked here.
	for _, target := range []string{"/api/fan/0", "/api/fan/101", "/api/fan/abc", "/api/test/0"} {
		handler := newTestApp(t, "").Routes()
		if code := do(t, handler, http.MethodPost, target, nil).Code; code != http.StatusBadRequest {
			t.Errorf("POST %s = %d, expected 400", target, code)
		}
	}
	handler := newTestApp(t, "").Routes()
	if code := do(t, handler, http.MethodPost, "/api/fan/75", nil).Code; code != http.StatusAccepted {
		t.Errorf("POST /api/fan/75 = %d, expected 202", code)
	}
}

func TestProfileSwitch(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	if code := do(t, handler, http.MethodPost, "/api/profile/silent", nil).Code; code != http.StatusAccepted {
		t.Fatalf("profile switch = %d, expected 202", code)
	}
	service.mu.RLock()
	active := service.runtime.ActiveProfile
	service.mu.RUnlock()
	if active != "silent" {
		t.Fatalf("active profile = %q", active)
	}

	// Fresh instance: the "profile" category's rate limit (one write per
	// second) would otherwise answer a second call on the same instance
	// with 429 instead of 404.
	other := newTestApp(t, "").Routes()
	if code := do(t, other, http.MethodPost, "/api/profile/doesnotexist", nil).Code; code != http.StatusNotFound {
		t.Fatalf("unknown profile = %d, expected 404", code)
	}
}

func TestOverridePersistsManualMode(t *testing.T) {
	service := newTestApp(t, "")
	if err := service.setOverride(Override{Mode: ModeManual, Percent: 42}); err != nil {
		t.Fatalf("setOverride: %v", err)
	}

	restored := service.loadOverride()
	if restored.Mode != ModeManual || restored.Percent != 42 {
		t.Fatalf("manual mode not saved: %+v", restored)
	}

	if err := service.setOverride(Override{}); err != nil {
		t.Fatalf("setOverride: %v", err)
	}
	if cleared := service.loadOverride(); cleared.Mode != "" {
		t.Fatalf("automatic mode not saved: %+v", cleared)
	}
}

// blockPersistPath places a non-empty directory at dataDir/name, so that any
// attempt to write or remove that file (SaveJSON's rename, Remove) fails
// portably on both Windows and Linux without touching file permissions.
func blockPersistPath(t *testing.T, dataDir, name string) {
	t.Helper()
	target := filepath.Join(dataDir, name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("blockPersistPath Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "blocker"), []byte("x"), 0o600); err != nil {
		t.Fatalf("blockPersistPath WriteFile: %v", err)
	}
}

func TestSetOverrideFailsWhenPersistFails(t *testing.T) {
	dataDir := t.TempDir()
	service, err := New(Config{
		DataDir:    dataDir,
		I2CAddress: "0x69",
		VarINI:     "/nonexistent/var.ini",
		DisksINI:   "/nonexistent/disks.ini",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blockPersistPath(t, dataDir, overrideFile)

	before := service.loadOverride()
	if err := service.setOverride(Override{Mode: ModeManual, Percent: 42}); err == nil {
		t.Fatal("expected an error when override.json cannot be written")
	}

	service.mu.RLock()
	current := service.override
	service.mu.RUnlock()
	if current != before {
		t.Fatalf("internal state changed despite persistence failure: %+v", current)
	}
}

func TestManualEndpointReturns500OnPersistFailure(t *testing.T) {
	dataDir := t.TempDir()
	service, err := New(Config{
		DataDir:    dataDir,
		I2CAddress: "0x69",
		VarINI:     "/nonexistent/var.ini",
		DisksINI:   "/nonexistent/disks.ini",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blockPersistPath(t, dataDir, overrideFile)
	handler := service.Routes()

	if code := do(t, handler, http.MethodPost, "/api/fan/75", nil).Code; code != http.StatusInternalServerError {
		t.Fatalf("POST /api/fan/75 with blocked persistence = %d, expected 500", code)
	}
}

func TestFanTestRejectsUnsafeValue(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	// Parity check active + a low test value must be rejected with 409
	// regardless of the hardware (which doesn't exist in tests), since the
	// check runs before the actual I2C write.
	service.mu.Lock()
	service.status.TemperatureValid = true
	service.status.ArrayOperation = "parity-check"
	service.mu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/5", nil).Code; code != http.StatusConflict {
		t.Fatalf("unsafe test value during parity check = %d, expected 409", code)
	}
}

func TestFanTestRejectsConcurrentRuns(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	if !service.testMu.TryLock() {
		t.Fatal("could not pre-lock testMu")
	}
	defer service.testMu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/50", nil).Code; code != http.StatusConflict {
		t.Fatalf("test while a test is already running = %d, expected 409", code)
	}
}

func TestFanTestCooldown(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	service.testMu.Lock()
	service.testCooldownUntil = time.Now().Add(time.Minute)
	service.testMu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/50", nil).Code; code != http.StatusTooManyRequests {
		t.Fatalf("test during cooldown = %d, expected 429", code)
	}
}

func TestWriteEndpointsAreRateLimitedPerCategory(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusAccepted {
		t.Fatalf("first call = %d, expected 202", code)
	}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusTooManyRequests {
		t.Fatalf("second call within 1s = %d, expected 429", code)
	}
	// A different category ("profile") must not be affected by the
	// "override" category's limit.
	if code := do(t, handler, http.MethodPost, "/api/profile/silent", nil).Code; code != http.StatusAccepted {
		t.Fatalf("different category right after = %d, expected 202", code)
	}
}

func TestConfigUpdateRejectsUnknownFields(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	body := `{"active_profile":"balanced","profiles":{},"unknown_field":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, expected 400", recorder.Code)
	}
}

func TestConfigUpdateRejectsTrailingData(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	body := `{"active_profile":"balanced","profiles":{}}{"extra":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("data after the JSON object = %d, expected 400", recorder.Code)
	}
}

// TestConcurrentConfigWritesDoNotInterleave calls the handler directly
// (not through Routes()/guardWrite) to check configMu independently of
// rate limiting: multiple concurrent config writes must not interleave,
// config.json must end up matching exactly one of the requests.
func TestConcurrentConfigWritesDoNotInterleave(t *testing.T) {
	service := newTestApp(t, "")

	const attempts = 20
	var wg sync.WaitGroup
	codes := make([]int, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"active_profile":"balanced","profiles":{"balanced":{"curve":[{"temperature":0,"percent":%d}],"array_boost_percent":100,"emergency_temperature":52,"emergency_percent":100}}}`, 60+i%20)
			request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
			recorder := httptest.NewRecorder()
			service.handleConfigUpdate(recorder, request)
			codes[i] = recorder.Code
		}(i)
	}
	wg.Wait()

	for _, code := range codes {
		if code != http.StatusAccepted {
			t.Fatalf("unexpected status code during concurrent config update: %d", code)
		}
	}

	var onDisk RuntimeConfig
	if err := service.store.LoadJSON(configFile, &onDisk); err != nil {
		t.Fatalf("could not read config.json: %v", err)
	}
	service.mu.RLock()
	inMemory := service.runtime
	service.mu.RUnlock()
	if onDisk.Profiles["balanced"].Curve[0].Percent != inMemory.Profiles["balanced"].Curve[0].Percent {
		t.Fatalf("config.json (%v) differs from in-memory state (%v), writes interleaved",
			onDisk.Profiles["balanced"], inMemory.Profiles["balanced"])
	}
}

func TestHealthReportsIndividualChecks(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	// Broken active profile -> config: false.
	service.mu.Lock()
	service.runtime.ActiveProfile = "does-not-exist"
	service.mu.Unlock()

	recorder := do(t, handler, http.MethodGet, "/api/health", nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("health with invalid profile = %d, expected 503", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("health response is not JSON: %v", err)
	}
	if config, _ := body["config"].(bool); config {
		t.Fatalf("config should be false: %+v", body)
	}

	// Simulate a storage error.
	service.setStorageOK(storageConfig, false)
	recorder = do(t, handler, http.MethodGet, "/api/health", nil)
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("health response is not JSON: %v", err)
	}
	if storage, _ := body["storage"].(bool); storage {
		t.Fatalf("storage should be false: %+v", body)
	}
}

// Regression: a single combined storageOK flag let an unrelated successful
// write mask an earlier, still-unresolved failure in a different category.
func TestStorageHealthDoesNotMaskUnrelatedCategoryFailure(t *testing.T) {
	service := newTestApp(t, "")

	service.setStorageOK(storageConfig, false)
	if service.storageHealthy() {
		t.Fatal("storageHealthy should be false after a failed config write")
	}

	service.setStorageOK(storageHistory, true)
	if service.storageHealthy() {
		t.Fatal("a successful history write must not mask an open config failure")
	}

	service.setStorageOK(storageConfig, true)
	if !service.storageHealthy() {
		t.Fatal("storageHealthy should be true once config succeeds again too")
	}
}
