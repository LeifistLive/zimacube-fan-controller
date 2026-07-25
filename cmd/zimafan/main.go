package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/unraid"
	"github.com/LeifistLive/zimacube-fan-controller/internal/webui"
)

type Config struct {
	I2CBus             int
	I2CAddress         string
	FanCurve           controller.Curve
	ArrayBoostPercent  int
	EmergencyTemp      int
	EmergencyPercent   int
	HysteresisC        int
	CheckInterval      time.Duration
	I2CRetries         int
	ListenAddress      string
	APIToken           string
	VarINI             string
	DisksINI           string
	DataDir            string
	ControllerTimeout  time.Duration
}

type Override struct {
	Mode    string `json:"mode"`
	Percent int    `json:"percent,omitempty"`
}

type Status struct {
	Mode                   string    `json:"mode"`
	FanPercent             int       `json:"fan_percent"`
	MaximumDiskTemperature int       `json:"maximum_disk_temperature"`
	ArrayOperation         string    `json:"array_operation"`
	Reason                 string    `json:"reason"`
	ControllerOnline       bool      `json:"controller_online"`
	LastWriteSuccessful    bool      `json:"last_write_successful"`
	LastError              string    `json:"last_error,omitempty"`
	Updated                time.Time `json:"updated"`
}

type App struct {
	cfg        Config
	i2c        *controller.I2C
	mu         sync.RWMutex
	status     Status
	override   Override
	commandCh  chan struct{}
	lastTarget int
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("[ERROR] Konfiguration ungültig: %v", err)
	}

	app := &App{
		cfg:       cfg,
		i2c:       controller.NewI2C(cfg.I2CBus, cfg.I2CAddress, cfg.ControllerTimeout, cfg.I2CRetries),
		commandCh: make(chan struct{}, 1),
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("[ERROR] Datenverzeichnis kann nicht erstellt werden: %v", err)
	}
	app.override = app.loadOverride()

	log.Printf("[INFO] ZimaCube Fan Controller startet")
	log.Printf("[INFO] I²C Bus %d, Adresse %s", cfg.I2CBus, cfg.I2CAddress)
	log.Printf("[INFO] Lüfterkurve: %s", cfg.FanCurve.String())
	log.Printf("[INFO] Array-Boost: %d%%", cfg.ArrayBoostPercent)
	log.Printf("[INFO] Notfall: %d%% ab %d °C", cfg.EmergencyPercent, cfg.EmergencyTemp)
	log.Printf("[INFO] Web/API: http://%s", cfg.ListenAddress)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := waitForController(ctx, app.i2c); err != nil {
		log.Fatalf("[ERROR] I²C-Controller nicht erreichbar: %v", err)
	}
	log.Printf("[OK] I²C-Controller gefunden")

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] HTTP-Server: %v", err)
			cancel()
		}
	}()

	app.run(ctx)
	log.Printf("[INFO] Container wird beendet")
}

func loadConfig() (Config, error) {
	curve, err := controller.ParseCurve(env("FAN_CURVE", "0:60,36:65,40:75,43:85,46:95,48:100"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		I2CBus:            envInt("I2C_BUS", 0),
		I2CAddress:        env("I2C_ADDRESS", "0x69"),
		FanCurve:          curve,
		ArrayBoostPercent: envInt("ARRAY_BOOST_PERCENT", 100),
		EmergencyTemp:     envInt("EMERGENCY_TEMP", 52),
		EmergencyPercent:  envInt("EMERGENCY_PERCENT", 100),
		HysteresisC:       envInt("HYSTERESIS_C", 2),
		CheckInterval:     time.Duration(envInt("CHECK_INTERVAL_SECONDS", 15)) * time.Second,
		I2CRetries:        envInt("I2C_RETRIES", 3),
		ListenAddress:     env("LISTEN_ADDRESS", ":8080"),
		APIToken:          os.Getenv("API_TOKEN"),
		VarINI:            env("UNRAID_VAR_INI", "/var/local/emhttp/var.ini"),
		DisksINI:          env("UNRAID_DISKS_INI", "/var/local/emhttp/disks.ini"),
		DataDir:           env("DATA_DIR", "/data"),
		ControllerTimeout: time.Duration(envInt("I2C_TIMEOUT_SECONDS", 5)) * time.Second,
	}

	for name, value := range map[string]int{
		"ARRAY_BOOST_PERCENT": cfg.ArrayBoostPercent,
		"EMERGENCY_PERCENT":   cfg.EmergencyPercent,
	} {
		if value < 1 || value > 100 {
			return Config{}, fmt.Errorf("%s muss zwischen 1 und 100 liegen", name)
		}
	}
	if cfg.EmergencyTemp < 1 || cfg.HysteresisC < 0 || cfg.CheckInterval < time.Second || cfg.I2CRetries < 1 {
		return Config{}, errors.New("Temperatur-, Intervall- oder Retry-Werte sind ungültig")
	}
	return cfg, nil
}

func waitForController(ctx context.Context, i2c *controller.I2C) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(120 * time.Second)
	defer timeout.Stop()

	for {
		ok, err := i2c.Probe(ctx)
		if ok {
			return nil
		}
		if err != nil {
			log.Printf("[WARN] Controller-Probe fehlgeschlagen: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return errors.New("Zeitüberschreitung")
		case <-ticker.C:
		}
	}
}

func (a *App) run(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.CheckInterval)
	defer ticker.Stop()

	a.evaluate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.evaluate(ctx)
		case <-a.commandCh:
			a.evaluate(ctx)
		}
	}
}

func (a *App) evaluate(ctx context.Context) {
	temp := unraid.MaxDiskTemperature(a.cfg.DisksINI)
	operation := unraid.ArrayOperation(a.cfg.VarINI)

	a.mu.RLock()
	override := a.override
	lastTarget := a.lastTarget
	a.mu.RUnlock()

	target := a.cfg.FanCurve.Speed(temp)
	mode := "automatic"
	reason := fmt.Sprintf("Temperaturkurve: höchste HDD-Temperatur %d °C", temp)

	if override.Mode == "manual" {
		target = override.Percent
		mode = "manual"
		reason = "manuelle Vorgabe"
	} else if override.Mode == "emergency" {
		target = a.cfg.EmergencyPercent
		mode = "emergency"
		reason = "manueller Notfallmodus"
	}

	if operation != "none" && target < a.cfg.ArrayBoostPercent {
		target = a.cfg.ArrayBoostPercent
		reason = "Array-Operation läuft: " + operation
		if mode == "automatic" {
			mode = "array-boost"
		}
	}

	if temp >= a.cfg.EmergencyTemp {
		target = a.cfg.EmergencyPercent
		mode = "emergency"
		reason = fmt.Sprintf("Notfalltemperatur erreicht: %d °C", temp)
	}

	if lastTarget > 0 && target < lastTarget {
		threshold := a.cfg.FanCurve.ThresholdForSpeed(lastTarget)
		if temp > threshold-a.cfg.HysteresisC {
			target = lastTarget
			reason += fmt.Sprintf(" (Hysterese hält %d%%)", lastTarget)
		}
	}

	online, probeErr := a.i2c.Probe(ctx)
	writeOK := true
	var writeErr error

	if !online {
		writeOK = false
		writeErr = probeErr
	} else if target != lastTarget {
		log.Printf("[INFO] Setze Lüfter auf %d%% – %s", target, reason)
		writeErr = a.i2c.SetPercent(ctx, target)
		writeOK = writeErr == nil
		if writeOK {
			log.Printf("[OK] Lüftergeschwindigkeit erfolgreich gesetzt")
		} else {
			log.Printf("[ERROR] Lüftergeschwindigkeit konnte nicht gesetzt werden: %v", writeErr)
		}
	}

	a.mu.Lock()
	if writeOK {
		a.lastTarget = target
	}
	a.status = Status{
		Mode:                   mode,
		FanPercent:             target,
		MaximumDiskTemperature: temp,
		ArrayOperation:         operation,
		Reason:                 reason,
		ControllerOnline:       online,
		LastWriteSuccessful:    writeOK,
		Updated:                time.Now(),
	}
	if writeErr != nil {
		a.status.LastError = writeErr.Error()
	}
	a.mu.Unlock()
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(webui.IndexHTML))
	})
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("POST /api/mode/auto", a.requireToken(a.handleAuto))
	mux.HandleFunc("POST /api/mode/emergency", a.requireToken(a.handleEmergency))
	mux.HandleFunc("POST /api/fan/{percent}", a.requireToken(a.handleManual))

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (a *App) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.APIToken != "" && r.Header.Get("X-API-Token") != a.cfg.APIToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *App) handleStatus(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	status := a.status
	override := a.override
	a.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"override": override,
		"curve":    a.cfg.FanCurve,
	})
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	status := a.status
	a.mu.RUnlock()

	code := http.StatusOK
	if !status.ControllerOnline || !status.LastWriteSuccessful || status.Updated.IsZero() ||
		time.Since(status.Updated) > 2*a.cfg.CheckInterval+10*time.Second {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"healthy":           code == http.StatusOK,
		"controller_online": status.ControllerOnline,
		"updated":           status.Updated,
	})
}

func (a *App) handleAuto(w http.ResponseWriter, _ *http.Request) {
	a.setOverride(Override{Mode: "auto"})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "auto requested"})
}

func (a *App) handleEmergency(w http.ResponseWriter, _ *http.Request) {
	a.setOverride(Override{Mode: "emergency"})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "emergency requested"})
}

func (a *App) handleManual(w http.ResponseWriter, r *http.Request) {
	percent, err := strconv.Atoi(r.PathValue("percent"))
	if err != nil || percent < 1 || percent > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "percent must be between 1 and 100"})
		return
	}
	a.setOverride(Override{Mode: "manual", Percent: percent})
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "manual requested", "percent": percent})
}

func (a *App) setOverride(override Override) {
	a.mu.Lock()
	if override.Mode == "auto" {
		a.override = Override{}
	} else {
		a.override = override
	}
	current := a.override
	a.mu.Unlock()

	_ = a.saveOverride(current)
	select {
	case a.commandCh <- struct{}{}:
	default:
	}
}

func (a *App) overridePath() string {
	return filepath.Join(a.cfg.DataDir, "override.json")
}

func (a *App) loadOverride() Override {
	data, err := os.ReadFile(a.overridePath())
	if err != nil {
		return Override{}
	}
	var value Override
	if json.Unmarshal(data, &value) != nil {
		return Override{}
	}
	if value.Mode == "manual" && (value.Percent < 1 || value.Percent > 100) {
		return Override{}
	}
	if value.Mode != "manual" && value.Mode != "emergency" {
		return Override{}
	}
	return value
}

func (a *App) saveOverride(value Override) error {
	path := a.overridePath()
	if value.Mode == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}

var _ = exec.ErrNotFound
var _ = strings.Builder{}
