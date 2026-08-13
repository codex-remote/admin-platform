package incident

import (
	"context"
	"testing"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/model"
)

type fakeRepository struct {
	anchor model.Event
	spec   SnapshotSpec
}

func (f *fakeRepository) Event(context.Context, int64) (model.Event, error) { return f.anchor, nil }
func (f *fakeRepository) CreateIncidentSnapshot(_ context.Context, spec SnapshotSpec) (*Snapshot, error) {
	f.spec = spec
	return &Snapshot{Incident: Summary{Title: spec.Title}}, nil
}
func (f *fakeRepository) ListIncidents(context.Context, int) ([]Summary, error)      { return nil, nil }
func (f *fakeRepository) IncidentSnapshot(context.Context, int64) (*Snapshot, error) { return nil, nil }

func TestCreateBuildsBoundedSnapshotSpec(t *testing.T) {
	anchorTime := time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC)
	repository := &fakeRepository{anchor: model.Event{Timestamp: anchorTime.Format(time.RFC3339Nano), SessionID: "session", TraceID: "trace"}}
	service := NewService(repository)
	_, err := service.Create(context.Background(), CreateInput{Title: " Memory incident ", AnchorEventID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if repository.spec.Title != "Memory incident" || repository.spec.MaxEvents != MaxSnapshotEvents {
		t.Fatalf("unexpected spec: %+v", repository.spec)
	}
	if !repository.spec.WindowStart.Equal(anchorTime.Add(-5*time.Minute)) || !repository.spec.WindowEnd.Equal(anchorTime.Add(2*time.Minute)) {
		t.Fatalf("unexpected window: %s to %s", repository.spec.WindowStart, repository.spec.WindowEnd)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})
	for _, input := range []CreateInput{{}, {Title: "x", AnchorEventID: 1, Severity: "unknown"}, {Title: "x", AnchorEventID: 1, BeforeSeconds: 3601}} {
		if _, err := service.Create(context.Background(), input); err == nil {
			t.Fatalf("expected validation error for %+v", input)
		}
	}
}
