package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
	"github.com/LeifistLive/zimacube-fan-controller/internal/webui"
)

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/html; charset=utf-8", webui.IndexHTML)
	})
	mux.HandleFunc("GET /app.css", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/css; charset=utf-8", webui.StyleCSS)
	})
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "text/javascript; charset=utf-8", webui.ScriptJS)
	})

	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/history", a.handleHistory)
	mux.HandleFunc("GET /api/events", a.handleEvents)
	mux.HandleFunc("GET /api/config", a.handleConfig)

	mux.HandleFunc("POST /api/fan/{percent}", a.guardWrite(a.handleManual))
	mux.HandleFunc("POST /api/mode/auto", a.guardWrite(a.handleAuto))
	mux.HandleFunc("POST /api/mode/emergency", a.guardWrite(a.handleEmergency))
	mux.HandleFunc("POST /api/profile/{name}", a.guardWrite(a.handleProfile))
	mux.HandleFunc("POST /api/config", a.guardWrite(a.handleConfigUpdate))
	mux.HandleFunc("POST /api/test/{percent}", a.guardWrite(a.handleFanTest))

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

// guardWrite protects every write endpoint. The origin check blocks browser
// based cross site requests even when no API token is configured.
func (a *App) guardWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checkSameOrigin(r); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if a.cfg.APIToken != "" {
			provided := r.Header.Get("X-API-Token")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(a.cfg.APIToken)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func checkSameOrigin(r *http.Request) error {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return errors.New("cross-site-Anfrage abgelehnt")
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return errors.New("ungültiger Origin-Header")
	}
	if parsed.Host != r.Host {
		return errors.New("cross-origin-Anfrage abgelehnt")
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
	a.mu.RUnlock()

	stale := status.Updated.IsZero() || time.Since(status.Updated) > 2*a.cfg.CheckInterval+10*time.Second
	healthy := status.ControllerOnline && status.LastWriteSuccessful && status.TemperatureValid && !stale

	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"healthy":           healthy,
		"version":           Version,
		"controller_online": status.ControllerOnline,
		"temperature_valid": status.TemperatureValid,
		"stale":             stale,
		"updated":           status.Updated,
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

func (a *App) handleConfig(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	runtime := a.runtime
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, runtime)
}

func (a *App) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var incoming RuntimeConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&incoming); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ungültiges JSON"})
		return
	}
	normalized, err := normalizeRuntimeConfig(incoming)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	a.mu.Lock()
	previous := a.runtime
	a.runtime = normalized
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	if err := a.store.SaveJSON(configFile, normalized); err != nil {
		a.mu.Lock()
		a.runtime = previous
		a.invalidateFanStateLocked()
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.trigger()
	writeJSON(w, http.StatusAccepted, normalized)
}

func (a *App) handleManual(w http.ResponseWriter, r *http.Request) {
	percent, err := parsePercent(r.PathValue("percent"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.setOverride(Override{Mode: ModeManual, Percent: percent})
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "manual requested", "percent": percent})
}

func (a *App) handleAuto(w http.ResponseWriter, _ *http.Request) {
	a.setOverride(Override{})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "auto requested"})
}

func (a *App) handleEmergency(w http.ResponseWriter, _ *http.Request) {
	a.setOverride(Override{Mode: ModeEmergency})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "emergency requested"})
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	a.mu.Lock()
	if _, ok := a.runtime.Profiles[name]; !ok {
		a.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Profil nicht gefunden"})
		return
	}
	previous := a.runtime.ActiveProfile
	a.runtime.ActiveProfile = name
	a.invalidateFanStateLocked()
	runtime := a.runtime
	a.mu.Unlock()

	if err := a.store.SaveJSON(configFile, runtime); err != nil {
		a.mu.Lock()
		a.runtime.ActiveProfile = previous
		a.invalidateFanStateLocked()
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.trigger()
	writeJSON(w, http.StatusAccepted, map[string]string{"active_profile": name})
}

func (a *App) handleFanTest(w http.ResponseWriter, r *http.Request) {
	percent, err := parsePercent(r.PathValue("percent"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := a.i2c.SetPercent(ctx, percent); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// The test wrote a value the control loop does not know about, so the
	// cached fan speed has to be invalidated or the loop would not restore it.
	a.mu.Lock()
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	_ = a.store.AppendEvent(store.Event{
		Time:    time.Now(),
		Type:    "fan-test",
		Message: fmt.Sprintf("Test: %d%%", percent),
	})
	a.trigger()
	writeJSON(w, http.StatusOK, map[string]any{"tested_percent": percent})
}

func (a *App) setOverride(value Override) {
	a.mu.Lock()
	a.override = value
	a.invalidateFanStateLocked()
	a.mu.Unlock()

	if value.Mode == "" {
		_ = a.store.Remove(overrideFile)
	} else {
		_ = a.store.SaveJSON(overrideFile, value)
	}
	a.trigger()
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
		return 0, fmt.Errorf("Prozentwert muss zwischen %d und %d liegen", controller.MinPercent, controller.MaxPercent)
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
