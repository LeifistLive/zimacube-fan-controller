package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
	"github.com/LeifistLive/zimacube-fan-controller/internal/webui"
)

// fanTestCooldown keeps repeated fan tests from spinning the fans up and down
// back to back; testMu additionally ensures only one test runs at a time.
const fanTestCooldown = 5 * time.Second

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated: the login page and endpoint themselves, and the
	// static assets both the login page and the dashboard need. /api/health
	// stays open too, for the Docker healthcheck and external monitoring
	// (Uptime Kuma etc.) that cannot log in.
	mux.HandleFunc("GET /login", a.handleLoginPage)
	mux.HandleFunc("POST /login", a.handleLogin)
	mux.HandleFunc("POST /logout", a.handleLogout)
	mux.HandleFunc("GET /login.js", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/javascript; charset=utf-8", webui.LoginJS)
	})
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /app.css", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/css; charset=utf-8", webui.StyleCSS)
	})
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/javascript; charset=utf-8", webui.ScriptJS)
	})

	// Everything else requires a valid session (see auth.go).
	mux.HandleFunc("GET /{$}", a.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/html; charset=utf-8", webui.IndexHTML)
	}))

	mux.HandleFunc("GET /api/status", a.requireAuth(a.handleStatus))
	mux.HandleFunc("GET /api/history", a.requireAuth(a.handleHistory))
	mux.HandleFunc("GET /api/events", a.requireAuth(a.handleEvents))
	mux.HandleFunc("GET /api/config", a.requireAuth(a.handleConfig))

	mux.HandleFunc("POST /api/fan/{percent}", a.requireAuth(a.guardWrite("override", a.handleManual)))
	mux.HandleFunc("POST /api/mode/auto", a.requireAuth(a.guardWrite("override", a.handleAuto)))
	mux.HandleFunc("POST /api/mode/emergency", a.requireAuth(a.guardWrite("override", a.handleEmergency)))
	mux.HandleFunc("POST /api/profile/{name}", a.requireAuth(a.guardWrite("profile", a.handleProfile)))
	mux.HandleFunc("POST /api/config", a.requireAuth(a.guardWrite("config", a.handleConfigUpdate)))
	mux.HandleFunc("POST /api/test/{percent}", a.requireAuth(a.guardWrite("test", a.handleFanTest)))
	mux.HandleFunc("POST /api/events/clear", a.requireAuth(a.guardWrite("events", a.handleClearEvents)))

	return securityHeaders(mux)
}

func serveAsset(w http.ResponseWriter, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write([]byte(body))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		// CSS and JavaScript are served from their own routes, so inline
		// sources are no longer needed and script-src stays strict.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'")
		next.ServeHTTP(w, r)
	})
}

// guardWrite rate-limits a write endpoint by category, independently of
// every other category (a profile change never blocks a fan test or vice
// versa). Authentication and the same-origin check already happened in
// requireAuth, which always wraps guardWrite, never the other way round.
func (a *App) guardWrite(category string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.writeLimiters[category].Allow(time.Now(), time.Second) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests, please wait a moment"})
			return
		}
		next(w, r)
	}
}

func checkSameOrigin(r *http.Request) error {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return errors.New("cross-site request rejected")
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return errors.New("invalid Origin header")
	}
	if parsed.Host != r.Host {
		return errors.New("cross-origin request rejected")
	}
	return nil
}

func (a *App) handleStatus(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	status := a.status
	override := a.override
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "override": override})
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	status := a.status
	_, configOK := a.runtime.Profiles[a.runtime.ActiveProfile]
	a.mu.RUnlock()

	stale := status.Updated.IsZero() || time.Since(status.Updated) > 2*a.cfg.CheckInterval+10*time.Second
	storageOK := a.storageHealthy()
	healthy := status.ControllerOnline && status.LastWriteSuccessful && status.TemperatureValid &&
		configOK && storageOK && !stale

	label := "unhealthy"
	if healthy {
		label = "healthy"
	}

	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"healthy":               healthy,
		"status":                label,
		"version":               Version,
		"controller":            status.ControllerOnline,
		"controller_online":     status.ControllerOnline,
		"config":                configOK,
		"last_write_successful": status.LastWriteSuccessful,
		"storage":               storageOK,
		"temperature_valid":     status.TemperatureValid,
		"stale":                 stale,
		"updated":               status.Updated,
	})
}

func (a *App) handleHistory(w http.ResponseWriter, r *http.Request) {
	history, err := a.store.ReadHistory(queryLimit(r, 288))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := a.store.ReadEvents(queryLimit(r, 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// handleClearEvents deletes every recorded event. A marker event is written
// immediately afterwards, both as a small audit trail (something did clear
// the log, and when) and so the dashboard's event list is not left looking
// broken/empty right after the action that just succeeded.
func (a *App) handleClearEvents(w http.ResponseWriter, _ *http.Request) {
	if err := a.store.ClearEvents(); err != nil {
		a.setStorageOK(storageEvents, false)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.appendEvent(store.Event{
		Time:    time.Now(),
		Type:    "log",
		Message: "event log cleared",
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (a *App) handleConfig(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	runtime := a.runtime
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, runtime)
}

func (a *App) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var incoming RuntimeConfig
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&incoming); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unexpected data after the JSON object"})
		return
	}

	normalized, err := normalizeRuntimeConfig(incoming)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Serialized against profile changes and overrides: validate -> persist
	// -> update memory -> trigger. Persisting before touching in-memory state
	// means a failed write leaves the running config untouched, no revert
	// needed.
	a.configMu.Lock()
	defer a.configMu.Unlock()

	if err := a.store.SaveJSON(configFile, normalized); err != nil {
		a.setStorageOK(storageConfig, false)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.setStorageOK(storageConfig, true)

	a.mu.Lock()
	a.runtime = normalized
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	a.trigger()
	writeJSON(w, http.StatusAccepted, normalized)
}

func (a *App) handleManual(w http.ResponseWriter, r *http.Request) {
	percent, err := parsePercent(r.PathValue("percent"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.setOverride(Override{Mode: ModeManual, Percent: percent}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "manual requested", "percent": percent})
}

func (a *App) handleAuto(w http.ResponseWriter, _ *http.Request) {
	if err := a.setOverride(Override{}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "auto requested"})
}

func (a *App) handleEmergency(w http.ResponseWriter, _ *http.Request) {
	if err := a.setOverride(Override{Mode: ModeEmergency}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "emergency requested"})
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Serialized against config updates and overrides, same validate ->
	// persist -> update memory -> trigger flow as handleConfigUpdate.
	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.mu.RLock()
	_, ok := a.runtime.Profiles[name]
	candidate := a.runtime
	a.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	candidate.ActiveProfile = name

	if err := a.store.SaveJSON(configFile, candidate); err != nil {
		a.setStorageOK(storageConfig, false)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.setStorageOK(storageConfig, true)

	a.mu.Lock()
	a.runtime.ActiveProfile = name
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	a.trigger()
	writeJSON(w, http.StatusAccepted, map[string]string{"active_profile": name})
}

func (a *App) handleFanTest(w http.ResponseWriter, r *http.Request) {
	percent, err := parsePercent(r.PathValue("percent"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Only one fan test may run at a time; TryLock rejects a second request
	// immediately instead of queuing behind the first (a test in flight is
	// only a few seconds, but stacking them would defeat the point of a
	// controlled test).
	if !a.testMu.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "fan test already running"})
		return
	}
	defer a.testMu.Unlock()

	now := time.Now()
	if now.Before(a.testCooldownUntil) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "fan test in cooldown, please wait a moment"})
		return
	}

	a.mu.RLock()
	profile, profileFound := a.runtime.Profiles[a.runtime.ActiveProfile]
	status := a.status
	a.mu.RUnlock()
	if profileFound {
		if floor := fanTestFloor(profile, status); percent < floor {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":           fmt.Sprintf("test value %d%% is below the current safety floor of %d%%", percent, floor),
				"minimum_percent": floor,
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := a.i2c.SetPercent(ctx, percent); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.testCooldownUntil = time.Now().Add(fanTestCooldown)

	// The test wrote a value the control loop does not know about, so the
	// cached fan speed has to be invalidated or the loop would not restore it.
	a.mu.Lock()
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	a.appendEvent(store.Event{
		Time:    time.Now(),
		Type:    "fan-test",
		Message: fmt.Sprintf("Test: %d%%", percent),
	})
	a.trigger()
	writeJSON(w, http.StatusOK, map[string]any{"tested_percent": percent})
}

// setOverride persists before touching in-memory state: if persistence
// fails, the running override must stay exactly what it was, not silently
// diverge from what a restart would restore from disk.
func (a *App) setOverride(value Override) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	var err error
	if value.Mode == "" {
		err = a.store.Remove(overrideFile)
	} else {
		err = a.store.SaveJSON(overrideFile, value)
	}
	a.setStorageOK(storageOverride, err == nil)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.override = value
	a.invalidateFanStateLocked()
	a.mu.Unlock()
	a.trigger()
	return nil
}

func (a *App) loadOverride() Override {
	var value Override
	if err := a.store.LoadJSON(overrideFile, &value); err != nil {
		return Override{}
	}
	switch value.Mode {
	case ModeManual:
		if value.Percent >= controller.MinPercent && value.Percent <= controller.MaxPercent {
			return value
		}
	case ModeEmergency:
		return Override{Mode: ModeEmergency}
	}
	return Override{}
}

func parsePercent(raw string) (int, error) {
	percent, err := strconv.Atoi(raw)
	if err != nil || percent < controller.MinPercent || percent > controller.MaxPercent {
		return 0, fmt.Errorf("percentage must be between %d and %d", controller.MinPercent, controller.MaxPercent)
	}
	return percent, nil
}

func queryLimit(r *http.Request, fallback int) int {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 5000 {
		return fallback
	}
	return limit
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
