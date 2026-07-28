package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultMaxLines caps history.jsonl and events.jsonl. Without a cap the
	// files grew forever and every API read loaded the whole file into memory.
	DefaultMaxLines = 20000

	historyFile = "history.jsonl"
	eventsFile  = "events.jsonl"

	maxLineBytes = 1 << 20
)

type HistoryPoint struct {
	Time        time.Time `json:"time"`
	Temperature int       `json:"temperature"`
	FanPercent  int       `json:"fan_percent"`
	Mode        string    `json:"mode"`
	Operation   string    `json:"operation"`
}

type Event struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}

type Store struct {
	dir      string
	maxLines int
	mu       sync.Mutex
	counts   map[string]int
}

func New(dir string, maxLines int) *Store {
	if maxLines <= 0 {
		maxLines = DefaultMaxLines
	}
	return &Store{
		dir:      dir,
		maxLines: maxLines,
		counts:   map[string]int{},
	}
}

func (s *Store) Ensure() error {
	return os.MkdirAll(s.dir, 0o755)
}

// SaveJSON writes atomically and flushes to disk, so that a power loss cannot
// leave a truncated config.json behind.
func (s *Store) SaveJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name)
	tmp := path + ".tmp"

	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(s.dir)
}

// LoadJSON rejects unknown fields and trailing data after the JSON object, so
// a corrupted or hand-edited config.json/override.json fails loudly instead
// of silently dropping fields the caller expected to be set.
func (s *Store) LoadJSON(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: unexpected data after the JSON object", name)
	}
	return nil
}

// Remove deletes name and fsyncs the directory, so a power loss right after
// clearing e.g. override.json cannot leave the old file back on disk (the
// same durability guarantee SaveJSON already gives the write path). Also
// drops any cached line count for name, so a subsequent appendJSONLine
// recomputes it instead of carrying over a stale count from before the file
// was removed.
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.counts, name)
	err := os.Remove(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncDir(s.dir)
}

// ClearEvents deletes events.jsonl entirely; the next AppendEvent recreates
// it. Used by the dashboard's "clear events" action.
func (s *Store) ClearEvents() error {
	return s.Remove(eventsFile)
}

func (s *Store) AppendHistory(point HistoryPoint) error {
	return s.appendJSONLine(historyFile, point)
}

func (s *Store) AppendEvent(event Event) error {
	return s.appendJSONLine(eventsFile, event)
}

func (s *Store) ReadHistory(limit int) ([]HistoryPoint, error) {
	s.mu.Lock()
	lines, err := tailLines(filepath.Join(s.dir, historyFile), limit)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	result := make([]HistoryPoint, 0, len(lines))
	for _, line := range lines {
		var point HistoryPoint
		if err := json.Unmarshal(line, &point); err != nil {
			continue
		}
		result = append(result, point)
	}
	return result, nil
}

func (s *Store) ReadEvents(limit int) ([]Event, error) {
	s.mu.Lock()
	lines, err := tailLines(filepath.Join(s.dir, eventsFile), limit)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	result := make([]Event, 0, len(lines))
	for _, line := range lines {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		result = append(result, event)
	}
	return result, nil
}

func (s *Store) appendJSONLine(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	count, known := s.counts[name]
	if known {
		count++
	} else {
		count, err = countLines(path)
		if err != nil {
			// Rotation is best effort; the line itself is already written.
			// Logged rather than silently dropped so a permissions/IO problem
			// that will keep the file growing unbounded is visible.
			log.Printf("[WARN] %s: could not count lines, rotation paused: %v", name, err)
			return nil
		}
	}
	s.counts[name] = count

	if count > s.maxLines+s.maxLines/10 {
		kept, err := prune(path, s.maxLines)
		if err != nil {
			log.Printf("[WARN] %s: rotation failed, file will keep growing for now: %v", name, err)
		} else {
			s.counts[name] = kept
		}
	}
	return nil
}

// prune rewrites the file with only the newest lines and returns how many
// remain.
func prune(path string, keep int) (int, error) {
	lines, err := tailLines(path, keep)
	if err != nil {
		return 0, err
	}

	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.Write(line); err != nil {
			_ = file.Close()
			_ = os.Remove(tmp)
			return 0, err
		}
		if err := writer.WriteByte('\n'); err != nil {
			_ = file.Close()
			_ = os.Remove(tmp)
			return 0, err
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return len(lines), syncDir(filepath.Dir(path))
}

// tailLines returns at most limit lines from the end of the file using a ring
// buffer, so memory stays proportional to limit and not to the file size.
func tailLines(path string, limit int) ([][]byte, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	if limit <= 0 {
		var all [][]byte
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			all = append(all, append([]byte(nil), line...))
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return all, nil
	}

	ring := make([][]byte, limit)
	total := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		ring[total%limit] = append([]byte(nil), line...)
		total++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if total <= limit {
		return ring[:total], nil
	}
	start := total % limit
	out := make([][]byte, 0, limit)
	out = append(out, ring[start:]...)
	out = append(out, ring[:start]...)
	return out, nil
}

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	buffer := make([]byte, 64*1024)
	count := 0
	for {
		read, err := file.Read(buffer)
		count += bytes.Count(buffer[:read], []byte{'\n'})
		if errors.Is(err, io.EOF) {
			return count, nil
		}
		if err != nil {
			return count, err
		}
	}
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()
	return handle.Sync()
}
