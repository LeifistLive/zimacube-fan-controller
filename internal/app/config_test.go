package app

import (
	"testing"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
)

func TestDefaultRuntimeConfigIsValid(t *testing.T) {
	if _, err := normalizeRuntimeConfig(defaultRuntimeConfig()); err != nil {
		t.Fatalf("Standardkonfiguration ist ungültig: %v", err)
	}
}

// Regression: Kurven aus JSON wurden nicht geprüft und nicht sortiert, obwohl
// Speed und ThresholdForSpeed sortierte Punkte erwarten.
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
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	profile := normalized.Profiles["custom"]
	if profile.Curve[0].Temperature != 0 {
		t.Fatalf("Kurve nicht sortiert: %s", profile.Curve)
	}
	if got := profile.Curve.Speed(10); got != 60 {
		t.Fatalf("Speed(10) = %d, erwartet 60", got)
	}
	if profile.Name != "custom" {
		t.Fatalf("leerer Anzeigename wurde nicht ersetzt: %q", profile.Name)
	}
}

func TestNormalizeRejectsBrokenProfiles(t *testing.T) {
	good, err := controller.ParseCurve("0:60,48:100")
	if err != nil {
		t.Fatalf("Kurve: %v", err)
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
		"leere Profilliste": {ActiveProfile: "p", Profiles: map[string]Profile{}},
		"aktives Profil fehlt": {
			ActiveProfile: "weg",
			Profiles:      map[string]Profile{"p": base},
		},
		"leerer Profilname": {
			ActiveProfile: "p",
			Profiles:      map[string]Profile{"": base},
		},
		"leere Kurve":               withProfile(func(p *Profile) { p.Curve = nil }),
		"Kurvenprozent ungültig":    withProfile(func(p *Profile) { p.Curve[0].Percent = 0 }),
		"Boost ungültig":            withProfile(func(p *Profile) { p.ArrayBoostPercent = 0 }),
		"Notfallprozent ungültig":   withProfile(func(p *Profile) { p.EmergencyPercent = 120 }),
		"Notfalltemperatur zu tief": withProfile(func(p *Profile) { p.EmergencyTemperature = 5 }),
		"Hysterese negativ":         withProfile(func(p *Profile) { p.HysteresisC = -1 }),
	}

	for name, config := range cases {
		if _, err := normalizeRuntimeConfig(config); err == nil {
			t.Errorf("%s: erwarte Fehler", name)
		}
	}
}

// Regression: die Konfiguration wurde in die Defaults hineingelesen, gelöschte
// Profile tauchten nach einem Neustart wieder auf.
func TestLoadRuntimeConfigDoesNotMergeDefaults(t *testing.T) {
	st := store.New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	curve, err := controller.ParseCurve("0:50,50:100")
	if err != nil {
		t.Fatalf("Kurve: %v", err)
	}
	saved := RuntimeConfig{
		ActiveProfile: "nur-eins",
		Profiles: map[string]Profile{
			"nur-eins": {
				Name:                 "Nur eins",
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

	loaded, err := loadRuntimeConfig(st)
	if err != nil {
		t.Fatalf("loadRuntimeConfig: %v", err)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("erwarte genau ein Profil, habe %d: %v", len(loaded.Profiles), loaded.Profiles)
	}
	if _, ok := loaded.Profiles["balanced"]; ok {
		t.Fatal("gelöschtes Standardprofil ist zurückgekehrt")
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
	if _, err := loadRuntimeConfig(st); err == nil {
		t.Fatal("Profil ohne Kurve muss abgelehnt werden")
	}
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
	if cfg.DataDir == "" || cfg.ListenAddress == "" || cfg.DetectInterval <= 0 {
		t.Errorf("Standardwerte fehlen: %+v", cfg)
	}
}
