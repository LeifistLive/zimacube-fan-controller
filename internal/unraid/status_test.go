package unraid

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("file %s: %v", name, err)
	}
	return path
}

func TestReadDiskTemperatures(t *testing.T) {
	path := write(t, "disks.ini", `["disk1"]
name="disk1"
temp="35"
["disk2"]
name="disk2"
temp="*"
["parity"]
name="parity"
temp="41"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Maximum != 41 {
		t.Errorf("Maximum = %d, expected 41", result.Maximum)
	}
	if result.Reporting != 3 {
		t.Errorf("Reporting = %d, expected 3", result.Reporting)
	}
	if result.Parsed != 2 {
		t.Errorf("Parsed = %d, expected 2", result.Parsed)
	}
}

func TestReadDiskTemperaturesAllSpunDown(t *testing.T) {
	path := write(t, "disks.ini", `["disk1"]
temp="*"
["disk2"]
temp="*"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("standby is not an error: %v", err)
	}
	if result.Maximum != 0 || result.Parsed != 0 || result.Reporting != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Disks) != 2 || result.Disks[0].Valid || result.Disks[1].Valid {
		t.Fatalf("standby drives should be present in Disks but marked invalid: %+v", result.Disks)
	}
}

// Regression: a cache/NVMe SSD in disks.ini used to be counted as an HDD
// and could skew the fan curve's target temperature.
func TestReadDiskTemperaturesExcludesCache(t *testing.T) {
	path := write(t, "disks.ini", `["disk1"]
name="disk1"
type="DATA"
temp="35"
["cache"]
name="cache"
type="CACHE"
temp="55"
["flash"]
name="flash"
type="FLASH"
temp="30"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Maximum != 35 {
		t.Fatalf("Maximum = %d, expected 35 (cache SSD at 55 degrees must be excluded)", result.Maximum)
	}
	if result.Reporting != 1 || result.Parsed != 1 {
		t.Fatalf("Reporting/Parsed = %d/%d, expected 1/1", result.Reporting, result.Parsed)
	}
	if len(result.Disks) != 1 || result.Disks[0].Name != "disk1" {
		t.Fatalf("Disks should contain only disk1: %+v", result.Disks)
	}
}

// Regression: on a real system the flash boot stick had no
// type="FLASH" field, so it was still counted as an HDD despite the type
// filter. The section name "flash" (Unraid's own convention) alone must
// be enough.
func TestReadDiskTemperaturesExcludesFlashByNameWithoutTypeField(t *testing.T) {
	path := write(t, "disks.ini", `["disk1"]
name="disk1"
type="DATA"
temp="35"
["flash"]
name="flash"
temp="42"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Maximum != 35 {
		t.Fatalf("Maximum = %d, expected 35 (flash stick without type= must still be excluded)", result.Maximum)
	}
	if len(result.Disks) != 1 || result.Disks[0].Name != "disk1" {
		t.Fatalf("Disks should contain only disk1: %+v", result.Disks)
	}
}

// Regression: disks.ini lists sections in emhttp's write order, not in
// an order that makes sense to the user.
func TestReadDiskTemperaturesSortsNaturally(t *testing.T) {
	path := write(t, "disks.ini", `["parity2"]
name="parity2"
type="PARITY"
temp="33"
["disk10"]
name="disk10"
type="DATA"
temp="30"
["disk2"]
name="disk2"
type="DATA"
temp="31"
["parity"]
name="parity"
type="PARITY"
temp="32"
["disk1"]
name="disk1"
type="DATA"
temp="34"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var names []string
	for _, disk := range result.Disks {
		names = append(names, disk.Name)
	}
	expected := []string{"disk1", "disk2", "disk10", "parity", "parity2"}
	if len(names) != len(expected) {
		t.Fatalf("names = %v, expected %v", names, expected)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("names = %v, expected %v", names, expected)
		}
	}
}

// If the type= field is missing entirely (older Unraid versions), the
// drive must still be counted as before, instead of disappearing due to
// the new filter.
func TestReadDiskTemperaturesCountsDisksWithoutTypeField(t *testing.T) {
	path := write(t, "disks.ini", `["disk1"]
name="disk1"
temp="35"
`)

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reporting != 1 || result.Maximum != 35 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// Regression: a missing or unusable disks.ini used to return 0 degrees
// and thus the lowest curve step.
func TestReadDiskTemperaturesReportsProblems(t *testing.T) {
	if _, err := ReadDiskTemperatures(filepath.Join(t.TempDir(), "missing.ini")); err == nil {
		t.Error("missing file must return an error")
	}
	path := write(t, "disks.ini", "name=\"disk1\"\nsize=\"100\"\n")
	if _, err := ReadDiskTemperatures(path); err == nil {
		t.Error("file without temp= must return an error")
	}
	if _, err := ReadDiskTemperatures(write(t, "disks.ini", "")); err == nil {
		t.Error("empty file must return an error")
	}
}

func TestArrayOperation(t *testing.T) {
	cases := []struct {
		content  string
		expected string
	}{
		{"mdResync=\"0\"\n", OperationNone},
		{"mdResync=\"\"\n", OperationNone},
		{"mdResync=\"1000\"\nmdResyncAction=\"check P\"\n", "parity-check"},
		{"mdResync=\"1000\"\nmdResyncAction=\"recon P\"\n", "rebuild"},
		{"mdResync=\"1000\"\nmdResyncAction=\"reconstruct P\"\n", "rebuild"},
		{"mdResync=\"1000\"\nmdResyncAction=\"resync\"\n", "parity-sync"},
		{"mdResync=\"1000\"\nmdResyncAction=\"clear\"\n", "clear"},
		{"mdResync=\"1000\"\n", "array-operation"},
		{"# comment\n[section]\nmdResync=\"1000\"\nmdResyncAction=\"weird\"\n", "weird"},
	}

	for _, testCase := range cases {
		path := write(t, "var.ini", testCase.content)
		operation, err := ArrayOperation(path)
		if err != nil {
			t.Errorf("%q: unexpected error %v", testCase.content, err)
			continue
		}
		if operation != testCase.expected {
			t.Errorf("%q: Operation = %q, expected %q", testCase.content, operation, testCase.expected)
		}
	}
}

func TestArrayOperationUnknownOnMissingFile(t *testing.T) {
	operation, err := ArrayOperation(filepath.Join(t.TempDir(), "missing.ini"))
	if err == nil {
		t.Error("missing file must return an error")
	}
	if operation != OperationUnknown {
		t.Errorf("Operation = %q, expected %q", operation, OperationUnknown)
	}
}

func TestSanitizeOperation(t *testing.T) {
	if got := sanitizeOperation("weird stuff!!"); got != "weird-stuff" {
		t.Errorf("sanitizeOperation = %q", got)
	}
	if got := sanitizeOperation("!!!"); got != "array-operation" {
		t.Errorf("sanitizeOperation = %q", got)
	}
	long := sanitizeOperation("abcdefghijklmnopqrstuvwxyz0123456789abcdefghij")
	if len(long) > 32 {
		t.Errorf("output too long: %d", len(long))
	}
}
