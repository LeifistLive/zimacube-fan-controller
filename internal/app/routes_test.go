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
		VarINI:        "/nicht/vorhanden/var.ini",
		DisksINI:      "/nicht/vorhanden/disks.ini",
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
		t.Fatalf("Login = %d, erwartet 200", recorder.Code)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie.Value
		}
	}
	t.Fatal("Login lieferte keine Session-Cookie")
	return ""
}

func sessionHeaders(session string) map[string]string {
	return map[string]string{"Cookie": sessionCookieName + "=" + session}
}

func TestReadEndpointsAreOpenWhenAuthDisabled(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, nil).Code; code != http.StatusOK {
			t.Errorf("GET %s = %d, erwartet 200", target, code)
		}
	}
	if code := do(t, handler, http.MethodGet, "/api/health", nil).Code; code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/health ohne Controller = %d, erwartet 503", code)
	}
}

func TestReadEndpointsRequireSessionWhenAuthEnabled(t *testing.T) {
	service := newTestApp(t, "geheim")
	handler := service.Routes()

	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, nil).Code; code != http.StatusUnauthorized {
			t.Errorf("GET %s ohne Session = %d, erwartet 401", target, code)
		}
	}

	session := loginSession(t, handler, "admin", "geheim")
	for _, target := range []string{"/api/status", "/api/config", "/api/history", "/api/events"} {
		if code := do(t, handler, http.MethodGet, target, sessionHeaders(session)).Code; code != http.StatusOK {
			t.Errorf("GET %s mit Session = %d, erwartet 200", target, code)
		}
	}
}

// Regression: /api/health muss auch mit aktiviertem Login ohne Session
// erreichbar bleiben, sonst können Docker-Healthcheck und externes
// Monitoring (z. B. Uptime Kuma) den Dienst nicht mehr abfragen.
func TestHealthStaysOpenWhenAuthEnabled(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()
	if code := do(t, handler, http.MethodGet, "/api/health", nil).Code; code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/health ohne Session = %d, erwartet 503 (nicht 401)", code)
	}
}

func TestIndexRedirectsToLoginWhenUnauthenticated(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusFound {
		t.Fatalf("GET / ohne Session = %d, erwartet 302", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/login" {
		t.Fatalf("Redirect-Ziel = %q, erwartet /login", location)
	}
}

func TestLoginRejectsWrongCredentials(t *testing.T) {
	// Je ein frisches App/Routes-Paar pro Fall: der Login-Endpunkt hat sein
	// eigenes, strengeres Rate-Limit (loginRateInterval), das ein zweiter
	// Aufruf auf derselben Instanz sonst mit 429 statt der hier geprüften
	// 401-Antwort beantworten würde.
	wrongPassword := httptest.NewRequest(http.MethodPost, "/login", nil)
	wrongPassword.SetBasicAuth("admin", "falsch")
	recorder := httptest.NewRecorder()
	newTestApp(t, "geheim").Routes().ServeHTTP(recorder, wrongPassword)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("falsches Passwort = %d, erwartet 401", recorder.Code)
	}

	wrongUser := httptest.NewRequest(http.MethodPost, "/login", nil)
	wrongUser.SetBasicAuth("jemand-anderes", "geheim")
	recorder = httptest.NewRecorder()
	newTestApp(t, "geheim").Routes().ServeHTTP(recorder, wrongUser)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("falscher Benutzername = %d, erwartet 401", recorder.Code)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()
	session := loginSession(t, handler, "admin", "geheim")

	if code := do(t, handler, http.MethodGet, "/api/status", sessionHeaders(session)).Code; code != http.StatusOK {
		t.Fatalf("Session vor Logout = %d, erwartet 200", code)
	}

	logout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logout.Header.Set("Cookie", sessionCookieName+"="+session)
	handler.ServeHTTP(httptest.NewRecorder(), logout)

	if code := do(t, handler, http.MethodGet, "/api/status", sessionHeaders(session)).Code; code != http.StatusUnauthorized {
		t.Fatalf("Session nach Logout = %d, erwartet 401", code)
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

func TestWriteEndpointsRequireSessionWhenAuthEnabled(t *testing.T) {
	handler := newTestApp(t, "geheim").Routes()

	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusUnauthorized {
		t.Errorf("ohne Session = %d, erwartet 401", code)
	}

	session := loginSession(t, handler, "admin", "geheim")
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", sessionHeaders(session)).Code; code != http.StatusAccepted {
		t.Errorf("mit Session = %d, erwartet 202", code)
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
	// Jede Anfrage bekommt ihre eigene App-Instanz: seit dem Rate-Limit pro
	// Kategorie (ein Schreibvorgang pro Sekunde für "override"/"test") würde
	// ein zweiter Aufruf auf derselben Instanz sonst 429 statt der hier
	// geprüften Validierungsantwort liefern.
	for _, target := range []string{"/api/fan/0", "/api/fan/101", "/api/fan/abc", "/api/test/0"} {
		handler := newTestApp(t, "").Routes()
		if code := do(t, handler, http.MethodPost, target, nil).Code; code != http.StatusBadRequest {
			t.Errorf("POST %s = %d, erwartet 400", target, code)
		}
	}
	handler := newTestApp(t, "").Routes()
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

	// Frische Instanz: das Rate-Limit der "profile"-Kategorie (ein
	// Schreibvorgang pro Sekunde) würde einen zweiten Aufruf auf derselben
	// Instanz sonst mit 429 statt 404 beantworten.
	other := newTestApp(t, "").Routes()
	if code := do(t, other, http.MethodPost, "/api/profile/gibtsnicht", nil).Code; code != http.StatusNotFound {
		t.Fatalf("unbekanntes Profil = %d, erwartet 404", code)
	}
}

func TestOverridePersistsManualMode(t *testing.T) {
	service := newTestApp(t, "")
	if err := service.setOverride(Override{Mode: ModeManual, Percent: 42}); err != nil {
		t.Fatalf("setOverride: %v", err)
	}

	restored := service.loadOverride()
	if restored.Mode != ModeManual || restored.Percent != 42 {
		t.Fatalf("manueller Modus nicht gespeichert: %+v", restored)
	}

	if err := service.setOverride(Override{}); err != nil {
		t.Fatalf("setOverride: %v", err)
	}
	if cleared := service.loadOverride(); cleared.Mode != "" {
		t.Fatalf("Automatik nicht gespeichert: %+v", cleared)
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
		VarINI:     "/nicht/vorhanden/var.ini",
		DisksINI:   "/nicht/vorhanden/disks.ini",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blockPersistPath(t, dataDir, overrideFile)

	before := service.loadOverride()
	if err := service.setOverride(Override{Mode: ModeManual, Percent: 42}); err == nil {
		t.Fatal("erwarte Fehler, wenn override.json nicht geschrieben werden kann")
	}

	service.mu.RLock()
	current := service.override
	service.mu.RUnlock()
	if current != before {
		t.Fatalf("interner Zustand wurde trotz Persistenzfehler geändert: %+v", current)
	}
}

func TestManualEndpointReturns500OnPersistFailure(t *testing.T) {
	dataDir := t.TempDir()
	service, err := New(Config{
		DataDir:    dataDir,
		I2CAddress: "0x69",
		VarINI:     "/nicht/vorhanden/var.ini",
		DisksINI:   "/nicht/vorhanden/disks.ini",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blockPersistPath(t, dataDir, overrideFile)
	handler := service.Routes()

	if code := do(t, handler, http.MethodPost, "/api/fan/75", nil).Code; code != http.StatusInternalServerError {
		t.Fatalf("POST /api/fan/75 mit blockierter Persistenz = %d, erwartet 500", code)
	}
}

func TestFanTestRejectsUnsafeValue(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	// Parity-Check aktiv + niedriger Testwert muss unabhängig von der
	// Hardware (die in Tests nicht existiert) mit 409 abgelehnt werden, da
	// die Prüfung vor dem eigentlichen I2C-Schreibvorgang greift.
	service.mu.Lock()
	service.status.TemperatureValid = true
	service.status.ArrayOperation = "parity-check"
	service.mu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/5", nil).Code; code != http.StatusConflict {
		t.Fatalf("unsicherer Testwert während Parity-Check = %d, erwartet 409", code)
	}
}

func TestFanTestRejectsConcurrentRuns(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	if !service.testMu.TryLock() {
		t.Fatal("konnte testMu nicht vorab sperren")
	}
	defer service.testMu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/50", nil).Code; code != http.StatusConflict {
		t.Fatalf("Test während laufendem Test = %d, erwartet 409", code)
	}
}

func TestFanTestCooldown(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	service.testMu.Lock()
	service.testCooldownUntil = time.Now().Add(time.Minute)
	service.testMu.Unlock()

	if code := do(t, handler, http.MethodPost, "/api/test/50", nil).Code; code != http.StatusTooManyRequests {
		t.Fatalf("Test im Cooldown = %d, erwartet 429", code)
	}
}

func TestWriteEndpointsAreRateLimitedPerCategory(t *testing.T) {
	handler := newTestApp(t, "").Routes()

	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusAccepted {
		t.Fatalf("erster Aufruf = %d, erwartet 202", code)
	}
	if code := do(t, handler, http.MethodPost, "/api/mode/auto", nil).Code; code != http.StatusTooManyRequests {
		t.Fatalf("zweiter Aufruf innerhalb 1s = %d, erwartet 429", code)
	}
	// Eine andere Kategorie ("profile") darf vom Limit der "override"-
	// Kategorie nicht betroffen sein.
	if code := do(t, handler, http.MethodPost, "/api/profile/silent", nil).Code; code != http.StatusAccepted {
		t.Fatalf("andere Kategorie direkt danach = %d, erwartet 202", code)
	}
}

func TestConfigUpdateRejectsUnknownFields(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	body := `{"active_profile":"balanced","profiles":{},"unbekanntes_feld":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unbekanntes Feld = %d, erwartet 400", recorder.Code)
	}
}

func TestConfigUpdateRejectsTrailingData(t *testing.T) {
	handler := newTestApp(t, "").Routes()
	body := `{"active_profile":"balanced","profiles":{}}{"extra":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Daten nach dem JSON-Objekt = %d, erwartet 400", recorder.Code)
	}
}

// TestConcurrentConfigWritesDoNotInterleave ruft den Handler direkt auf
// (nicht über Routes()/guardWrite), um configMu unabhängig vom
// Rate-Limiting zu prüfen: mehrere gleichzeitige Config-Schreibvorgänge
// dürfen sich nicht überschneiden, config.json muss am Ende exakt einem der
// Requests entsprechen.
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
			t.Fatalf("unerwarteter Statuscode bei nebenläufigem Config-Update: %d", code)
		}
	}

	var onDisk RuntimeConfig
	if err := service.store.LoadJSON(configFile, &onDisk); err != nil {
		t.Fatalf("config.json konnte nicht gelesen werden: %v", err)
	}
	service.mu.RLock()
	inMemory := service.runtime
	service.mu.RUnlock()
	if onDisk.Profiles["balanced"].Curve[0].Percent != inMemory.Profiles["balanced"].Curve[0].Percent {
		t.Fatalf("config.json (%v) weicht vom Speicherzustand (%v) ab, Schreibvorgänge haben sich überschnitten",
			onDisk.Profiles["balanced"], inMemory.Profiles["balanced"])
	}
}

func TestHealthReportsIndividualChecks(t *testing.T) {
	service := newTestApp(t, "")
	handler := service.Routes()

	// Kaputtes aktives Profil -> config: false.
	service.mu.Lock()
	service.runtime.ActiveProfile = "existiert-nicht"
	service.mu.Unlock()

	recorder := do(t, handler, http.MethodGet, "/api/health", nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("Health mit ungültigem Profil = %d, erwartet 503", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Health-Antwort ist kein JSON: %v", err)
	}
	if config, _ := body["config"].(bool); config {
		t.Fatalf("config sollte false sein: %+v", body)
	}

	// Storage-Fehler simulieren.
	service.setStorageOK(storageConfig, false)
	recorder = do(t, handler, http.MethodGet, "/api/health", nil)
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Health-Antwort ist kein JSON: %v", err)
	}
	if storage, _ := body["storage"].(bool); storage {
		t.Fatalf("storage sollte false sein: %+v", body)
	}
}

// Regression: a single combined storageOK flag let an unrelated successful
// write mask an earlier, still-unresolved failure in a different category.
func TestStorageHealthDoesNotMaskUnrelatedCategoryFailure(t *testing.T) {
	service := newTestApp(t, "")

	service.setStorageOK(storageConfig, false)
	if service.storageHealthy() {
		t.Fatal("storageHealthy sollte nach fehlgeschlagenem Config-Write false sein")
	}

	service.setStorageOK(storageHistory, true)
	if service.storageHealthy() {
		t.Fatal("ein erfolgreicher History-Write darf einen offenen Config-Fehler nicht verdecken")
	}

	service.setStorageOK(storageConfig, true)
	if !service.storageHealthy() {
		t.Fatal("storageHealthy sollte true sein, sobald auch Config wieder erfolgreich ist")
	}
}
