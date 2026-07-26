package app

import (
	"testing"
	"time"
)

func TestShouldWriteFanOnValueChange(t *testing.T) {
	if !shouldWriteFan(true, 0, writeFailureRetryInterval, true, false) {
		t.Fatal("eine geänderte Zielvorgabe muss immer geschrieben werden")
	}
}

func TestShouldWriteFanOnRediscovery(t *testing.T) {
	if !shouldWriteFan(false, 0, 5*time.Minute, true, true) {
		t.Fatal("nach einer Controller-Neuerkennung muss sofort geschrieben werden")
	}
}

func TestShouldWriteFanReapplyInterval(t *testing.T) {
	if shouldWriteFan(false, 4*time.Minute, 5*time.Minute, true, false) {
		t.Fatal("vor Ablauf des Reapply-Intervalls darf nicht erneut geschrieben werden")
	}
	if !shouldWriteFan(false, 5*time.Minute, 5*time.Minute, true, false) {
		t.Fatal("nach Ablauf des Reapply-Intervalls muss erneut geschrieben werden")
	}
}

func TestShouldWriteFanFastRetryAfterFailure(t *testing.T) {
	// 30s liegt weit unter dem normalen 5-Minuten-Intervall, aber über der
	// 10-Sekunden-Retry-Frist nach einem fehlgeschlagenen Schreibversuch.
	if shouldWriteFan(false, 30*time.Second, 5*time.Minute, true, false) {
		t.Fatal("ohne vorherigen Fehler darf vor Ablauf des Intervalls nicht geschrieben werden")
	}
	if !shouldWriteFan(false, 30*time.Second, 5*time.Minute, false, false) {
		t.Fatal("nach einem fehlgeschlagenen Schreibversuch muss nach spätestens 10s erneut versucht werden")
	}
}
