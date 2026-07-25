package controller

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Percent bounds accepted by the backplane controller.
const (
	MinPercent = 1
	MaxPercent = 100
)

// Temperature bounds used for sanity checks on curve points.
const (
	minTemperature = -50
	maxTemperature = 150
)

type Point struct {
	Temperature int `json:"temperature"`
	Percent     int `json:"percent"`
}

type Curve []Point

// ParseCurve reads the compact "temp:percent,temp:percent" notation.
// The result is always validated and sorted.
func ParseCurve(raw string) (Curve, error) {
	parts := strings.Split(raw, ",")
	curve := make(Curve, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) != 2 {
			return nil, fmt.Errorf("ungültiger Kurvenpunkt %q", part)
		}
		temperature, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("ungültige Temperatur in %q", part)
		}
		percent, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			return nil, fmt.Errorf("ungültiger Prozentwert in %q", part)
		}
		curve = append(curve, Point{Temperature: temperature, Percent: percent})
	}

	return curve.Normalized()
}

// Normalized validates the curve and returns a sorted copy. Every code path
// that accepts a curve from outside (env, config file, REST API) must run it
// through this function, because Speed and ThresholdForSpeed rely on the
// points being sorted and monotonic.
func (c Curve) Normalized() (Curve, error) {
	if len(c) == 0 {
		return nil, errors.New("Lüfterkurve ist leer")
	}

	out := make(Curve, len(c))
	copy(out, c)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Temperature < out[j].Temperature
	})

	for index, point := range out {
		if point.Percent < MinPercent || point.Percent > MaxPercent {
			return nil, fmt.Errorf("Prozentwert %d bei %d Grad liegt nicht zwischen %d und %d",
				point.Percent, point.Temperature, MinPercent, MaxPercent)
		}
		if point.Temperature < minTemperature || point.Temperature > maxTemperature {
			return nil, fmt.Errorf("Temperatur %d Grad liegt nicht zwischen %d und %d",
				point.Temperature, minTemperature, maxTemperature)
		}
		if index == 0 {
			continue
		}
		if point.Temperature == out[index-1].Temperature {
			return nil, fmt.Errorf("doppelte Temperatur %d Grad", point.Temperature)
		}
		if point.Percent < out[index-1].Percent {
			return nil, fmt.Errorf("Prozentwert fällt von %d auf %d bei steigender Temperatur %d Grad",
				out[index-1].Percent, point.Percent, point.Temperature)
		}
	}

	return out, nil
}

// Validate reports whether the curve is usable without copying it.
func (c Curve) Validate() error {
	_, err := c.Normalized()
	return err
}

// Speed returns the fan percentage for a temperature. An empty curve must not
// happen after validation; if it does, fail loud instead of panicking.
func (c Curve) Speed(temperature int) int {
	if len(c) == 0 {
		return MaxPercent
	}
	speed := c[0].Percent
	for _, point := range c {
		if temperature >= point.Temperature {
			speed = point.Percent
		}
	}
	return speed
}

// ThresholdForSpeed returns the highest temperature that still maps to at most
// the given speed. Used as the release point for the hysteresis.
func (c Curve) ThresholdForSpeed(speed int) int {
	if len(c) == 0 {
		return 0
	}
	threshold := c[0].Temperature
	for _, point := range c {
		if point.Percent <= speed {
			threshold = point.Temperature
		}
	}
	return threshold
}

// Highest returns the largest percentage in the curve.
func (c Curve) Highest() int {
	highest := 0
	for _, point := range c {
		if point.Percent > highest {
			highest = point.Percent
		}
	}
	return highest
}

func (c Curve) String() string {
	parts := make([]string, 0, len(c))
	for _, point := range c {
		parts = append(parts, fmt.Sprintf("%d:%d", point.Temperature, point.Percent))
	}
	return strings.Join(parts, ",")
}
