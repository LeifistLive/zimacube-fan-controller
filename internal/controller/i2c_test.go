package controller

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAddress(t *testing.T) {
	cases := map[string]string{
		"0x69": "0x69",
		"0X69": "0x69",
		" 69 ": "0x69",
		"69":   "0x69",
		"6f":   "0x6f",
		"":     "0x69",
		"zz":   "zz",
		"1":    "1",
		"0x99": "0x99",
	}
	for input, expected := range cases {
		if got := NormalizeAddress(input); got != expected {
			t.Errorf("NormalizeAddress(%q) = %q, erwartet %q", input, got, expected)
		}
	}
}

func TestValidAddress(t *testing.T) {
	valid := []string{"0x03", "0x69", "0x77", "69"}
	for _, address := range valid {
		if !ValidAddress(address) {
			t.Errorf("%q sollte gültig sein", address)
		}
	}
	invalid := []string{"", "0x02", "0x78", "zz", "0x100"}
	for _, address := range invalid {
		if ValidAddress(address) {
			t.Errorf("%q sollte ungültig sein", address)
		}
	}
}

func TestAddressPresent(t *testing.T) {
	header := "     0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f\n"

	if !addressPresent(header+"60:                   69                        \n", "0x69") {
		t.Error("Adresse 0x69 wurde nicht erkannt")
	}
	if !addressPresent(header+"60:                   UU\n", "0x69") {
		t.Error("belegte Adresse (UU) wurde nicht erkannt")
	}
	if addressPresent(header+"60:                   --                        \n", "0x69") {
		t.Error("fehlende Adresse wurde als vorhanden gemeldet")
	}
	// Letzte Spalte einer Zeile, früher wurde hier " 6f " nicht gefunden.
	if !addressPresent(header+"60: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- 6f\n", "0x6f") {
		t.Error("Adresse in der letzten Spalte wurde nicht erkannt")
	}
	if addressPresent("", "0x69") {
		t.Error("leere Ausgabe darf keine Adresse melden")
	}
}

func TestSetPercentRejectsOutOfRange(t *testing.T) {
	device := NewI2C(0, "0x69", time.Second, 3)
	for _, percent := range []int{-1, 0, 101, 255} {
		if err := device.SetPercent(context.Background(), percent); err == nil {
			t.Errorf("Prozentwert %d hätte abgelehnt werden müssen", percent)
		}
	}
}

// Regression: retries < 1 ließ SetPercent ohne Schreibvorgang Erfolg melden.
func TestSetPercentAlwaysAttemptsWrite(t *testing.T) {
	device := NewI2C(0, "0x69", 500*time.Millisecond, 0)
	if device.retries < 1 {
		t.Fatalf("retries wurde nicht auf mindestens 1 angehoben: %d", device.retries)
	}
	err := device.SetPercent(context.Background(), 50)
	if err == nil {
		t.Fatal("ohne erreichbaren Bus muss SetPercent einen Fehler liefern")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("Fehlermeldung ist leer")
	}
}

func TestSetPercentHonoursCancelledContext(t *testing.T) {
	device := NewI2C(0, "0x69", 500*time.Millisecond, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if err := device.SetPercent(ctx, 50); err == nil {
		t.Fatal("abgebrochener Context muss zu einem Fehler führen")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Abbruch dauerte %s, Backoff ignoriert den Context", elapsed)
	}
}

func TestBackoffIsCapped(t *testing.T) {
	if got := backoff(1); got != 250*time.Millisecond {
		t.Errorf("backoff(1) = %s", got)
	}
	if got := backoff(100); got != 2*time.Second {
		t.Errorf("backoff(100) = %s, erwartet 2s", got)
	}
}
