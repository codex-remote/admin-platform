package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/model"
)

func TestParseEventQueryDefaultsToBoundedWindow(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	request := httptest.NewRequest("GET", "/api/v1/diagnostics/events?source=iphone-app,relay-server&level=WARNING&limit=50", nil)
	query, err := parseEventQuery(request, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := query.To.Sub(query.From); got != 30*time.Minute {
		t.Fatalf("expected 30 minute window, got %s", got)
	}
	if query.Limit != 50 || len(query.Sources) != 2 || query.Levels[0] != "warning" {
		t.Fatalf("unexpected query: %#v", query)
	}
}

func TestParseEventQueryRejectsUnboundedRange(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/diagnostics/events?from=2026-08-01T00:00:00Z&to=2026-08-13T00:00:00Z", nil)
	if _, err := parseEventQuery(request, time.Now()); err == nil {
		t.Fatal("expected a range error")
	}
}

func TestEventCursorRoundTrip(t *testing.T) {
	event := model.Event{ID: 42, Timestamp: "2026-08-13T08:01:02.123456Z"}
	timestamp, id, err := decodeEventCursor(encodeEventCursor(event))
	if err != nil {
		t.Fatal(err)
	}
	if id != event.ID || timestamp.Format(time.RFC3339Nano) != event.Timestamp {
		t.Fatalf("cursor mismatch: %d %s", id, timestamp)
	}
}

func TestHistogramIntervalKeepsBucketCountBounded(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		30 * time.Minute: time.Minute,
		6 * time.Hour:    5 * time.Minute,
		12 * time.Hour:   15 * time.Minute,
		24 * time.Hour:   30 * time.Minute,
	}
	for window, expected := range cases {
		if got := histogramInterval(window); got != expected {
			t.Fatalf("window %s: expected %s, got %s", window, expected, got)
		}
	}
}
