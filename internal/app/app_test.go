package app

import (
	"testing"
	"time"
)

func TestShouldWriteFanOnValueChange(t *testing.T) {
	if !shouldWriteFan(true, 0, writeFailureRetryInterval, true, false) {
		t.Fatal("a changed target must always be written")
	}
}

func TestShouldWriteFanOnRediscovery(t *testing.T) {
	if !shouldWriteFan(false, 0, 5*time.Minute, true, true) {
		t.Fatal("must write immediately after a controller rediscovery")
	}
}

func TestShouldWriteFanReapplyInterval(t *testing.T) {
	if shouldWriteFan(false, 4*time.Minute, 5*time.Minute, true, false) {
		t.Fatal("must not write again before the reapply interval elapses")
	}
	if !shouldWriteFan(false, 5*time.Minute, 5*time.Minute, true, false) {
		t.Fatal("must write again after the reapply interval elapses")
	}
}

func TestShouldWriteFanFastRetryAfterFailure(t *testing.T) {
	// 30s is well under the normal 5-minute interval, but above the
	// 10-second retry window after a failed write attempt.
	if shouldWriteFan(false, 30*time.Second, 5*time.Minute, true, false) {
		t.Fatal("without a prior failure, must not write before the interval elapses")
	}
	if !shouldWriteFan(false, 30*time.Second, 5*time.Minute, false, false) {
		t.Fatal("after a failed write attempt, must retry within 10s at the latest")
	}
}
