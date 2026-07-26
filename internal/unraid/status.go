package unraid

import (
	"bufio"
	"fmt"
	"os"
	"sort"
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

// Disk describes one array member's live state, one disks.ini section.
type Disk struct {
	Name        string
	Type        string // e.g. DATA, PARITY, CACHE, FLASH; empty if the field is absent
	Temperature int
	Valid       bool // false if spun down ("*") or unparseable
}

// IsHDD reports whether this disk should count toward the HDD temperature
// aggregate and appear in the per-disk list. Cache pools and the boot flash
// drive are typically SSD/NVMe and are excluded; everything else (DATA,
// PARITY, and sections with no type= field at all, e.g. older Unraid
// versions) counts, which is the safe default when the field is missing.
//
// The type= field alone turned out not to be reliable for the flash boot
// device (some Unraid versions leave it unset for that section), so the
// section name is also checked: Unraid's own naming convention always calls
// the boot drive "flash" and cache pool members "cache"/"cache2"/... .
func (d Disk) IsHDD() bool {
	name := strings.ToLower(strings.TrimSpace(d.Name))
	if name == "flash" || strings.HasPrefix(name, "cache") {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(d.Type)) {
	case "CACHE", "FLASH":
		return false
	default:
		return true
	}
}

// DiskTemperatures separates "no disk is warm" from "the file could not be
// understood". The old code returned 0 for both, which made a missing
// disks.ini look like an idle array and dropped the fans to the lowest step.
type DiskTemperatures struct {
	Maximum   int
	Reporting int
	Parsed    int
	// Disks holds every HDD section (see Disk.IsHDD), in file order. Cache
	// and flash devices are already filtered out here.
	Disks []Disk
}

// ReadDiskTemperatures collects the temp= entries from an Unraid disks.ini.
// An error means the caller must treat the temperature as unknown and fall
// back to a safe fan speed.
//
// disks.ini groups fields into sections, one per device:
//
//	["disk1"]
//	name="disk1"
//	type="DATA"
//	temp="35"
//
// Sections are tracked so that name/type/temp are attributed to the right
// disk instead of overwriting each other in a flat map, and so cache/flash
// devices can be excluded from the HDD aggregate (Maximum/Reporting/Parsed)
// and from Disks. Any temp= line seen before the first section header (no
// real disks.ini does this, but a handful of tests exercise it) is treated
// as one anonymous, type-less disk, which defaults to counting as an HDD.
func ReadDiskTemperatures(path string) (DiskTemperatures, error) {
	var result DiskTemperatures

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()

	current := Disk{}
	sawTemp := false

	flush := func() {
		if !sawTemp {
			current = Disk{}
			return
		}
		if current.IsHDD() {
			result.Reporting++
			if current.Valid {
				result.Parsed++
				if current.Temperature > result.Maximum {
					result.Maximum = current.Temperature
				}
			}
			result.Disks = append(result.Disks, current)
		}
		current = Disk{}
		sawTemp = false
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			flush()
			current.Name = strings.Trim(strings.Trim(line, "[]"), "\"")
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")

		switch strings.TrimSpace(key) {
		case "name":
			if value != "" {
				current.Name = value
			}
		case "type":
			current.Type = value
		case "temp":
			sawTemp = true
			temperature, err := strconv.Atoi(value)
			// A parse failure means the disk is spun down (temp="*") or the
			// field is otherwise unusable; current.Valid simply stays false.
			if err == nil && temperature >= minTemperature && temperature <= maxTemperature {
				current.Temperature = temperature
				current.Valid = true
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return result, err
	}

	if result.Reporting == 0 {
		return result, fmt.Errorf("no temp= entries found in %s", path)
	}

	// disks.ini lists sections in whatever order emhttp last wrote them,
	// which is not necessarily a sensible display order. A natural sort
	// (name prefix, then trailing number) puts disk1..disk10 and
	// parity/parity2 in the order an operator expects instead of file order.
	sort.SliceStable(result.Disks, func(i, j int) bool {
		return diskNameLess(result.Disks[i].Name, result.Disks[j].Name)
	})
	return result, nil
}

func diskNameLess(a, b string) bool {
	aPrefix, aNum, aHasNum := splitTrailingNumber(a)
	bPrefix, bNum, bHasNum := splitTrailingNumber(b)
	if aPrefix != bPrefix {
		return aPrefix < bPrefix
	}
	if aHasNum != bHasNum {
		// "parity" before "parity2": the unnumbered entry is the first one.
		return bHasNum
	}
	return aNum < bNum
}

func splitTrailingNumber(s string) (prefix string, number int, hasNumber bool) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return s, 0, false
	}
	n, err := strconv.Atoi(s[i:])
	if err != nil {
		return s, 0, false
	}
	return s[:i], n, true
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
