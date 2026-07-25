package store

import (
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
		t.Fatalf("unerwarteter Inhalt: %+v", loaded)
	}

	if err := st.Remove("config.json"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := st.Remove("config.json"); err != nil {
		t.Fatalf("zweites Remove darf keinen Fehler liefern: %v", err)
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
		t.Fatalf("erwarte 3 Ereignisse, habe %d", len(events))
	}
	if events[0].Message != "7" || events[2].Message != "9" {
		t.Fatalf("falsche Reihenfolge: %q, %q", events[0].Message, events[2].Message)
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
		t.Fatalf("erwarte leeren Verlauf, habe %d", len(history))
	}
}

// Regression: history.jsonl und events.jsonl wuchsen unbegrenzt.
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
		t.Fatalf("Rotation greift nicht, %d Zeilen vorhanden", len(all))
	}
	newest := all[len(all)-1]
	if newest.Temperature != 69 {
		t.Fatalf("neuester Punkt fehlt, Temperatur = %d", newest.Temperature)
	}
}

