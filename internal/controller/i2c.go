package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	detectTool = "i2cdetect"
	setTool    = "i2cset"
)

// Usable range for 7 bit I²C addresses.
const (
	lowestAddress  = 0x03
	highestAddress = 0x77
)

type I2C struct {
	bus     int
	address string
	timeout time.Duration
	retries int
	mu      sync.Mutex
}

// NewI2C clamps its arguments so that a bad environment variable can never
// disable writing altogether (retries < 1 used to make SetPercent a no-op that
// still reported success).
func NewI2C(bus int, address string, timeout time.Duration, retries int) *I2C {
	if bus < 0 {
		bus = 0
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if retries < 1 {
		retries = 1
	}
	return &I2C{
		bus:     bus,
		address: NormalizeAddress(address),
		timeout: timeout,
		retries: retries,
	}
}

// NormalizeAddress returns the canonical 0x form used by i2c-tools. Values
// without a 0x prefix are read as hexadecimal, because that is how i2cdetect
// prints them ("69" and "0x69" therefore mean the same address). Unparsable or
// out-of-range input is returned unchanged so that ValidAddress can reject it
// with a useful message at startup.
func NormalizeAddress(address string) string {
	trimmed := strings.TrimSpace(strings.ToLower(address))
	if trimmed == "" {
		return "0x69"
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(trimmed, "0x"), 16, 16)
	if err != nil {
		return trimmed
	}
	if value < lowestAddress || value > highestAddress {
		return trimmed
	}
	return fmt.Sprintf("0x%02x", value)
}

// ValidAddress reports whether the address is a usable 7 bit I²C address.
func ValidAddress(address string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(address))
	value, err := strconv.ParseUint(strings.TrimPrefix(trimmed, "0x"), 16, 16)
	if err != nil {
		return false
	}
	return value >= lowestAddress && value <= highestAddress
}

func (i *I2C) Address() string {
	return i.address
}

func (i *I2C) Device() string {
	return fmt.Sprintf("/dev/i2c-%d", i.bus)
}

// DeviceAvailable is the cheap liveness check: no subprocess, no bus traffic.
// Called on every cycle instead of the old i2cdetect probe.
func (i *I2C) DeviceAvailable() error {
	if _, err := os.Stat(i.Device()); err != nil {
		return err
	}
	return nil
}

// Detect asks i2cdetect whether the controller answers. This puts traffic on
// the bus, so callers should use it sparingly (startup, after write failures).
func (i *I2C) Detect(parent context.Context) (bool, error) {
	if err := i.DeviceAvailable(); err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(parent, i.timeout)
	defer cancel()

	i.mu.Lock()
	out, err := exec.CommandContext(
		ctx,
		detectTool,
		"-y",
		strconv.Itoa(i.bus),
		i.address,
		i.address,
	).CombinedOutput()
	i.mu.Unlock()

	if err != nil {
		return false, fmt.Errorf("%s: %w: %s", detectTool, err, strings.TrimSpace(string(out)))
	}
	return addressPresent(string(out), i.address), nil
}

// addressPresent parses the i2cdetect grid cell by cell. The previous
// implementation searched for " 69 " in the raw output, which missed addresses
// printed in the last column of a row and matched "UU" anywhere.
func addressPresent(output, address string) bool {
	needle := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(address)), "0x")
	for _, line := range strings.Split(output, "\n") {
		_, cells, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		for _, cell := range strings.Fields(cells) {
			cell = strings.ToLower(cell)
			if cell == needle || cell == "uu" {
				return true
			}
		}
	}
	return false
}

// SetPercent writes the fan speed via i2cset. Retries use a context aware
// backoff so that a shutdown is not delayed by sleeping goroutines.
func (i *I2C) SetPercent(parent context.Context, percent int) error {
	if percent < MinPercent || percent > MaxPercent {
		return fmt.Errorf("Prozentwert %d liegt nicht zwischen %d und %d", percent, MinPercent, MaxPercent)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	value := fmt.Sprintf("0x%02x", percent)
	var lastErr error

	for attempt := 1; attempt <= i.retries; attempt++ {
		if err := parent.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		ctx, cancel := context.WithTimeout(parent, i.timeout)
		out, err := exec.CommandContext(
			ctx,
			setTool,
			"-f",
			"-y",
			strconv.Itoa(i.bus),
			i.address,
			"0x04",
			"0x01",
			value,
			"0x00",
			"0x00",
			"0x00",
			"0x00",
			"0x01",
			"0x00",
			"i",
		).CombinedOutput()
		cancel()

		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("Versuch %d/%d: %w: %s", attempt, i.retries, err, strings.TrimSpace(string(out)))
		if attempt == i.retries {
			break
		}

		select {
		case <-parent.Done():
			return lastErr
		case <-time.After(backoff(attempt)):
		}
	}

	if lastErr == nil {
		lastErr = errors.New("i2cset wurde nicht ausgeführt")
	}
	return lastErr
}

func backoff(attempt int) time.Duration {
	delay := time.Duration(attempt) * 250 * time.Millisecond
	if delay > 2*time.Second {
		delay = 2 * time.Second
	}
	return delay
}
