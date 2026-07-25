package app

import (
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
)

// Version is reported through the API and shown in the web UI.
const Version = "3.1.0"

// Modes reported in Status.Mode.
const (
	ModeAutomatic  = "automatic"
	ModeManual     = "manual"
	ModeEmergency  = "emergency"
	ModeArrayBoost = "array-boost"
	ModeFailsafe   = "failsafe"
)

type Profile struct {
	Name                 string           `json:"name"`
	Curve                controller.Curve `json:"curve"`
	ArrayBoostPercent    int              `json:"array_boost_percent"`
	EmergencyTemperature int              `json:"emergency_temperature"`
	EmergencyPercent     int              `json:"emergency_percent"`
	HysteresisC          int              `json:"hysteresis_c"`
}

type RuntimeConfig struct {
	ActiveProfile string             `json:"active_profile"`
	Profiles      map[string]Profile `json:"profiles"`
}

type Override struct {
	Mode    string `json:"mode"`
	Percent int    `json:"percent,omitempty"`
}

type Status struct {
	Version                string    `json:"version"`
	Mode                   string    `json:"mode"`
	ActiveProfile          string    `json:"active_profile"`
	FanPercent             int       `json:"fan_percent"`
	MaximumDiskTemperature int       `json:"maximum_disk_temperature"`
	TemperatureValid       bool      `json:"temperature_valid"`
	DisksReporting         int       `json:"disks_reporting"`
	ArrayOperation         string    `json:"array_operation"`
	Reason                 string    `json:"reason"`
	ControllerOnline       bool      `json:"controller_online"`
	LastWriteSuccessful    bool      `json:"last_write_successful"`
	LastError              string    `json:"last_error,omitempty"`
	UptimeSeconds          int64     `json:"uptime_seconds"`
	Updated                time.Time `json:"updated"`
}

// sample is one reading of the inputs that the control loop decides on.
type sample struct {
	Temperature int
	Valid       bool
	Reporting   int
	Operation   string
}

// decision is the pure result of the control logic, see decide.
type decision struct {
	Percent int
	Mode    string
	Reason  string
}
