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
		t.Fatalf("Datei %s: %v", name, err)
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
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if result.Maximum != 41 {
		t.Errorf("Maximum = %d, erwartet 41", result.Maximum)
	}
	if result.Reporting != 3 {
		t.Errorf("Reporting = %d, erwartet 3", result.Reporting)
	}
	if result.Parsed != 2 {
		t.Errorf("Parsed = %d, erwartet 2", result.Parsed)
	}
}

func TestReadDiskTemperaturesAllSpunDown(t *testing.T) {
	path := write(t, "disks.ini", "temp=\"*\"\ntemp=\"*\"\n")

	result, err := ReadDiskTemperatures(path)
	if err != nil {
		t.Fatalf("Standby ist kein Fehler: %v", err)
	}
	if result.Maximum != 0 || result.Parsed != 0 || result.Reporting != 2 {
		t.Fatalf("unerwartetes Ergebnis: %+v", result)
	}
}

// Regression: eine fehlende oder unbrauchbare disks.ini lieferte früher 0 Grad
// und damit die niedrigste Kurvenstufe.
func TestReadDiskTemperaturesReportsProblems(t *testing.T) {
	if _, err := ReadDiskTemperatures(filepath.Join(t.TempDir(), "fehlt.ini")); err == nil {
		t.Error("fehlende Datei muss einen Fehler liefern")
	}
	path := write(t, "disks.ini", "name=\"disk1\"\nsize=\"100\"\n")
	if _, err := ReadDiskTemperatures(path); err == nil {
		t.Error("Datei ohne temp= muss einen Fehler liefern")
	}
	if _, err := ReadDiskTemperatures(write(t, "disks.ini", "")); err == nil {
		t.Error("leere Datei muss einen Fehler liefern")
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
		{"# Kommentar\n[section]\nmdResync=\"1000\"\nmdResyncAction=\"seltsam\"\n", "seltsam"},
	}

	for _, testCase := range cases {
		path := write(t, "var.ini", testCase.content)
		operation, err := ArrayOperation(path)
		if err != nil {
			t.Errorf("%q: unerwarteter Fehler %v", testCase.content, err)
			continue
		}
		if operation != testCase.expected {
			t.Errorf("%q: Operation = %q, erwartet %q", testCase.content, operation, testCase.expected)
		}
	}
}

func TestArrayOperationUnknownOnMissingFile(t *testing.T) {
	operation, err := ArrayOperation(filepath.Join(t.TempDir(), "fehlt.ini"))
	if err == nil {
		t.Error("fehlende Datei muss einen Fehler liefern")
	}
	if operation != OperationUnknown {
		t.Errorf("Operation = %q, erwartet %q", operation, OperationUnknown)
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
		t.Errorf("Ausgabe zu lang: %d", len(long))
	}
}
