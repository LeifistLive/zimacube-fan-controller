package controller

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Point struct {
	Temperature int `json:"temperature"`
	Percent     int `json:"percent"`
}

type Curve []Point

func ParseCurve(raw string) (Curve, error) {
	parts := strings.Split(raw, ",")
	curve := make(Curve, 0, len(parts))

	for _, part := range parts {
		fields := strings.Split(strings.TrimSpace(part), ":")
		if len(fields) != 2 {
			return nil, fmt.Errorf("ungültiger Kurvenpunkt %q", part)
		}
		temp, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("ungültige Temperatur in %q", part)
		}
		percent, err := strconv.Atoi(fields[1])
		if err != nil || percent < 1 || percent > 100 {
			return nil, fmt.Errorf("ungültiger Prozentwert in %q", part)
		}
		curve = append(curve, Point{Temperature: temp, Percent: percent})
	}

	sort.Slice(curve, func(i, j int) bool {
		return curve[i].Temperature < curve[j].Temperature
	})
	if len(curve) == 0 {
		return nil, fmt.Errorf("Lüfterkurve ist leer")
	}
	return curve, nil
}

func (c Curve) Speed(temp int) int {
	speed := c[0].Percent
	for _, point := range c {
		if temp >= point.Temperature {
			speed = point.Percent
		}
	}
	return speed
}

func (c Curve) ThresholdForSpeed(speed int) int {
	threshold := c[0].Temperature
	for _, point := range c {
		if point.Percent <= speed {
			threshold = point.Temperature
		}
	}
	return threshold
}

func (c Curve) String() string {
	parts := make([]string, 0, len(c))
	for _, point := range c {
		parts = append(parts, fmt.Sprintf("%d:%d", point.Temperature, point.Percent))
	}
	return strings.Join(parts, ",")
}

func (c Curve) MarshalJSON() ([]byte, error) {
	type alias Curve
	return json.Marshal(alias(c))
}
