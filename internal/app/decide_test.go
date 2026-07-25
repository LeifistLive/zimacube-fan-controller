package app

import (
	"testing"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
	"github.com/LeifistLive/zimacube-fan-controller/internal/unraid"
)

func testProfile(t *testing.T) Profile {
	t.Helper()
	curve, err := controller.ParseCurve("0:60,36:65,40:75,43:85,46:95,48:100")
	if err != nil {
		t.Fatalf("Kurve: %v", err)
	}
	return Profile{
		Name:                 "Balanced",
		Curve:                curve,
		ArrayBoostPercent:    90,
		EmergencyTemperature: 52,
		EmergencyPercent:     100,
		HysteresisC:          2,
	}
}

func valid(temperature int, operation string) sample {
	return sample{Temperature: temperature, Valid: true, Reporting: 3, Operation: operation}
}

func TestDecideFollowsCurve(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{}, 0)
	if result.Percent != 60 || result.Mode != ModeAutomatic {
		t.Fatalf("unerwartet: %+v", result)
	}

	result = decide(profile, valid(41, unraid.OperationNone), Override{}, 0)
	if result.Percent != 75 {
		t.Fatalf("bei 41 Grad erwarte 75%%, habe %d", result.Percent)
	}
}

func TestDecideEmergencyBeatsManual(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(55, unraid.OperationNone), Override{Mode: ModeManual, Percent: 20}, 0)
	if result.Percent != 100 || result.Mode != ModeEmergency {
		t.Fatalf("Notfallschutz greift nicht: %+v", result)
	}
}

func TestDecideArrayBoostBeatsManual(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, "parity-check"), Override{Mode: ModeManual, Percent: 20}, 0)
	if result.Percent != 90 || result.Mode != ModeArrayBoost {
		t.Fatalf("Array-Boost greift nicht: %+v", result)
	}
}

func TestDecideArrayBoostDoesNotLowerSpeed(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(50, "parity-check"), Override{}, 0)
	if result.Percent != 100 || result.Mode != ModeEmergency {
		t.Fatalf("Boost darf die Drehzahl nicht senken: %+v", result)
	}
}

// Regression: ein unlesbarer Array-Status ("unknown") darf keinen Dauerboost
// auslösen.
func TestDecideUnknownOperationDoesNotBoost(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationUnknown), Override{}, 0)
	if result.Percent != 60 || result.Mode != ModeAutomatic {
		t.Fatalf("unbekannter Array-Status boostet: %+v", result)
	}
}

func TestDecideManualIsHonoured(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{Mode: ModeManual, Percent: 25}, 0)
	if result.Percent != 25 || result.Mode != ModeManual {
		t.Fatalf("manuelle Vorgabe wird ignoriert: %+v", result)
	}
}

func TestDecideManualEmergencyOverride(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{Mode: ModeEmergency}, 0)
	if result.Percent != 100 || result.Mode != ModeEmergency {
		t.Fatalf("manueller Notfallmodus greift nicht: %+v", result)
	}
}

// Regression: eine unlesbare disks.ini führte zur niedrigsten Kurvenstufe.
func TestDecideFailsafeOnUnknownTemperature(t *testing.T) {
	profile := testProfile(t)
	unknown := sample{Temperature: 0, Valid: false, Operation: unraid.OperationNone}

	result := decide(profile, unknown, Override{}, 0)
	if result.Percent != profile.ArrayBoostPercent || result.Mode != ModeFailsafe {
		t.Fatalf("Sicherheitsdrehzahl fehlt: %+v", result)
	}

	result = decide(profile, unknown, Override{Mode: ModeManual, Percent: 10}, 0)
	if result.Percent != profile.ArrayBoostPercent || result.Mode != ModeFailsafe {
		t.Fatalf("manuelle Vorgabe unterläuft die Sicherheitsdrehzahl: %+v", result)
	}

	result = decide(profile, unknown, Override{Mode: ModeManual, Percent: 100}, 0)
	if result.Percent != 100 || result.Mode != ModeManual {
		t.Fatalf("höhere manuelle Vorgabe darf bestehen bleiben: %+v", result)
	}
}

func TestDecideHysteresisHoldsAndReleases(t *testing.T) {
	profile := testProfile(t)

	held := decide(profile, valid(35, unraid.OperationNone), Override{}, 65)
	if held.Percent != 65 {
		t.Fatalf("Hysterese hält nicht: %+v", held)
	}

	released := decide(profile, valid(33, unraid.OperationNone), Override{}, 65)
	if released.Percent != 60 {
		t.Fatalf("Hysterese gibt nicht frei: %+v", released)
	}
}

// Regression: nach einem Array-Boost hielt die Hysterese die hohe Drehzahl.
func TestDecideHysteresisDoesNotHoldBoostValue(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{}, 100)
	if result.Percent != 60 {
		t.Fatalf("Boost-Drehzahl wird festgehalten: %+v", result)
	}
}

func TestDecideClampsResult(t *testing.T) {
	profile := testProfile(t)
	profile.EmergencyPercent = 250
	result := decide(profile, valid(60, unraid.OperationNone), Override{}, 0)
	if result.Percent != controller.MaxPercent {
		t.Fatalf("Ergebnis nicht begrenzt: %+v", result)
	}
}

func TestArrayActive(t *testing.T) {
	if arrayActive(unraid.OperationNone) || arrayActive(unraid.OperationUnknown) || arrayActive("") {
		t.Error("none, unknown und leer dürfen nicht aktiv sein")
	}
	if !arrayActive("parity-check") || !arrayActive("rebuild") {
		t.Error("laufende Operationen müssen aktiv sein")
	}
}
