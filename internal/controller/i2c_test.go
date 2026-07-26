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
			t.Errorf("NormalizeAddress(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestValidAddress(t *testing.T) {
	valid := []string{"0x03", "0x69", "0x77", "69"}
	for _, address := range valid {
		if !ValidAddress(address) {
			t.Errorf("%q should be valid", address)
		}
	}
	invalid := []string{"", "0x02", "0x78", "zz", "0x100"}
	for _, address := range invalid {
		if ValidAddress(address) {
			t.Errorf("%q should be invalid", address)
		}
	}
}

func TestAddressPresent(t *testing.T) {
	header := "     0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f\n"

	if !addressPresent(header+"60:                   69                        \n", "0x69") {
		t.Error("address 0x69 was not detected")
	}
	if !addressPresent(header+"60:                   UU\n", "0x69") {
		t.Error("busy address (UU) was not detected")
	}
	if addressPresent(header+"60:                   --                        \n", "0x69") {
		t.Error("missing address was reported as present")
	}
	// Last column of a line, " 6f " used to not be found here.
	if !addressPresent(header+"60: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- 6f\n", "0x6f") {
		t.Error("address in the last column was not detected")
	}
	if addressPresent("", "0x69") {
		t.Error("empty output must not report an address")
	}
}

func TestSetPercentRejectsOutOfRange(t *testing.T) {
	device := NewI2C(0, "0x69", time.Second, 3)
	for _, percent := range []int{-1, 0, 101, 255} {
		if err := device.SetPercent(context.Background(), percent); err == nil {
			t.Errorf("percent value %d should have been rejected", percent)
		}
	}
}

// Regression: retries < 1 let SetPercent report success without a write.
func TestSetPercentAlwaysAttemptsWrite(t *testing.T) {
	device := NewI2C(0, "0x69", 500*time.Millisecond, 0)
	if device.retries < 1 {
		t.Fatalf("retries was not raised to at least 1: %d", device.retries)
	}
	err := device.SetPercent(context.Background(), 50)
	if err == nil {
		t.Fatal("SetPercent must return an error without a reachable bus")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("error message is empty")
	}
}

func TestSetPercentHonoursCancelledContext(t *testing.T) {
	device := NewI2C(0, "0x69", 500*time.Millisecond, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if err := device.SetPercent(ctx, 50); err == nil {
		t.Fatal("a cancelled context must result in an error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancellation took %s, backoff is ignoring the context", elapsed)
	}
}

func TestBackoffIsCapped(t *testing.T) {
	if got := backoff(1); got != 250*time.Millisecond {
		t.Errorf("backoff(1) = %s", got)
	}
	if got := backoff(100); got != 2*time.Second {
		t.Errorf("backoff(100) = %s, expected 2s", got)
	}
}
