package app

import (
	"strings"
	"testing"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
)

func TestDefaultRuntimeConfigIsValid(t *testing.T) {
	if _, err := normalizeRuntimeConfig(defaultRuntimeConfig()); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
}

// Regression: curves from JSON were not validated or sorted, even though
// Speed and ThresholdForSpeed expect sorted points.
func TestNormalizeSortsCurvesFromJSON(t *testing.T) {
	input := RuntimeConfig{
		ActiveProfile: "custom",
		Profiles: map[string]Profile{
			"custom": {
				Curve: controller.Curve{
					{Temperature: 48, Percent: 100},
					{Temperature: 0, Percent: 60},
				},
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				HysteresisC:          2,
			},
		},
	}

	normalized, err := normalizeRuntimeConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	profile := normalized.Profiles["custom"]
	if profile.Curve[0].Temperature != 0 {
		t.Fatalf("curve not sorted: %s", profile.Curve)
	}
	if got := profile.Curve.Speed(10); got != 60 {
		t.Fatalf("Speed(10) = %d, expected 60", got)
	}
	if profile.Name != "custom" {
		t.Fatalf("empty display name was not replaced: %q", profile.Name)
	}
}

func TestNormalizeRejectsBrokenProfiles(t *testing.T) {
	good, err := controller.ParseCurve("0:60,48:100")
	if err != nil {
		t.Fatalf("curve: %v", err)
	}
	base := Profile{
		Curve:                good,
		ArrayBoostPercent:    100,
		EmergencyTemperature: 52,
		EmergencyPercent:     100,
		HysteresisC:          2,
	}

	withProfile := func(change func(p *Profile)) RuntimeConfig {
		profile := base
		profile.Curve = append(controller.Curve(nil), good...)
		change(&profile)
		return RuntimeConfig{ActiveProfile: "p", Profiles: map[string]Profile{"p": profile}}
	}

	cases := map[string]RuntimeConfig{
		"empty profile list": {ActiveProfile: "p", Profiles: map[string]Profile{}},
		"active profile missing": {
			ActiveProfile: "gone",
			Profiles:      map[string]Profile{"p": base},
		},
		"empty profile name": {
			ActiveProfile: "p",
			Profiles:      map[string]Profile{"": base},
		},
		"empty curve":                   withProfile(func(p *Profile) { p.Curve = nil }),
		"invalid curve percent":         withProfile(func(p *Profile) { p.Curve[0].Percent = 0 }),
		"invalid boost":                 withProfile(func(p *Profile) { p.ArrayBoostPercent = 0 }),
		"invalid emergency percent":     withProfile(func(p *Profile) { p.EmergencyPercent = 120 }),
		"emergency temperature too low": withProfile(func(p *Profile) { p.EmergencyTemperature = 5 }),
		"negative hysteresis":           withProfile(func(p *Profile) { p.HysteresisC = -1 }),
		"display name too long":         withProfile(func(p *Profile) { p.Name = strings.Repeat("x", maxProfileNameLength+1) }),
		"profile name (key) too long": {
			ActiveProfile: strings.Repeat("k", maxProfileNameLength+1),
			Profiles:      map[string]Profile{strings.Repeat("k", maxProfileNameLength+1): base},
		},
	}

	for name, config := range cases {
		if _, err := normalizeRuntimeConfig(config); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// Regression: the config was merged into the defaults, so deleted
// profiles reappeared after a restart.
func TestLoadRuntimeConfigDoesNotMergeDefaults(t *testing.T) {
	st := store.New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	curve, err := controller.ParseCurve("0:50,50:100")
	if err != nil {
		t.Fatalf("curve: %v", err)
	}
	saved := RuntimeConfig{
		ActiveProfile: "only-one",
		Profiles: map[string]Profile{
			"only-one": {
				Name:                 "Only one",
				Curve:                curve,
				ArrayBoostPercent:    80,
				EmergencyTemperature: 55,
				EmergencyPercent:     100,
				HysteresisC:          3,
			},
		},
	}
	if err := st.SaveJSON("config.json", saved); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	loaded, _, err := loadRuntimeConfig(st)
	if err != nil {
		t.Fatalf("loadRuntimeConfig: %v", err)
	}
	// "target-temp" is expected here: ConfigVersion is 0 (pre-version-2), so
	// the one-time migration backfills it (see
	// TestNormalizeBackfillsTargetTempProfileBelowVersion2). Every other
	// deleted default must stay gone.
	if len(loaded.Profiles) != 2 {
		t.Fatalf("expected exactly two profiles (only-one + migrated target-temp), have %d: %v",
			len(loaded.Profiles), loaded.Profiles)
	}
	if _, ok := loaded.Profiles["target-temp"]; !ok {
		t.Fatal("expected target-temp to be backfilled for a pre-version-2 config")
	}
	if _, ok := loaded.Profiles["balanced"]; ok {
		t.Fatal("deleted default profile came back")
	}
	if _, ok := loaded.Profiles["silent"]; ok {
		t.Fatal("deleted default profile came back")
	}
}

// Regression: an existing config.json with an inconsistent safety-percentage
// ordering (see TestNormalizeRejectsEmergencyBelowArrayBoost) must be
// repaired and loaded, not discarded wholesale in favor of hardcoded
// defaults - unlike POST /api/config, which still rejects the same
// inconsistency outright.
func TestLoadRuntimeConfigRepairsSafetyOrdering(t *testing.T) {
	st := store.New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	saved := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:50,50:100"),
				ArrayBoostPercent:    80,
				EmergencyTemperature: 55,
				EmergencyPercent:     50,
			},
		},
	}
	if err := st.SaveJSON("config.json", saved); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	loaded, notes, err := loadRuntimeConfig(st)
	if err != nil {
		t.Fatalf("loadRuntimeConfig: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected a repair note for the inconsistent emergency_percent")
	}
	if got := loaded.Profiles["p"].EmergencyPercent; got != 80 {
		t.Fatalf("emergency_percent = %d, expected it raised to array_boost_percent (80)", got)
	}
}

func TestLoadRuntimeConfigRejectsBrokenFile(t *testing.T) {
	st := store.New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	broken := RuntimeConfig{
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {ArrayBoostPercent: 100, EmergencyPercent: 100, EmergencyTemperature: 52},
		},
	}
	if err := st.SaveJSON("config.json", broken); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	if _, _, err := loadRuntimeConfig(st); err == nil {
		t.Fatal("a profile without a curve must be rejected")
	}
}

// Regression: a config.json written before config_version was introduced
// must still load after an update, without requiring a user to intervene
// manually.
func TestNormalizeMigratesMissingConfigVersion(t *testing.T) {
	input := RuntimeConfig{
		// ConfigVersion intentionally not set (zero value, like a
		// config.json from before this change).
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
			},
		},
	}
	normalized, err := normalizeRuntimeConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized.ConfigVersion != currentConfigVersion {
		t.Fatalf("ConfigVersion = %d, expected %d", normalized.ConfigVersion, currentConfigVersion)
	}
}

// Regression: a config.json saved before config_version 2 (i.e. before the
// Target Temp profile existed) must gain it automatically, so upgrading
// installations see it without a manual JSON edit.
func TestNormalizeBackfillsTargetTempProfileBelowVersion2(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: 1,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
			},
		},
	}
	normalized, err := normalizeRuntimeConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := normalized.Profiles["target-temp"]; !ok {
		t.Fatal("expected target-temp profile to be backfilled for a pre-version-2 config")
	}
}

// Regression: once a config is saved at version 2, a deliberately removed
// (or renamed) target-temp profile must not keep reappearing on restart.
func TestNormalizeDoesNotReintroduceTargetTempAtVersion2(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: 2,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
			},
		},
	}
	normalized, err := normalizeRuntimeConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := normalized.Profiles["target-temp"]; ok {
		t.Fatal("a config already at version 2 must not have target-temp reintroduced")
	}
}

func TestNormalizeRejectsOutOfRangeTargetMinimumPercent(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				TargetTemperature:    40,
				TargetMinimumPercent: 101,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err == nil {
		t.Fatal("target_minimum_percent above 100 must be rejected")
	}
}

// Regression: safety percentages were only range-checked individually, so
// emergency_percent lower than array_boost_percent was accepted - reaching
// the emergency temperature during an array operation would then actually
// slow the fans down instead of speeding them up.
func TestNormalizeRejectsEmergencyBelowArrayBoost(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    80,
				EmergencyTemperature: 52,
				EmergencyPercent:     50,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err == nil {
		t.Fatal("emergency_percent lower than array_boost_percent must be rejected")
	}
}

// Regression: same issue as above, but against the Target Temp floor -
// reaching the emergency temperature on a target-temp profile could
// otherwise end up slower than the floor that is supposed to apply even in
// normal operation.
func TestNormalizeRejectsEmergencyBelowTargetMinimum(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     20,
				TargetTemperature:    40,
				TargetMinimumPercent: 30,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err == nil {
		t.Fatal("emergency_percent lower than target_minimum_percent must be rejected")
	}
}

// Regression: array_boost_percent lower than target_minimum_percent is
// accepted range-wise but contradicts the floor's purpose (a "minimum" that
// array-boost could undercut).
func TestNormalizeRejectsArrayBoostBelowTargetMinimum(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    20,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
				TargetTemperature:    40,
				TargetMinimumPercent: 30,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err == nil {
		t.Fatal("array_boost_percent lower than target_minimum_percent must be rejected")
	}
}

// A curve-based (non-target-temp) profile must not be held to the
// target_minimum_percent ordering, since that floor never applies to it.
func TestNormalizeAllowsLowArrayBoostWithoutTargetTemperature(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    20,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err != nil {
		t.Fatalf("unexpected error for a profile without target_temperature: %v", err)
	}
}

func TestNormalizeRejectsFutureConfigVersion(t *testing.T) {
	input := RuntimeConfig{
		ConfigVersion: currentConfigVersion + 1,
		ActiveProfile: "p",
		Profiles: map[string]Profile{
			"p": {
				Curve:                mustCurve(t, "0:60,48:100"),
				ArrayBoostPercent:    100,
				EmergencyTemperature: 52,
				EmergencyPercent:     100,
			},
		},
	}
	if _, err := normalizeRuntimeConfig(input); err == nil {
		t.Fatal("a config_version newer than supported must be rejected")
	}
}

func mustCurve(t *testing.T, raw string) controller.Curve {
	t.Helper()
	curve, err := controller.ParseCurve(raw)
	if err != nil {
		t.Fatalf("curve: %v", err)
	}
	return curve
}

func TestSanitizedClampsConfig(t *testing.T) {
	cfg := Config{
		I2CBus:              -1,
		I2CAddress:          "69",
		I2CRetries:          0,
		CheckInterval:       0,
		HistoryInterval:     0,
		SafeShutdownPercent: 500,
	}.sanitized()

	if cfg.I2CBus != 0 {
		t.Errorf("I2CBus = %d", cfg.I2CBus)
	}
	if cfg.I2CAddress != "0x69" {
		t.Errorf("I2CAddress = %q", cfg.I2CAddress)
	}
	if cfg.I2CRetries < 1 {
		t.Errorf("I2CRetries = %d", cfg.I2CRetries)
	}
	if cfg.CheckInterval <= 0 {
		t.Errorf("CheckInterval = %s", cfg.CheckInterval)
	}
	if cfg.HistoryInterval < cfg.CheckInterval {
		t.Errorf("HistoryInterval = %s", cfg.HistoryInterval)
	}
	if cfg.SafeShutdownPercent != controller.MaxPercent {
		t.Errorf("SafeShutdownPercent = %d", cfg.SafeShutdownPercent)
	}
	if cfg.DataDir == "" || cfg.ListenAddress == "" || cfg.DetectInterval <= 0 || cfg.ReapplyInterval <= 0 {
		t.Errorf("default values missing: %+v", cfg)
	}
}
