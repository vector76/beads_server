package model

import (
	"encoding/json"
	"testing"
)

func TestStatusNotReadyValid(t *testing.T) {
	if !StatusNotReady.Valid() {
		t.Error("StatusNotReady.Valid() should return true")
	}
}

func TestStatusNotReadyStringValid(t *testing.T) {
	if !Status("not_ready").Valid() {
		t.Error(`Status("not_ready").Valid() should return true`)
	}
}

func TestStatusNotReadyJSONRoundTrip(t *testing.T) {
	data, err := json.Marshal(StatusNotReady)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != `"not_ready"` {
		t.Errorf("expected %q, got %s", "not_ready", data)
	}

	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if s != StatusNotReady {
		t.Errorf("expected %q, got %q", StatusNotReady, s)
	}
}
