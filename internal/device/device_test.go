package device

import "testing"

func TestReferenceIsStableAndDoesNotExposeIdentifier(t *testing.T) {
	identifier := "00008120-001E7CAC1AC3601E"
	first, second := Reference(identifier), Reference(identifier)
	if first != second {
		t.Fatal("device reference is not stable")
	}
	if len(first) != len("device_")+24 {
		t.Fatalf("unexpected reference: %s", first)
	}
	if first == identifier {
		t.Fatal("reference exposes raw identifier")
	}
}
