package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type I2C struct {
	bus     int
	address string
	timeout time.Duration
	retries int
	mu      sync.Mutex
}

func NewI2C(bus int, address string, timeout time.Duration, retries int) *I2C {
	return &I2C{bus: bus, address: address, timeout: timeout, retries: retries}
}

func (i *I2C) Probe(parent context.Context) (bool, error) {
	device := fmt.Sprintf("/dev/i2c-%d", i.bus)
	if _, err := os.Stat(device); err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(parent, i.timeout)
	defer cancel()

	out, err := exec.CommandContext(
		ctx,
		"i2cdetect",
		"-y",
		strconv.Itoa(i.bus),
		i.address,
		i.address,
	).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("i2cdetect: %w: %s", err, strings.TrimSpace(string(out)))
	}

	needle := strings.TrimPrefix(strings.ToLower(i.address), "0x")
	text := strings.ToLower(string(out))
	return strings.Contains(text, " "+needle+" ") || strings.Contains(text, " uu "), nil
}

func (i *I2C) SetPercent(parent context.Context, percent int) error {
	if percent < 1 || percent > 100 {
		return fmt.Errorf("percent must be between 1 and 100")
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	hex := fmt.Sprintf("0x%02x", percent)
	var lastErr error

	for attempt := 1; attempt <= i.retries; attempt++ {
		ctx, cancel := context.WithTimeout(parent, i.timeout)
		out, err := exec.CommandContext(
			ctx,
			"i2cset",
			"-f",
			"-y",
			strconv.Itoa(i.bus),
			i.address,
			"0x04",
			"0x01",
			hex,
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
		lastErr = fmt.Errorf("attempt %d: %w: %s", attempt, err, strings.TrimSpace(string(out)))
		time.Sleep(time.Second)
	}
	return lastErr
}
