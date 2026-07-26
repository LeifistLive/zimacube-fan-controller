package store

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestSaveAndLoadJSON(t *testing.T) {
	st := New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := st.SaveJSON("config.json", payload{Name: "test", Value: 7}); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	var loaded payload
	if err := st.LoadJSON("config.json", &loaded); err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	if loaded.Name != "test" || loaded.Value != 7 {
		t.Fatalf("unexpected content: %+v", loaded)
	}

	if err := st.Remove("config.json"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := st.Remove("config.json"); err != nil {
		t.Fatalf("second Remove must not return an error: %v", err)
	}
}

func TestLoadJSONRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	st := New(dir, 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	type payload struct {
		Name string `json:"name"`
	}

	raw := []byte(`{"name":"test","unknown":true}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var loaded payload
	if err := st.LoadJSON("config.json", &loaded); err == nil {
		t.Fatal("unknown field must be rejected")
	}
}

func TestLoadJSONRejectsTrailingData(t *testing.T) {
	dir := t.TempDir()
	st := New(dir, 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	type payload struct {
		Name string `json:"name"`
	}

	raw := []byte(`{"name":"test"}{"name":"second-object"}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var loaded payload
	if err := st.LoadJSON("config.json", &loaded); err == nil {
		t.Fatal("data after the first JSON object must be rejected")
	}
}

func TestReadEventsReturnsNewestInOrder(t *testing.T) {
	st := New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	base := time.Now()
	for i := 0; i < 10; i++ {
		event := Event{Time: base.Add(time.Duration(i) * time.Second), Type: "mode", Message: strconv.Itoa(i)}
		if err := st.AppendEvent(event); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	events, err := st.ReadEvents(3)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, have %d", len(events))
	}
	if events[0].Message != "7" || events[2].Message != "9" {
		t.Fatalf("wrong order: %q, %q", events[0].Message, events[2].Message)
	}
}

func TestReadMissingFileIsEmpty(t *testing.T) {
	st := New(t.TempDir(), 0)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	history, err := st.ReadHistory(10)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, have %d", len(history))
	}
}

// Regression: history.jsonl and events.jsonl grew without bound.
func TestAppendPrunesOldLines(t *testing.T) {
	st := New(t.TempDir(), 10)
	if err := st.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	base := time.Now()
	for i := 0; i < 40; i++ {
		point := HistoryPoint{Time: base.Add(time.Duration(i) * time.Minute), Temperature: 30 + i, FanPercent: 50}
		if err := st.AppendHistory(point); err != nil {
			t.Fatalf("AppendHistory: %v", err)
		}
	}

	all, err := st.ReadHistory(0)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if len(all) > 11 {
		t.Fatalf("rotation is not kicking in, %d lines present", len(all))
	}
	newest := all[len(all)-1]
	if newest.Temperature != 69 {
		t.Fatalf("newest point missing, temperature = %d", newest.Temperature)
	}
}
