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
		t.Fatalf("curve: %v", err)
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
		t.Fatalf("unexpected: %+v", result)
	}

	result = decide(profile, valid(41, unraid.OperationNone), Override{}, 0)
	if result.Percent != 75 {
		t.Fatalf("at 41 degrees expected 75%%, got %d", result.Percent)
	}
}

func TestDecideEmergencyBeatsManual(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(55, unraid.OperationNone), Override{Mode: ModeManual, Percent: 20}, 0)
	if result.Percent != 100 || result.Mode != ModeEmergency {
		t.Fatalf("emergency protection does not kick in: %+v", result)
	}
}

func TestDecideArrayBoostBeatsManual(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, "parity-check"), Override{Mode: ModeManual, Percent: 20}, 0)
	if result.Percent != 90 || result.Mode != ModeArrayBoost {
		t.Fatalf("array boost does not kick in: %+v", result)
	}
}

// At 50 degrees the curve already yields 100 percent, the 90 percent
// boost must not lower that. The emergency threshold is 52 degrees, so
// the mode stays automatic.
func TestDecideArrayBoostDoesNotLowerSpeed(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(50, "parity-check"), Override{}, 0)
	if result.Percent != 100 {
		t.Fatalf("boost lowers the fan speed: %+v", result)
	}
	if result.Mode != ModeAutomatic {
		t.Fatalf("mode = %q, expected %q: %+v", result.Mode, ModeAutomatic, result)
	}
}

// Regression: an unreadable array status ("unknown") must not trigger a
// continuous boost.
func TestDecideUnknownOperationDoesNotBoost(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationUnknown), Override{}, 0)
	if result.Percent != 60 || result.Mode != ModeAutomatic {
		t.Fatalf("unknown array status boosts: %+v", result)
	}
}

func TestDecideManualIsHonoured(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{Mode: ModeManual, Percent: 25}, 0)
	if result.Percent != 25 || result.Mode != ModeManual {
		t.Fatalf("manual setting is ignored: %+v", result)
	}
}

func TestDecideManualEmergencyOverride(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{Mode: ModeEmergency}, 0)
	if result.Percent != 100 || result.Mode != ModeEmergency {
		t.Fatalf("manual emergency mode does not kick in: %+v", result)
	}
}

// Regression: an unreadable disks.ini resulted in the lowest curve step.
func TestDecideFailsafeOnUnknownTemperature(t *testing.T) {
	profile := testProfile(t)
	unknown := sample{Temperature: 0, Valid: false, Operation: unraid.OperationNone}

	result := decide(profile, unknown, Override{}, 0)
	if result.Percent != profile.ArrayBoostPercent || result.Mode != ModeFailsafe {
		t.Fatalf("safety fan speed missing: %+v", result)
	}

	result = decide(profile, unknown, Override{Mode: ModeManual, Percent: 10}, 0)
	if result.Percent != profile.ArrayBoostPercent || result.Mode != ModeFailsafe {
		t.Fatalf("manual setting undercuts the safety fan speed: %+v", result)
	}

	result = decide(profile, unknown, Override{Mode: ModeManual, Percent: 100}, 0)
	if result.Percent != 100 || result.Mode != ModeManual {
		t.Fatalf("a higher manual setting is allowed to stand: %+v", result)
	}
}

func TestDecideHysteresisHoldsAndReleases(t *testing.T) {
	profile := testProfile(t)

	held := decide(profile, valid(35, unraid.OperationNone), Override{}, 65)
	if held.Percent != 65 {
		t.Fatalf("hysteresis does not hold: %+v", held)
	}

	released := decide(profile, valid(33, unraid.OperationNone), Override{}, 65)
	if released.Percent != 60 {
		t.Fatalf("hysteresis does not release: %+v", released)
	}
}

// Regression: after an array boost, hysteresis held the high fan speed.
func TestDecideHysteresisDoesNotHoldBoostValue(t *testing.T) {
	profile := testProfile(t)
	result := decide(profile, valid(30, unraid.OperationNone), Override{}, 100)
	if result.Percent != 60 {
		t.Fatalf("boost fan speed is being held: %+v", result)
	}
}

func TestDecideClampsResult(t *testing.T) {
	profile := testProfile(t)
	profile.EmergencyPercent = 250
	result := decide(profile, valid(60, unraid.OperationNone), Override{}, 0)
	if result.Percent != controller.MaxPercent {
		t.Fatalf("result not clamped: %+v", result)
	}
}

func TestArrayActive(t *testing.T) {
	if arrayActive(unraid.OperationNone) || arrayActive(unraid.OperationUnknown) || arrayActive("") {
		t.Error("none, unknown, and empty must not be active")
	}
	if !arrayActive("parity-check") || !arrayActive("rebuild") {
		t.Error("running operations must be active")
	}
}
