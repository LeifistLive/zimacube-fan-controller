package unraid

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Plausible range for disk temperatures in degrees Celsius.
const (
	minTemperature = 1
	maxTemperature = 120
)

// Operation values returned by ArrayOperation.
const (
	OperationNone    = "none"
	OperationUnknown = "unknown"
)

// DiskTemperatures separates "no disk is warm" from "the file could not be
// understood". The old code returned 0 for both, which made a missing
// disks.ini look like an idle array and dropped the fans to the lowest step.
type DiskTemperatures struct {
	Maximum   int
	Reporting int
	Parsed    int
}

// ReadDiskTemperatures collects the temp= entries from an Unraid disks.ini.
// An error means the caller must treat the temperature as unknown and fall
// back to a safe fan speed.
func ReadDiskTemperatures(path string) (DiskTemperatures, error) {
	var result DiskTemperatures

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "temp=") {
			continue
		}
		result.Reporting++

		raw := strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "temp="), "\""))
		temperature, err := strconv.Atoi(raw)
		if err != nil {
			// Spun down disks report temp="*".
			continue
		}
		if temperature < minTemperature || temperature > maxTemperature {
			continue
		}

		result.Parsed++
		if temperature > result.Maximum {
			result.Maximum = temperature
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}

	if result.Reporting == 0 {
		return result, fmt.Errorf("keine temp=-Einträge in %s gefunden", path)
	}
	return result, nil
}

// ArrayOperation reports a running parity check, rebuild, resync or clear.
// A read error yields OperationUnknown, which callers must not treat as a
// running operation (that would boost the fans forever) nor as a guaranteed
// idle array.
func ArrayOperation(path string) (string, error) {
	values, err := readINI(path)
	if err != nil {
		return OperationUnknown, err
	}

	resync, _ := strconv.ParseInt(strings.TrimSpace(values["mdResync"]), 10, 64)
	if resync <= 0 {
		return OperationNone, nil
	}

	action := strings.ToLower(strings.TrimSpace(values["mdResyncAction"]))
	switch {
	case strings.HasPrefix(action, "check"):
		return "parity-check", nil
	case strings.HasPrefix(action, "recon"):
		return "rebuild", nil
	case strings.HasPrefix(action, "resync"):
		return "parity-sync", nil
	case strings.HasPrefix(action, "clear"):
		return "clear", nil
	case action != "":
		return sanitizeOperation(action), nil
	default:
		return "array-operation", nil
	}
}

// sanitizeOperation keeps unknown values from var.ini short and printable.
// They end up in the event log and in the web UI.
func sanitizeOperation(action string) string {
	var builder strings.Builder
	for _, r := range action {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune('-')
		}
		if builder.Len() >= 32 {
			break
		}
	}
	if builder.Len() == 0 {
		return "array-operation"
	}
	return builder.String()
}

func readINI(path string) (map[string]string, error) {
	result := map[string]string{}

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, nil
}
