package incident

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/model"
)

const (
	SnapshotSchemaVersion = "codexremote.incident.snapshot.v1"
	MaxSnapshotEvents     = 500
	defaultBeforeSeconds  = 300
	defaultAfterSeconds   = 120
	maxWindowSeconds      = 3600
)

var ErrNotFound = errors.New("incident not found")

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

type CreateInput struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Severity      string `json:"severity"`
	AnchorEventID int64  `json:"anchor_event_id"`
	BeforeSeconds int    `json:"before_seconds"`
	AfterSeconds  int    `json:"after_seconds"`
}

type SnapshotSpec struct {
	Title         string
	Summary       string
	Severity      string
	AnchorEventID int64
	WindowStart   time.Time
	WindowEnd     time.Time
	SessionID     string
	TraceID       string
	TurnRef       string
	MaxEvents     int
}

type Summary struct {
	ID            int64  `json:"id"`
	CreatedAt     string `json:"created_at"`
	Title         string `json:"title"`
	Summary       string `json:"summary,omitempty"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	AnchorEventID int64  `json:"anchor_event_id,omitempty"`
	WindowStart   string `json:"window_start"`
	WindowEnd     string `json:"window_end"`
	SessionID     string `json:"session_id,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`
	TurnRef       string `json:"turn_ref,omitempty"`
	EventCount    int    `json:"event_count"`
	ArtifactCount int    `json:"artifact_count"`
	Truncated     bool   `json:"truncated"`
}

type ArtifactManifest struct {
	ID        int64  `json:"id,omitempty"`
	CreatedAt string `json:"created_at"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type Provenance struct {
	DataSource       string         `json:"data_source"`
	DatabaseTimezone string         `json:"database_timezone"`
	WindowStart      string         `json:"window_start"`
	WindowEnd        string         `json:"window_end"`
	MaxEvents        int            `json:"max_events"`
	Truncated        bool           `json:"truncated"`
	SourceCounts     map[string]int `json:"source_counts"`
	PrivacyPolicy    string         `json:"privacy_policy"`
}

type Snapshot struct {
	SchemaVersion string             `json:"schema_version"`
	CreatedAt     string             `json:"created_at"`
	Incident      Summary            `json:"incident"`
	Events        []model.Event      `json:"events"`
	Artifacts     []ArtifactManifest `json:"artifacts"`
	Provenance    Provenance         `json:"provenance"`
}

type Repository interface {
	Event(context.Context, int64) (model.Event, error)
	CreateIncidentSnapshot(context.Context, SnapshotSpec) (*Snapshot, error)
	ListIncidents(context.Context, int) ([]Summary, error)
	IncidentSnapshot(context.Context, int64) (*Snapshot, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Create(ctx context.Context, input CreateInput) (*Snapshot, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Severity = strings.ToLower(strings.TrimSpace(input.Severity))
	if input.AnchorEventID <= 0 {
		return nil, ValidationError{"anchor_event_id is required"}
	}
	if input.Title == "" || len([]rune(input.Title)) > 160 {
		return nil, ValidationError{"title must contain 1 to 160 characters"}
	}
	if len([]rune(input.Summary)) > 2000 {
		return nil, ValidationError{"summary must not exceed 2000 characters"}
	}
	if input.Severity == "" {
		input.Severity = "warning"
	}
	if input.Severity != "info" && input.Severity != "warning" && input.Severity != "critical" {
		return nil, ValidationError{"severity must be info, warning, or critical"}
	}
	before, err := boundedSeconds(input.BeforeSeconds, defaultBeforeSeconds)
	if err != nil {
		return nil, err
	}
	after, err := boundedSeconds(input.AfterSeconds, defaultAfterSeconds)
	if err != nil {
		return nil, err
	}
	anchor, err := s.repository.Event(ctx, input.AnchorEventID)
	if err != nil {
		return nil, err
	}
	anchorTime, err := time.Parse(time.RFC3339Nano, anchor.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("anchor timestamp: %w", err)
	}
	return s.repository.CreateIncidentSnapshot(ctx, SnapshotSpec{
		Title: input.Title, Summary: input.Summary, Severity: input.Severity, AnchorEventID: input.AnchorEventID,
		WindowStart: anchorTime.Add(-time.Duration(before) * time.Second), WindowEnd: anchorTime.Add(time.Duration(after) * time.Second),
		SessionID: anchor.SessionID, TraceID: anchor.TraceID, TurnRef: anchor.TurnRef, MaxEvents: MaxSnapshotEvents,
	})
}

func (s *Service) List(ctx context.Context, limit int) ([]Summary, error) {
	return s.repository.ListIncidents(ctx, limit)
}

func (s *Service) Snapshot(ctx context.Context, id int64) (*Snapshot, error) {
	if id <= 0 {
		return nil, ValidationError{"invalid incident id"}
	}
	return s.repository.IncidentSnapshot(ctx, id)
}

func boundedSeconds(value, fallback int) (int, error) {
	if value == 0 {
		return fallback, nil
	}
	if value < 1 || value > maxWindowSeconds {
		return 0, ValidationError{"incident window must be between 1 and 3600 seconds"}
	}
	return value, nil
}
