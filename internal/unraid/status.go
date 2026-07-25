package unraid

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func MaxDiskTemperature(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	maximum := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "temp=") {
			continue
		}
		value := strings.Trim(strings.TrimPrefix(line, "temp="), "\"")
		temp, err := strconv.Atoi(value)
		if err == nil && temp > maximum {
			maximum = temp
		}
	}
	return maximum
}

func ArrayOperation(path string) string {
	values := readINI(path)
	resync, _ := strconv.ParseInt(values["mdResync"], 10, 64)
	if resync <= 0 {
		return "none"
	}

	action := strings.ToLower(values["mdResyncAction"])
	switch {
	case strings.HasPrefix(action, "check"):
		return "parity-check"
	case strings.HasPrefix(action, "reconstruct"):
		return "rebuild"
	case strings.HasPrefix(action, "resync"):
		return "parity-sync"
	case strings.HasPrefix(action, "clear"):
		return "clear"
	case action != "":
		return action
	default:
		return "array-operation"
	}
}

func readINI(path string) map[string]string {
	result := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		result[key] = strings.Trim(value, "\"")
	}
	return result
}
