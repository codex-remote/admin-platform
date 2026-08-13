package server

import "testing"

func TestNonNilEncodesNilAsEmptySlice(t *testing.T) {
	if values := nonNil[string](nil); values == nil || len(values) != 0 {
		t.Fatalf("expected a non-nil empty slice, got %#v", values)
	}
}
