package app

import "testing"

func floorTestProfile() Profile {
	return Profile{
		Name:                 "Balanced",
		ArrayBoostPercent:    90,
		EmergencyTemperature: 52,
		EmergencyPercent:     100,
	}
}

func TestFanTestFloorParityCheck(t *testing.T) {
	profile := floorTestProfile()
	status := Status{TemperatureValid: true, MaximumDiskTemperature: 30, ArrayOperation: "parity-check"}
	if got := fanTestFloor(profile, status); got != profile.ArrayBoostPercent {
		t.Fatalf("floor = %d, expected array boost minimum %d", got, profile.ArrayBoostPercent)
	}
}

func TestFanTestFloorRebuild(t *testing.T) {
	profile := floorTestProfile()
	status := Status{TemperatureValid: true, MaximumDiskTemperature: 30, ArrayOperation: "rebuild"}
	if got := fanTestFloor(profile, status); got != profile.ArrayBoostPercent {
		t.Fatalf("floor = %d, expected array boost minimum %d", got, profile.ArrayBoostPercent)
	}
}

func TestFanTestFloorEmergencyTemperature(t *testing.T) {
	profile := floorTestProfile()
	status := Status{TemperatureValid: true, MaximumDiskTemperature: 55, ArrayOperation: "none"}
	if got := fanTestFloor(profile, status); got != profile.EmergencyPercent {
		t.Fatalf("floor = %d, expected emergency minimum %d", got, profile.EmergencyPercent)
	}
}

func TestFanTestFloorSensorFailure(t *testing.T) {
	profile := floorTestProfile()
	status := Status{TemperatureValid: false, ArrayOperation: "none"}
	if got := fanTestFloor(profile, status); got != failsafePercent(profile) {
		t.Fatalf("floor = %d, expected failsafe minimum %d", got, failsafePercent(profile))
	}
}

func TestFanTestFloorNoConstraints(t *testing.T) {
	profile := floorTestProfile()
	status := Status{TemperatureValid: true, MaximumDiskTemperature: 30, ArrayOperation: "none"}
	if got := fanTestFloor(profile, status); got != 1 {
		t.Fatalf("floor = %d, expected the minimum of 1 with no constraints", got)
	}
}
