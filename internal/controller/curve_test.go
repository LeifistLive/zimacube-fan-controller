package controller

import "testing"

func TestParseCurveSortsAndValidates(t *testing.T) {
	curve, err := ParseCurve("48:100, 0:60 ,40:75")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(curve) != 3 {
		t.Fatalf("expected 3 points, have %d", len(curve))
	}
	if curve[0].Temperature != 0 || curve[2].Temperature != 48 {
		t.Fatalf("curve not sorted: %s", curve)
	}
}

func TestParseCurveRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"empty":              "",
		"no colon":           "40",
		"not a number":       "abc:50",
		"percent too small":  "40:0",
		"percent too large":  "40:120",
		"duplicate temp":     "40:50,40:60",
		"decreasing percent": "30:80,40:50",
		"unrealistic temp":   "500:50",
	}
	for name, raw := range cases {
		if _, err := ParseCurve(raw); err == nil {
			t.Errorf("%s: expected error for %q", name, raw)
		}
	}
}

func TestNormalizedDoesNotMutateInput(t *testing.T) {
	input := Curve{{Temperature: 48, Percent: 100}, {Temperature: 0, Percent: 60}}
	if _, err := input.Normalized(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input[0].Temperature != 48 {
		t.Fatalf("input was mutated: %s", input)
	}
}

func TestSpeed(t *testing.T) {
	curve, err := ParseCurve("0:60,36:65,40:75,43:85,46:95,48:100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cases := map[int]int{-5: 60, 0: 60, 35: 60, 36: 65, 39: 65, 40: 75, 47: 95, 48: 100, 99: 100}
	for temperature, expected := range cases {
		if got := curve.Speed(temperature); got != expected {
			t.Errorf("Speed(%d) = %d, expected %d", temperature, got, expected)
		}
	}
}

func TestSpeedOnEmptyCurveIsSafe(t *testing.T) {
	var curve Curve
	if got := curve.Speed(30); got != MaxPercent {
		t.Fatalf("empty curve must return %d, has %d", MaxPercent, got)
	}
	if got := curve.ThresholdForSpeed(50); got != 0 {
		t.Fatalf("empty curve must return threshold 0, has %d", got)
	}
}

func TestThresholdForSpeed(t *testing.T) {
	curve, err := ParseCurve("0:60,36:65,40:75,43:85,46:95,48:100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := curve.ThresholdForSpeed(65); got != 36 {
		t.Errorf("ThresholdForSpeed(65) = %d, expected 36", got)
	}
	if got := curve.ThresholdForSpeed(100); got != 48 {
		t.Errorf("ThresholdForSpeed(100) = %d, expected 48", got)
	}
	if got := curve.ThresholdForSpeed(10); got != 0 {
		t.Errorf("ThresholdForSpeed(10) = %d, expected 0", got)
	}
}

func TestHighest(t *testing.T) {
	curve, err := ParseCurve("0:45,48:90")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := curve.Highest(); got != 90 {
		t.Fatalf("Highest() = %d, expected 90", got)
	}
}
