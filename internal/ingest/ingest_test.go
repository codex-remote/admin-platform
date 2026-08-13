package ingest

import (
	"testing"
)

func TestParseSlogLine(t *testing.T) {
	event, err := ParseSlogLine([]byte(`{"time":"2026-08-13T01:02:03Z","level":"WARN","msg":"relay disconnected","trace_id":"trace-1","attempt":2}`), "relay-server", "simulator", "/tmp/relay.log", 12)
	if err != nil {
		t.Fatal(err)
	}
	if event.Name != "relay-server.relay_disconnected" || event.Level != "warning" || event.TraceID != "trace-1" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if len(event.Fingerprint) != 64 {
		t.Fatalf("unexpected fingerprint: %q", event.Fingerprint)
	}
}

func TestParseIPhoneLine(t *testing.T) {
	event, err := ParseIPhoneLine([]byte(`{"schema_version":1,"timestamp":"2026-08-13T01:02:03Z","session_id":"session-1","sequence":4,"level":"notice","category":"memory","event":"memory.snapshot","trace_id":"trace-1","fields":{"available_mb":512}}`), "simulator", "/tmp/events.jsonl", 0)
	if err != nil {
		t.Fatal(err)
	}
	if event.Source != "iphone-app" || event.Profile != "simulator" || event.Name != "memory.snapshot" || event.Sequence != 4 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseIPhoneLineRejectsUncorrelatedRecord(t *testing.T) {
	if _, err := ParseIPhoneLine([]byte(`{"event":"memory.snapshot"}`), "simulator", "/tmp/events.jsonl", 0); err == nil {
		t.Fatal("expected missing session_id error")
	}
}
