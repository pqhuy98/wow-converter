package db

import (
	"errors"
	"testing"
)

func TestIsDBDRetryable(t *testing.T) {
	if !isDBDRetryable(errors.New("status code: 429")) {
		t.Fatal("expected 429 to be retryable")
	}
	if isDBDRetryable(errors.New("status code: 404")) {
		t.Fatal("expected 404 not retryable")
	}
}

func TestDBDRetryDelayMS(t *testing.T) {
	if got := dbdRetryDelayMS(1, errors.New("status code: 429")); got < 2000 {
		t.Fatalf("429 delay too short: %d", got)
	}
	if got := dbdRetryDelayMS(2, errors.New("timeout")); got < 1600 {
		t.Fatalf("timeout delay too short: %d", got)
	}
}
