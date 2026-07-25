package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
	"github.com/LeifistLive/zimacube-fan-controller/internal/unraid"
)

const (
	configFile   = "config.json"
	overrideFile = "override.json"
)

type Config struct {
	I2CBus              int
	I2CAddress          string
	I2CTimeout          time.Duration
	I2CRetries          int
	CheckInterval       time.Duration
	HistoryInterval     time.Duration
	DetectInterval      time.Duration
	ListenAddress       string
	APIToken            string
	VarINI              string
	DisksINI            string
	DataDir             string
	MaxLogLines         int
	SafeShutdownPercent int
}

// sanitized clamps every value so that a bad environment variable cannot crash
// the service (NewTicker panics on a non-positive interval) or silently disable
// writing (retries below one).
func (c Config) sanitized() Config {
	if c.I2CBus < 0 {
		c.I2CBus = 0
	}
	c.I2CAddress = controller.NormalizeAddress(c.I2CAddress)
	if c.I2CTimeout <= 0 {
		c.I2CTimeout = 5 * time.Second
	}
	if c.I2CRetries < 1 {
		c.I2CRetries = 1
	}
	if c.CheckInterval < time.Second {
		c.CheckInterval = 15 * time.Second
	}
	if c.HistoryInterval < c.CheckInterval {
		c.HistoryInterval = c.CheckInterval
	}
	if c.DetectInterval < c.CheckInterval {
		c.DetectInterval = 5 * time.Minute
	}
	if c.ListenAddress == "" {
		c.ListenAddress = ":8080"
	}
	if c.VarINI == "" {
		c.VarINI = "/var/local/emhttp/var.ini"
	}
	if c.DisksINI == "" {
		c.DisksINI = "/var/local/emhttp/disks.ini"
	}
	if c.DataDir == "" {
		c.DataDir = "/data"
	}
	if c.MaxLogLines <= 0 {
		c.MaxLogLines = store.DefaultMaxLines
	}
	if c.SafeShutdownPercent < 0 {
		c.SafeShutdownPercent = 0
	}
	if c.SafeShutdownPercent > controller.MaxPercent {
		c.SafeShutdownPercent = controller.MaxPercent
	}
	return c
}

// loopState is the mutable state the control loop reads and writes every
// cycle. It is grouped separately from status/override/runtime because those
// three are the public, API-visible state, while loopState is internal
// bookkeeping the loop uses to decide whether to write or probe again.
type loopState struct {
	fanPercent int
	mode       string
	operation  string
	historyAt  time.Time
	detectAt   time.Time
	writeOK    bool
	online     bool
	tempOK     bool
}

type App struct {
	cfg       Config
	i2c       *controller.I2C
	store     *store.Store
	started   time.Time
	commandCh chan struct{}

	mu         sync.RWMutex
	status     Status
	override   Override
	runtime    RuntimeConfig
	generation uint64
	state      loopState
}

func New(cfg Config) (*App, error) {
	cfg = cfg.sanitized()
	if !controller.ValidAddress(cfg.I2CAddress) {
		return nil, fmt.Errorf("ungültige I²C-Adresse %q", cfg.I2CAddress)
	}

	st := store.New(cfg.DataDir, cfg.MaxLogLines)
	if err := st.Ensure(); err != nil {
		return nil, err
	}

	app := &App{
		cfg:       cfg,
		i2c:       controller.NewI2C(cfg.I2CBus, cfg.I2CAddress, cfg.I2CTimeout, cfg.I2CRetries),
		store:     st,
		started:   time.Now(),
		commandCh: make(chan struct{}, 1),
		runtime:   defaultRuntimeConfig(),
		state:     loopState{online: true, tempOK: true},
	}

	loaded, err := loadRuntimeConfig(st)
	switch {
	case err == nil:
		app.runtime = loaded
	case errors.Is(err, os.ErrNotExist):
		log.Printf("[INFO] Keine %s vorhanden, Standardprofile werden verwendet", configFile)
	default:
		log.Printf("[WARN] %s wird ignoriert (%v), Standardprofile werden verwendet", configFile, err)
		_ = st.AppendEvent(store.Event{
			Time:    time.Now(),
			Type:    "config",
			Message: configFile + " unbrauchbar: " + err.Error(),
		})
	}

	app.override = app.loadOverride()

	if cfg.APIToken == "" {
		log.Printf("[WARN] API_TOKEN ist leer, Schreibzugriffe sind ohne Authentifizierung möglich")
	}
	if profile, ok := app.runtime.Profiles[app.runtime.ActiveProfile]; ok && profile.ArrayBoostPercent < 50 {
		log.Printf("[WARN] Sicherheitsdrehzahl von Profil %q ist nur %d%%, bei Sensorausfall bleibt es leise",
			app.runtime.ActiveProfile, profile.ArrayBoostPercent)
	}
	return app, nil
}

// loadRuntimeConfig deliberately decodes into an empty struct. The previous
// version decoded into the defaults, so a profile deleted through the API
// reappeared on the next restart.
func loadRuntimeConfig(st *store.Store) (RuntimeConfig, error) {
	var loaded RuntimeConfig
	if err := st.LoadJSON(configFile, &loaded); err != nil {
		return RuntimeConfig{}, err
	}
	return normalizeRuntimeConfig(loaded)
}

func defaultRuntimeConfig() RuntimeConfig {
	must := func(raw string) controller.Curve {
		curve, err := controller.ParseCurve(raw)
		if err != nil {
			panic(err)
		}
		return curve
	}
	return RuntimeConfig{
		ActiveProfile: "balanced",
		Profiles: map[string]Profile{
			"silent": {
				Name:                 "Silent",
				Curve:                must("0:45,36:55,40:65,43:75,46:90,48:100"),
				ArrayBoostPercent:    95,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				HysteresisC:          2,
			},
			"balanced": {
				Name:                 "Balanced",
				Curve:                must("0:60,36:65,40:75,43:85,46:95,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				HysteresisC:          2,
			},
			"performance": {
				Name:                 "Performance",
				Curve:                must("0:75,36:80,40:85,43:95,46:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 50,
				EmergencyPercent:     100,
				HysteresisC:          1,
			},
		},
	}
}

// normalizeRuntimeConfig validates every profile and returns a copy with sorted
// curves. Both the config file and the REST API go through here, so Speed and
// ThresholdForSpeed can rely on sorted, monotonic curves.
func normalizeRuntimeConfig(in RuntimeConfig) (RuntimeConfig, error) {
	if len(in.Profiles) == 0 {
		return RuntimeConfig{}, errors.New("mindestens ein Profil erforderlich")
	}

	out := RuntimeConfig{
		ActiveProfile: strings.TrimSpace(in.ActiveProfile),
		Profiles:      make(map[string]Profile, len(in.Profiles)),
	}

	for name, profile := range in.Profiles {
		if strings.TrimSpace(name) == "" {
			return RuntimeConfig{}, errors.New("Profilname darf nicht leer sein")
		}
		curve, err := profile.Curve.Normalized()
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Profil %q: %w", name, err)
		}
		if err := checkPercent("array_boost_percent", name, profile.ArrayBoostPercent); err != nil {
			return RuntimeConfig{}, err
		}
		if err := checkPercent("emergency_percent", name, profile.EmergencyPercent); err != nil {
			return RuntimeConfig{}, err
		}
		if profile.EmergencyTemperature < 20 || profile.EmergencyTemperature > 100 {
			return RuntimeConfig{}, fmt.Errorf("Profil %q: emergency_temperature %d liegt nicht zwischen 20 und 100",
				name, profile.EmergencyTemperature)
		}
		if profile.HysteresisC < 0 || profile.HysteresisC > 20 {
			return RuntimeConfig{}, fmt.Errorf("Profil %q: hysteresis_c %d liegt nicht zwischen 0 und 20",
				name, profile.HysteresisC)
		}
		if strings.TrimSpace(profile.Name) == "" {
			profile.Name = name
		}
		profile.Curve = curve
		out.Profiles[name] = profile
	}

	if _, ok := out.Profiles[out.ActiveProfile]; !ok {
		return RuntimeConfig{}, fmt.Errorf("aktives Profil %q existiert nicht", out.ActiveProfile)
	}
	return out, nil
}

func checkPercent(field, profileName string, value int) error {
	if value < controller.MinPercent || value > controller.MaxPercent {
		return fmt.Errorf("Profil %q: %s %d liegt nicht zwischen %d und %d",
			profileName, field, value, controller.MinPercent, controller.MaxPercent)
	}
	return nil
}

// AwaitController waits for the controller to answer. It is no longer fatal:
// the HTTP server is already running at this point, so the dashboard can show
// why the controller is missing instead of the container restart-looping.
func (a *App) AwaitController(ctx context.Context, limit time.Duration) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()

	for {
		found, err := a.i2c.Detect(ctx)
		if found {
			a.mu.Lock()
			a.state.detectAt = time.Now()
			a.mu.Unlock()
			return nil
		}
		if err != nil {
			log.Printf("[WARN] Controller-Suche fehlgeschlagen: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("Adresse %s auf Bus %d antwortet nicht", a.cfg.I2CAddress, a.cfg.I2CBus)
		case <-ticker.C:
		}
	}
}

func (a *App) Run(ctx context.Context) {
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

// ApplySafeState puts the fans into a defined speed when the service stops.
// Without it the backplane keeps whatever value was written last, which may be
// the minimum step while the array is still running.
func (a *App) ApplySafeState(percent int) {
	if percent < controller.MinPercent {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.i2c.SetPercent(ctx, percent); err != nil {
		log.Printf("[WARN] Abschaltdrehzahl %d%% konnte nicht gesetzt werden: %v", percent, err)
		return
	}
	log.Printf("[INFO] Abschaltdrehzahl %d%% gesetzt", percent)
	_ = a.store.AppendEvent(store.Event{
		Time:    time.Now(),
		Type:    "shutdown",
		Message: fmt.Sprintf("Abschaltdrehzahl %d%% gesetzt", percent),
	})
}

// decide is pure so that the control logic is unit testable.
func decide(profile Profile, reading sample, override Override, lastFan int) decision {
	failsafe := failsafePercent(profile)

	result := decision{
		Percent: profile.Curve.Speed(reading.Temperature),
		Mode:    ModeAutomatic,
		Reason:  fmt.Sprintf("Temperaturkurve: höchste HDD-Temperatur %d °C", reading.Temperature),
	}
	if !reading.Valid {
		result.Percent = failsafe
		result.Mode = ModeFailsafe
		result.Reason = "HDD-Temperatur unbekannt, Sicherheitsdrehzahl"
	}

	switch override.Mode {
	case ModeManual:
		result.Percent = override.Percent
		result.Mode = ModeManual
		result.Reason = fmt.Sprintf("manuelle Vorgabe %d%%", override.Percent)
		if !reading.Valid && result.Percent < failsafe {
			result.Percent = failsafe
			result.Mode = ModeFailsafe
			result.Reason = "HDD-Temperatur unbekannt, Sicherheitsdrehzahl über manueller Vorgabe"
		}
	case ModeEmergency:
		result.Percent = profile.EmergencyPercent
		result.Mode = ModeEmergency
		result.Reason = "manueller Notfallmodus"
	}

	if arrayActive(reading.Operation) && result.Percent < profile.ArrayBoostPercent {
		result.Percent = profile.ArrayBoostPercent
		result.Mode = ModeArrayBoost
		result.Reason = "Array-Operation läuft: " + reading.Operation
	}

	if reading.Valid && reading.Temperature >= profile.EmergencyTemperature {
		result.Percent = profile.EmergencyPercent
		result.Mode = ModeEmergency
		result.Reason = fmt.Sprintf("Notfalltemperatur erreicht: %d °C (Grenze %d °C)",
			reading.Temperature, profile.EmergencyTemperature)
	}

	// Hysteresis only damps the automatic curve. Applying it to boost or
	// emergency values kept the fans high long after the reason was gone.
	if result.Mode == ModeAutomatic && reading.Valid && profile.HysteresisC > 0 &&
		lastFan > 0 && result.Percent < lastFan {
		threshold := profile.Curve.ThresholdForSpeed(lastFan)
		if reading.Temperature > threshold-profile.HysteresisC {
			result.Percent = lastFan
			result.Reason += fmt.Sprintf(" (Hysterese hält %d%%)", lastFan)
		}
	}

	if result.Percent < controller.MinPercent {
		result.Percent = controller.MinPercent
	}
	if result.Percent > controller.MaxPercent {
		result.Percent = controller.MaxPercent
	}
	return result
}

// failsafePercent is used whenever the disk temperature is unknown.
func failsafePercent(profile Profile) int {
	percent := profile.ArrayBoostPercent
	if percent < controller.MinPercent {
		percent = controller.MaxPercent
	}
	return percent
}

func arrayActive(operation string) bool {
	switch operation {
	case "", unraid.OperationNone, unraid.OperationUnknown:
		return false
	default:
		return true
	}
}

func (a *App) evaluate(ctx context.Context) {
	now := time.Now()
	temperatures, tempErr := unraid.ReadDiskTemperatures(a.cfg.DisksINI)
	operation, opErr := unraid.ArrayOperation(a.cfg.VarINI)

	reading := sample{
		Temperature: temperatures.Maximum,
		Valid:       tempErr == nil,
		Reporting:   temperatures.Reporting,
		Operation:   operation,
	}

	a.mu.RLock()
	activeProfile := a.runtime.ActiveProfile
	profile, profileFound := a.runtime.Profiles[activeProfile]
	override := a.override
	lastFan := a.state.fanPercent
	generation := a.generation
	previousTempOK := a.state.tempOK
	previousOnline := a.state.online
	a.mu.RUnlock()

	if tempErr != nil && previousTempOK {
		log.Printf("[ERROR] HDD-Temperatur nicht lesbar (%v), Sicherheitsdrehzahl aktiv", tempErr)
		_ = a.store.AppendEvent(store.Event{
			Time:    now,
			Type:    "sensor",
			Message: "HDD-Temperatur nicht lesbar: " + tempErr.Error(),
		})
	}
	if tempErr == nil && !previousTempOK {
		log.Printf("[OK] HDD-Temperatur wieder lesbar")
	}
	if opErr != nil && operation == unraid.OperationUnknown {
		log.Printf("[WARN] Array-Status nicht lesbar: %v", opErr)
	}
	if !profileFound {
		log.Printf("[ERROR] Aktives Profil %q fehlt, es gilt die Sicherheitsdrehzahl", activeProfile)
	}

	result := decide(profile, reading, override, lastFan)

	online := true
	var statusErr error
	if err := a.i2c.DeviceAvailable(); err != nil {
		online = false
		statusErr = err
	} else if a.needsDetect(now) {
		found, err := a.i2c.Detect(ctx)
		switch {
		case err != nil:
			online = false
			statusErr = err
		case !found:
			online = false
			statusErr = fmt.Errorf("Adresse %s antwortet nicht auf Bus %d", a.cfg.I2CAddress, a.cfg.I2CBus)
		default:
			a.mu.Lock()
			a.state.detectAt = now
			a.mu.Unlock()
		}
	}

	writeOK := online
	var writeErr error
	switch {
	case !online:
		writeErr = statusErr
	case result.Percent != lastFan:
		log.Printf("[INFO] Setze Lüfter auf %d%%, %s", result.Percent, result.Reason)
		writeErr = a.i2c.SetPercent(ctx, result.Percent)
		writeOK = writeErr == nil
		if writeOK {
			_ = a.store.AppendEvent(store.Event{
				Time:    time.Now(),
				Type:    "fan-change",
				Message: fmt.Sprintf("%d%%, %s", result.Percent, result.Reason),
			})
		} else {
			log.Printf("[ERROR] Lüftergeschwindigkeit konnte nicht gesetzt werden: %v", writeErr)
		}
	}

	a.mu.Lock()
	// Only trust lastFan when no configuration change happened while the
	// decision was being made and written.
	if writeOK && a.generation == generation {
		a.state.fanPercent = result.Percent
	}
	a.state.writeOK = writeOK
	a.state.tempOK = reading.Valid
	a.state.online = online
	a.status = Status{
		Version:                Version,
		Mode:                   result.Mode,
		ActiveProfile:          activeProfile,
		FanPercent:             result.Percent,
		MaximumDiskTemperature: reading.Temperature,
		TemperatureValid:       reading.Valid,
		DisksReporting:         reading.Reporting,
		ArrayOperation:         operation,
		Reason:                 result.Reason,
		ControllerOnline:       online,
		LastWriteSuccessful:    writeOK,
		UptimeSeconds:          int64(now.Sub(a.started).Seconds()),
		Updated:                now,
	}
	if writeErr != nil {
		a.status.LastError = writeErr.Error()
	}
	shouldHistory := now.Sub(a.state.historyAt) >= a.cfg.HistoryInterval
	modeChanged := result.Mode != a.state.mode || operation != a.state.operation
	a.state.mode = result.Mode
	a.state.operation = operation
	if shouldHistory {
		a.state.historyAt = now
	}
	a.mu.Unlock()

	if online != previousOnline {
		if online {
			log.Printf("[OK] I²C-Controller erreichbar")
		} else {
			log.Printf("[ERROR] I²C-Controller nicht erreichbar: %v", statusErr)
		}
	}
	if shouldHistory {
		_ = a.store.AppendHistory(store.HistoryPoint{
			Time:        now,
			Temperature: reading.Temperature,
			FanPercent:  result.Percent,
			Mode:        result.Mode,
			Operation:   operation,
		})
	}
	if modeChanged {
		_ = a.store.AppendEvent(store.Event{
			Time:    now,
			Type:    "mode",
			Message: fmt.Sprintf("Modus=%s, Array=%s", result.Mode, operation),
		})
	}
}

// needsDetect keeps i2cdetect off the bus during normal operation. It only runs
// after a failed write or every DetectInterval.
func (a *App) needsDetect(now time.Time) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.state.writeOK {
		return true
	}
	return now.Sub(a.state.detectAt) >= a.cfg.DetectInterval
}

// invalidateFanStateLocked forces the next cycle to write the fan speed again.
func (a *App) invalidateFanStateLocked() {
	a.state.fanPercent = 0
	a.generation++
}

func (a *App) trigger() {
	select {
	case a.commandCh <- struct{}{}:
	default:
	}
}
