package main

import "testing"

func TestValidateListenRejectsNonLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18880", "[::1]:18880", "localhost:18880"} {
		if err := validateListen(address); err != nil {
			t.Fatalf("loopback %s: %v", address, err)
		}
	}
	if err := validateListen("0.0.0.0:18880"); err == nil {
		t.Fatal("expected non-loopback rejection")
	}
}
