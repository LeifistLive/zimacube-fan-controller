package app

import (
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/controller"
)

// Version is reported through the API and shown in the web UI. It is set at
// build time via -ldflags "-X .../internal/app.Version=..." (see Dockerfile),
// which in turn reads the repo-root VERSION file unless the VERSION build arg
// is explicitly overridden (CI does this from the git tag, see ghcr.yml).
// "dev" is what `go run`/`go test` and unlinked builds fall back to.
var Version = "dev"

// Modes reported in Status.Mode.
const (
	ModeAutomatic  = "automatic"
	ModeManual     = "manual"
	ModeEmergency  = "emergency"
	ModeArrayBoost = "array-boost"
	ModeFailsafe   = "failsafe"
)

// currentConfigVersion is bumped whenever RuntimeConfig's on-disk shape
// changes in a way that needs migration. normalizeRuntimeConfig is the one
// place that translates an older (or missing) version into the current one.
const currentConfigVersion = 1

type Profile struct {
	Name                 string           `json:"name"`
	Curve                controller.Curve `json:"curve"`
	ArrayBoostPercent    int              `json:"array_boost_percent"`
	EmergencyTemperature int              `json:"emergency_temperature"`
	EmergencyPercent     int              `json:"emergency_percent"`
	HysteresisC          int              `json:"hysteresis_c"`
}

type RuntimeConfig struct {
	ConfigVersion int                `json:"config_version"`
	ActiveProfile string             `json:"active_profile"`
	Profiles      map[string]Profile `json:"profiles"`
}

type Override struct {
	Mode    string `json:"mode"`
	Percent int    `json:"percent,omitempty"`
}

type Status struct {
	Version       string `json:"version"`
	Mode          string `json:"mode"`
	ActiveProfile string `json:"active_profile"`
	I2CBus        int    `json:"i2c_bus"`
	I2CAddress    string `json:"i2c_address"`
	// FanPercent is a deprecated alias for TargetPercent, kept so existing
	// API consumers do not break. New code should read TargetPercent and
	// LastAppliedPercent instead, since a failed I2C write means those two
	// differ.
	FanPercent             int    `json:"fan_percent"`
	TargetPercent          int    `json:"target_percent"`
	LastAppliedPercent     int    `json:"last_applied_percent"`
	MaximumDiskTemperature int    `json:"maximum_disk_temperature"`
	TemperatureValid       bool   `json:"temperature_valid"`
	DisksReporting         int    `json:"disks_reporting"`
	ArrayOperation         string `json:"array_operation"`
	Reason                 string `json:"reason"`
	ControllerOnline       bool   `json:"controller_online"`
	LastWriteSuccessful    bool   `json:"last_write_successful"`
	// FeedbackAvailable is always false: the backplane controller has no
	// RPM/PWM readback, so LastAppliedPercent is the last value this
	// service successfully wrote, not a confirmed hardware reading.
	FeedbackAvailable bool       `json:"feedback_available"`
	Disks             []DiskInfo `json:"disks"`
	LastError         string     `json:"last_error,omitempty"`
	UptimeSeconds     int64      `json:"uptime_seconds"`
	Updated           time.Time  `json:"updated"`
}

// DiskInfo is one HDD's live temperature, shown as its own tile in the web
// UI. Cache/flash devices are already filtered out by
// unraid.ReadDiskTemperatures before this is built.
type DiskInfo struct {
	Name        string `json:"name"`
	Temperature int    `json:"temperature"`
	Valid       bool   `json:"valid"`
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
