package privacy

import (
	"encoding/json"
	"testing"

	"github.com/codex-remote/admin-platform/internal/model"
)

func TestSanitizeEventUsesAllowlistAndRedactsLocations(t *testing.T) {
	event := model.Event{Message: "failed at /Users/person/secret/project", Fields: json.RawMessage(`{
      "available_mb":128,"prompt":"secret","authorization":"Bearer token",
      "reason":"https://internal.example/path","nested":{"value":1}
    }`)}
	SanitizeEvent(&event)
	if event.Message != Redacted {
		t.Fatalf("message was not redacted: %q", event.Message)
	}
	var fields map[string]any
	if err := json.Unmarshal(event.Fields, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["available_mb"] != float64(128) || fields["reason"] != Redacted {
		t.Fatalf("unexpected sanitized fields: %#v", fields)
	}
}

func TestSanitizeEventRejectsInvalidFields(t *testing.T) {
	event := model.Event{Fields: json.RawMessage(`{`)}
	SanitizeEvent(&event)
	if string(event.Fields) != `{}` {
		t.Fatalf("expected empty fields, got %s", event.Fields)
	}
}
