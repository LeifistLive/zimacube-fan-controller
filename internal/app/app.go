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

	// writeFailureRetryInterval overrides ReapplyInterval right after a
	// failed I2C write, so a transient fault is retried quickly instead of
	// waiting out the full (multi-minute) reapply interval.
	writeFailureRetryInterval = 10 * time.Second
)

type Config struct {
	I2CBus          int
	I2CAddress      string
	I2CTimeout      time.Duration
	I2CRetries      int
	CheckInterval   time.Duration
	HistoryInterval time.Duration
	DetectInterval  time.Duration
	ReapplyInterval time.Duration
	ListenAddress   string
	AdminUser       string
	AdminPassword   string
	VarINI          string
	DisksINI        string
	// DataDir holds config.json, override.json, history.jsonl and
	// events.jsonl. History/event writes happen every few minutes (see
	// HistoryInterval) even when nothing changes, which would periodically
	// wake a spun-down Unraid array disk; keep this on non-array storage
	// (the Docker data-root/cache, not a /mnt/user bind mount).
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
	if c.ReapplyInterval < c.CheckInterval {
		c.ReapplyInterval = 5 * time.Minute
	}
	if c.ListenAddress == "" {
		c.ListenAddress = ":8080"
	}
	if c.AdminUser == "" {
		c.AdminUser = "admin"
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
	reapplyAt  time.Time
	writeOK    bool
	online     bool
	tempOK     bool
}

// rateGate is a minimal "at most once per interval" gate, used to rate-limit
// write endpoints per category. It intentionally does not queue or burst:
// a rejected call must be retried later by the caller.
type rateGate struct {
	mu   sync.Mutex
	next time.Time
}

func (g *rateGate) Allow(now time.Time, interval time.Duration) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if now.Before(g.next) {
		return false
	}
	g.next = now.Add(interval)
	return true
}

type App struct {
	cfg       Config
	i2c       *controller.I2C
	store     *store.Store
	started   time.Time
	commandCh chan struct{}

	// configMu serializes the validate -> persist -> update-memory -> trigger
	// flow for profile changes, config updates and overrides, so concurrent
	// HTTP requests cannot interleave their persistence steps.
	configMu sync.Mutex

	// writeLimiters rate-limits write endpoints per category ("override",
	// "profile", "config", "test") instead of globally, so an action on one
	// endpoint never blocks an unrelated one.
	writeLimiters map[string]*rateGate

	// testMu ensures only one fan test runs at a time; testCooldownUntil adds
	// a short cooldown afterwards, since a test is physically more disruptive
	// than other writes.
	testMu            sync.Mutex
	testCooldownUntil time.Time

	// storageMu/storageOK track the most recent persistence result per
	// category (config, override, history, events) separately, surfaced via
	// /api/health. Tracking one combined flag let a successful, unrelated
	// write (e.g. a history tick) mask an earlier, still-unresolved failure
	// (e.g. config.json); storageHealthy() is only true when every category
	// last succeeded.
	storageMu sync.RWMutex
	storageOK map[string]bool

	// auth guards the whole dashboard (session cookies), see auth.go.
	auth *auth

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
		return nil, fmt.Errorf("invalid I²C address %q", cfg.I2CAddress)
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
		storageOK: map[string]bool{
			storageConfig:   true,
			storageOverride: true,
			storageHistory:  true,
			storageEvents:   true,
		},
		writeLimiters: map[string]*rateGate{
			"override": {},
			"profile":  {},
			"config":   {},
			"test":     {},
			"events":   {},
		},
	}
	auth, err := newAuth(cfg.AdminUser, cfg.AdminPassword)
	if err != nil {
		// A password that cannot be hashed must never fall back to "auth
		// disabled" (that would mean the dashboard is open); refuse to start
		// instead, exactly like the invalid-I2C-address check above.
		return nil, fmt.Errorf("invalid login configuration: %w", err)
	}
	app.auth = auth

	loaded, err := loadRuntimeConfig(st)
	switch {
	case err == nil:
		app.runtime = loaded
	case errors.Is(err, os.ErrNotExist):
		log.Printf("[INFO] No %s found, using default profiles", configFile)
	default:
		log.Printf("[WARN] %s ignored (%v), using default profiles", configFile, err)
		_ = st.AppendEvent(store.Event{
			Time:     time.Now(),
			Type:     "config",
			Message:  configFile + " unusable: " + err.Error(),
			Severity: SeverityWarning,
		})
	}

	app.override = app.loadOverride()

	if profile, ok := app.runtime.Profiles[app.runtime.ActiveProfile]; ok && profile.ArrayBoostPercent < 50 {
		log.Printf("[WARN] safety speed for profile %q is only %d%%, it stays quiet on sensor failure",
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
		ConfigVersion: currentConfigVersion,
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
			"target-temp": {
				Name: "Target Temp",
				// Kept so this profile still has a sane curve to fall back
				// to if TargetTemperature is ever cleared - unused while it
				// is set, since that replaces the curve as the "automatic"
				// behavior (see decide()).
				Curve:                must("0:60,36:65,40:75,43:85,46:95,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				HysteresisC:          2,
				TargetTemperature:    40,
				TargetMinimumPercent: defaultTargetMinimumPercent,
			},
		},
	}
}

// normalizeRuntimeConfig validates every profile and returns a copy with sorted
// curves. Both the config file and the REST API go through here, so Speed and
// ThresholdForSpeed can rely on sorted, monotonic curves.
func normalizeRuntimeConfig(in RuntimeConfig) (RuntimeConfig, error) {
	// ConfigVersion 0 means the field was absent, i.e. a config.json written
	// before this field existed; treat it as version 1 so existing
	// installations keep working without a manual migration step.
	if in.ConfigVersion > currentConfigVersion {
		return RuntimeConfig{}, fmt.Errorf("config_version %d is not supported by this version (max %d)",
			in.ConfigVersion, currentConfigVersion)
	}

	if len(in.Profiles) == 0 {
		return RuntimeConfig{}, errors.New("at least one profile is required")
	}

	out := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: strings.TrimSpace(in.ActiveProfile),
		Profiles:      make(map[string]Profile, len(in.Profiles)),
	}

	for name, profile := range in.Profiles {
		if strings.TrimSpace(name) == "" {
			return RuntimeConfig{}, errors.New("profile name must not be empty")
		}
		if len(name) > maxProfileNameLength {
			return RuntimeConfig{}, fmt.Errorf("profile name %q is longer than %d characters", name, maxProfileNameLength)
		}
		if len(profile.Name) > maxProfileNameLength {
			return RuntimeConfig{}, fmt.Errorf("display name %q of profile %q is longer than %d characters", profile.Name, name, maxProfileNameLength)
		}
		curve, err := profile.Curve.Normalized()
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("profile %q: %w", name, err)
		}
		if err := checkPercent("array_boost_percent", name, profile.ArrayBoostPercent); err != nil {
			return RuntimeConfig{}, err
		}
		if err := checkPercent("emergency_percent", name, profile.EmergencyPercent); err != nil {
			return RuntimeConfig{}, err
		}
		if profile.EmergencyTemperature < 20 || profile.EmergencyTemperature > 100 {
			return RuntimeConfig{}, fmt.Errorf("profile %q: emergency_temperature %d is not between 20 and 100",
				name, profile.EmergencyTemperature)
		}
		if profile.HysteresisC < 0 || profile.HysteresisC > 20 {
			return RuntimeConfig{}, fmt.Errorf("profile %q: hysteresis_c %d is not between 0 and 20",
				name, profile.HysteresisC)
		}
		if profile.TargetTemperature != 0 && (profile.TargetTemperature < 20 || profile.TargetTemperature > 100) {
			return RuntimeConfig{}, fmt.Errorf("profile %q: target_temperature %d is not between 20 and 100",
				name, profile.TargetTemperature)
		}
		if profile.TargetMinimumPercent != 0 &&
			(profile.TargetMinimumPercent < controller.MinPercent || profile.TargetMinimumPercent > controller.MaxPercent) {
			return RuntimeConfig{}, fmt.Errorf("profile %q: target_minimum_percent %d is not between %d and %d",
				name, profile.TargetMinimumPercent, controller.MinPercent, controller.MaxPercent)
		}
		if strings.TrimSpace(profile.Name) == "" {
			profile.Name = name
		}
		profile.Curve = curve
		out.Profiles[name] = profile
	}

	// One-time migration: a config.json saved before config_version 2 never
	// had the built-in "target-temp" profile, so it would otherwise be stuck
	// missing from the profile table until someone hand-edits the JSON. This
	// only fires for configs below version 2; one already saved at 2 is left
	// alone whether the profile was kept, renamed, or deliberately removed.
	if in.ConfigVersion < 2 {
		if _, ok := out.Profiles["target-temp"]; !ok {
			out.Profiles["target-temp"] = defaultRuntimeConfig().Profiles["target-temp"]
		}
	}

	if _, ok := out.Profiles[out.ActiveProfile]; !ok {
		return RuntimeConfig{}, fmt.Errorf("active profile %q does not exist", out.ActiveProfile)
	}
	return out, nil
}

func checkPercent(field, profileName string, value int) error {
	if value < controller.MinPercent || value > controller.MaxPercent {
		return fmt.Errorf("profile %q: %s %d is not between %d and %d",
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
			log.Printf("[WARN] controller search failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("address %s on bus %d is not responding", a.cfg.I2CAddress, a.cfg.I2CBus)
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
		log.Printf("[WARN] shutdown speed %d%% could not be set: %v", percent, err)
		return
	}
	// Deliberately worded so this line is easy to grep for after the
	// container has already stopped, to confirm the safe speed was really
	// written before Docker terminated the process.
	log.Printf("[OK] safe-shutdown speed %d%% written before the service stops", percent)
	a.appendEvent(store.Event{
		Time:     time.Now(),
		Type:     "shutdown",
		Message:  fmt.Sprintf("shutdown speed %d%% set", percent),
		Severity: SeverityInfo,
	})
}

// Storage categories tracked independently by setStorageOK/storageHealthy.
const (
	storageConfig   = "config"
	storageOverride = "override"
	storageHistory  = "history"
	storageEvents   = "events"
)

// setStorageOK records whether the most recent persistence attempt for one
// category (config, override, history or events) succeeded. Categories are
// independent on purpose: a successful history tick a minute after a failed
// config save must not erase the fact that config.json is still broken.
func (a *App) setStorageOK(category string, ok bool) {
	a.storageMu.Lock()
	a.storageOK[category] = ok
	a.storageMu.Unlock()
}

// storageHealthy reports whether every category's most recent attempt
// succeeded. Surfaced via /api/health so monitoring can distinguish
// "controller unreachable" from "disk full".
func (a *App) storageHealthy() bool {
	a.storageMu.RLock()
	defer a.storageMu.RUnlock()
	for _, ok := range a.storageOK {
		if !ok {
			return false
		}
	}
	return true
}

// appendEvent and appendHistory wrap the store's persistence calls so a
// failure is logged and reflected in storageOK instead of being silently
// discarded. The control loop must keep running either way.
func (a *App) appendEvent(event store.Event) {
	err := a.store.AppendEvent(event)
	a.setStorageOK(storageEvents, err == nil)
	if err != nil {
		log.Printf("[WARN] event could not be saved: %v", err)
	}
}

func (a *App) appendHistory(point store.HistoryPoint) {
	err := a.store.AppendHistory(point)
	a.setStorageOK(storageHistory, err == nil)
	if err != nil {
		log.Printf("[WARN] history point could not be saved: %v", err)
	}
}

// decide is pure so that the control logic is unit testable.
func decide(profile Profile, reading sample, override Override, lastFan int) decision {
	failsafe := failsafePercent(profile)

	var result decision
	if profile.TargetTemperature > 0 {
		// A target-temperature profile replaces the curve as the "automatic"
		// behavior: step the fan speed toward keeping the array at or below
		// the profile's configured target instead of following fixed points.
		if !reading.Valid {
			result = decision{Percent: failsafe, Mode: ModeFailsafe, Reason: "HDD temperature unknown, safety speed"}
		} else {
			start := lastFan
			if start <= 0 {
				start = failsafe
			}
			percent := targetTemperatureStep(reading.Temperature, profile.TargetTemperature, start)
			if floor := targetMinimumPercent(profile); percent < floor {
				percent = floor
			}
			result = decision{
				Percent: percent,
				Mode:    ModeTargetTemp,
				Reason: fmt.Sprintf("keeping HDDs at or below %d °C (currently %d °C)",
					profile.TargetTemperature, reading.Temperature),
			}
		}
	} else {
		result = decision{
			Percent: profile.Curve.Speed(reading.Temperature),
			Mode:    ModeAutomatic,
			Reason:  fmt.Sprintf("temperature curve: highest HDD temperature %d °C", reading.Temperature),
		}
		if !reading.Valid {
			result.Percent = failsafe
			result.Mode = ModeFailsafe
			result.Reason = "HDD temperature unknown, safety speed"
		}
	}

	switch override.Mode {
	case ModeManual:
		result.Percent = override.Percent
		result.Mode = ModeManual
		result.Reason = fmt.Sprintf("manual setting %d%%", override.Percent)
		if !reading.Valid && result.Percent < failsafe {
			result.Percent = failsafe
			result.Mode = ModeFailsafe
			result.Reason = "HDD temperature unknown, safety speed overrides manual setting"
		}
	case ModeEmergency:
		result.Percent = profile.EmergencyPercent
		result.Mode = ModeEmergency
		result.Reason = "manual emergency mode"
	}

	if arrayActive(reading.Operation) && result.Percent < profile.ArrayBoostPercent {
		result.Percent = profile.ArrayBoostPercent
		result.Mode = ModeArrayBoost
		result.Reason = "array operation running: " + reading.Operation
	}

	if reading.Valid && reading.Temperature >= profile.EmergencyTemperature {
		result.Percent = profile.EmergencyPercent
		result.Mode = ModeEmergency
		result.Reason = fmt.Sprintf("emergency temperature reached: %d °C (limit %d °C)",
			reading.Temperature, profile.EmergencyTemperature)
	}

	// Hysteresis only damps the automatic curve. Applying it to boost or
	// emergency values kept the fans high long after the reason was gone.
	if result.Mode == ModeAutomatic && reading.Valid && profile.HysteresisC > 0 &&
		lastFan > 0 && result.Percent < lastFan {
		threshold := profile.Curve.ThresholdForSpeed(lastFan)
		if reading.Temperature > threshold-profile.HysteresisC {
			result.Percent = lastFan
			result.Reason += fmt.Sprintf(" (hysteresis holds %d%%)", lastFan)
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

// Step sizes for ModeTargetTemp: a bigger upward step than downward, so the
// loop reacts quickly to rising temperatures but backs off gradually instead
// of oscillating the fan speed every cycle. targetTempBandC keeps it from
// stepping down again until the temperature is comfortably under the
// target, not just barely at it.
const (
	targetTempStepUp   = 8
	targetTempStepDown = 4
	targetTempBandC    = 2
)

// defaultTargetMinimumPercent is the fallback floor for a target-temperature
// profile that leaves TargetMinimumPercent unset (0), including profiles
// saved before that field existed - see targetMinimumPercent.
const defaultTargetMinimumPercent = 30

// targetTemperatureStep is the pure feedback step for ModeTargetTemp: nudge
// the previous speed up while above target, down once comfortably under it,
// and hold steady in between. decide() clamps the result to
// MinPercent/MaxPercent same as every other mode.
func targetTemperatureStep(currentTemp, target, lastPercent int) int {
	switch {
	case currentTemp > target:
		return lastPercent + targetTempStepUp
	case currentTemp <= target-targetTempBandC:
		return lastPercent - targetTempStepDown
	default:
		return lastPercent
	}
}

// targetMinimumPercent is the floor decide() will not step below while
// stepping down: a long stretch comfortably under the target would otherwise
// walk the fan all the way to the global 1% minimum. Zero means "unset" -
// including a profile saved before this field existed - and falls back to
// defaultTargetMinimumPercent instead of no floor at all.
func targetMinimumPercent(profile Profile) int {
	if profile.TargetMinimumPercent > 0 {
		return profile.TargetMinimumPercent
	}
	return defaultTargetMinimumPercent
}

// fanTestFloor mirrors decide()'s safety minimums: a manual fan test must
// never write a value lower than what the emergency, failsafe or array-boost
// logic would currently require, or it would briefly undercut the very
// protections those mechanisms exist for.
func fanTestFloor(profile Profile, status Status) int {
	floor := controller.MinPercent
	if !status.TemperatureValid {
		if v := failsafePercent(profile); v > floor {
			floor = v
		}
	}
	if arrayActive(status.ArrayOperation) && profile.ArrayBoostPercent > floor {
		floor = profile.ArrayBoostPercent
	}
	if status.TemperatureValid && status.MaximumDiskTemperature >= profile.EmergencyTemperature &&
		profile.EmergencyPercent > floor {
		floor = profile.EmergencyPercent
	}
	return floor
}

// shouldWriteFan decides whether the control loop must write to the
// controller this cycle: the target changed, the controller was just
// rediscovered after being offline (a reset may have forgotten the PWM), or
// the reapply interval elapsed. That interval shrinks to
// writeFailureRetryInterval right after a failed write, so a transient I2C
// fault is retried within seconds instead of waiting out the full,
// multi-minute reapply cadence.
func shouldWriteFan(valueChanged bool, sinceLastApply, reapplyInterval time.Duration, previousWriteOK, rediscovered bool) bool {
	if valueChanged || rediscovered {
		return true
	}
	interval := reapplyInterval
	if !previousWriteOK {
		interval = writeFailureRetryInterval
	}
	return sinceLastApply >= interval
}

// diskInfos converts the unraid package's internal disk records into the
// API-facing shape. unraid.ReadDiskTemperatures has already filtered out
// cache/flash devices, so every entry here is an HDD.
func diskInfos(disks []unraid.Disk) []DiskInfo {
	out := make([]DiskInfo, 0, len(disks))
	for _, disk := range disks {
		out = append(out, DiskInfo{Name: disk.Name, Temperature: disk.Temperature, Valid: disk.Valid})
	}
	return out
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
	previousWriteOK := a.state.writeOK
	previousReapplyAt := a.state.reapplyAt
	a.mu.RUnlock()

	if tempErr != nil && previousTempOK {
		log.Printf("[ERROR] HDD temperature unreadable (%v), safety speed active", tempErr)
		a.appendEvent(store.Event{
			Time:     now,
			Type:     "sensor",
			Message:  "HDD temperature unreadable: " + tempErr.Error(),
			Severity: SeverityWarning,
		})
	}
	if tempErr == nil && !previousTempOK {
		log.Printf("[OK] HDD temperature readable again")
	}
	if opErr != nil && operation == unraid.OperationUnknown {
		log.Printf("[WARN] array status unreadable: %v", opErr)
	}
	if !profileFound {
		log.Printf("[ERROR] active profile %q missing, safety speed applies", activeProfile)
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
			statusErr = fmt.Errorf("address %s is not responding on bus %d", a.cfg.I2CAddress, a.cfg.I2CBus)
		default:
			a.mu.Lock()
			a.state.detectAt = now
			a.mu.Unlock()
		}
	}

	// A reapply/retry write repeats the same value the loop already believes
	// is active, so it must not be logged as a fan-change event (that would
	// flood events.jsonl every ReapplyInterval) and must not affect
	// mode/hysteresis decisions, only the wire write itself.
	valueChanged := result.Percent != lastFan
	rediscovered := online && !previousOnline
	attemptWrite := shouldWriteFan(valueChanged, now.Sub(previousReapplyAt), a.cfg.ReapplyInterval, previousWriteOK, rediscovered)

	writeOK := online
	var writeErr error
	switch {
	case !online:
		writeErr = statusErr
	case attemptWrite:
		if valueChanged {
			log.Printf("[INFO] setting fan to %d%%, %s", result.Percent, result.Reason)
		} else {
			log.Printf("[INFO] rewriting %d%% (reapply)", result.Percent)
		}
		writeErr = a.i2c.SetPercent(ctx, result.Percent)
		writeOK = writeErr == nil
		if writeOK {
			if valueChanged {
				// Distinguished from automatic/safety-driven changes so the
				// event log (and its filter) can single out speeds the
				// operator set directly, rather than ones the control loop
				// derived on its own.
				eventType := "fan-change"
				if result.Mode == ModeManual {
					eventType = "fan-change-manual"
				}
				a.appendEvent(store.Event{
					Time:     time.Now(),
					Type:     eventType,
					Message:  fmt.Sprintf("%d%%, %s", result.Percent, result.Reason),
					Severity: severityForMode(result.Mode),
				})
			}
		} else {
			log.Printf("[ERROR] fan speed could not be set: %v", writeErr)
		}
	}

	a.mu.Lock()
	// Only trust lastFan when no configuration change happened while the
	// decision was being made and written.
	if writeOK && a.generation == generation {
		a.state.fanPercent = result.Percent
	}
	if attemptWrite && writeOK && a.generation == generation {
		a.state.reapplyAt = now
	}
	a.state.writeOK = writeOK
	a.state.tempOK = reading.Valid
	a.state.online = online
	a.status = Status{
		Version:                Version,
		Mode:                   result.Mode,
		ActiveProfile:          activeProfile,
		I2CBus:                 a.cfg.I2CBus,
		I2CAddress:             a.cfg.I2CAddress,
		FanPercent:             result.Percent,
		TargetPercent:          result.Percent,
		LastAppliedPercent:     a.state.fanPercent,
		FeedbackAvailable:      false,
		Disks:                  diskInfos(temperatures.Disks),
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
			log.Printf("[OK] I²C controller reachable")
		} else {
			log.Printf("[ERROR] I²C controller unreachable: %v", statusErr)
		}
	}
	if shouldHistory {
		a.appendHistory(store.HistoryPoint{
			Time:        now,
			Temperature: reading.Temperature,
			FanPercent:  result.Percent,
			Mode:        result.Mode,
			Operation:   operation,
		})
	}
	if modeChanged {
		a.appendEvent(store.Event{
			Time:     now,
			Type:     "mode",
			Message:  fmt.Sprintf("Mode=%s, Array=%s", result.Mode, operation),
			Severity: severityForMode(result.Mode),
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
