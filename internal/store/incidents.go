package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/incident"
	"github.com/ai-coding-remote/admin-platform/internal/model"
)

const privacyPolicy = "allowlisted metadata only; prompts, responses, credentials, relay URLs, and full file paths are prohibited"

func (s *Store) CreateIncidentSnapshot(ctx context.Context, spec incident.SnapshotSpec) (*incident.Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	events, truncated, err := snapshotEvents(ctx, tx, spec)
	if err != nil {
		return nil, err
	}
	artifacts, err := snapshotArtifacts(ctx, tx, events)
	if err != nil {
		return nil, err
	}
	createdAt := time.Now().UTC()
	metadata, _ := json.Marshal(map[string]any{
		"data_source": "postgresql/local", "database_timezone": "UTC", "max_events": spec.MaxEvents,
		"source_counts": sourceCounts(events), "privacy_policy": privacyPolicy,
	})
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO incidents
(created_at,title,summary,severity,status,anchor_event_id,window_start,window_end,session_id,trace_id,turn_ref,event_count,artifact_count,truncated,snapshot_version,snapshot_metadata)
VALUES($1,$2,$3,$4,'open',$5,$6,$7,$8,$9,$10,$11,$12,$13,1,$14::jsonb) RETURNING id`,
		createdAt, spec.Title, spec.Summary, spec.Severity, spec.AnchorEventID, spec.WindowStart.UTC(), spec.WindowEnd.UTC(),
		spec.SessionID, spec.TraceID, spec.TurnRef, len(events), len(artifacts), truncated, string(metadata)).Scan(&id)
	if err != nil {
		return nil, err
	}
	for index, event := range events {
		snapshot, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO incident_events(incident_id,ordinal,source_event_id,snapshot_json) VALUES($1,$2,$3,$4::jsonb)`,
			id, index, event.ID, string(snapshot)); err != nil {
			return nil, err
		}
	}
	for index, artifact := range artifacts {
		manifest, err := json.Marshal(artifact)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO incident_artifacts(incident_id,ordinal,artifact_id,manifest_json) VALUES($1,$2,$3,$4::jsonb)`,
			id, index, nullableArtifactID(artifact.ID), string(manifest)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	summary := incident.Summary{ID: id, CreatedAt: createdAt.Format(time.RFC3339Nano), Title: spec.Title, Summary: spec.Summary,
		Severity: spec.Severity, Status: "open", AnchorEventID: spec.AnchorEventID,
		WindowStart: spec.WindowStart.UTC().Format(time.RFC3339Nano), WindowEnd: spec.WindowEnd.UTC().Format(time.RFC3339Nano),
		SessionID: spec.SessionID, TraceID: spec.TraceID, TurnRef: spec.TurnRef,
		EventCount: len(events), ArtifactCount: len(artifacts), Truncated: truncated}
	return buildSnapshot(createdAt, summary, events, artifacts, spec.MaxEvents, sourceCounts(events)), nil
}

func snapshotEvents(ctx context.Context, tx *sql.Tx, spec incident.SnapshotSpec) ([]model.Event, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,timestamp,received_at,source,profile,level,category,event,message,
session_id,trace_id,turn_ref,sequence,fields_json,artifact_id FROM events
WHERE timestamp BETWEEN $1 AND $2 ORDER BY timestamp,id LIMIT $3`, spec.WindowStart.UTC(), spec.WindowEnd.UTC(), spec.MaxEvents+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	events := make([]model.Event, 0, spec.MaxEvents+1)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, false, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(events) > spec.MaxEvents
	if truncated {
		events = events[:spec.MaxEvents]
	}
	return events, truncated, nil
}

func snapshotArtifacts(ctx context.Context, tx *sql.Tx, events []model.Event) ([]incident.ArtifactManifest, error) {
	ids := make([]int64, 0)
	seen := map[int64]bool{}
	for _, event := range events {
		if event.ArtifactID != nil && !seen[*event.ArtifactID] {
			seen[*event.ArtifactID] = true
			ids = append(ids, *event.ArtifactID)
		}
	}
	if len(ids) == 0 {
		return []incident.ArtifactManifest{}, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,created_at,kind,source,name,size_bytes,sha256 FROM artifacts WHERE id=ANY($1::bigint[]) ORDER BY id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]incident.ArtifactManifest, 0, len(ids))
	for rows.Next() {
		var artifact incident.ArtifactManifest
		var createdAt time.Time
		if err := rows.Scan(&artifact.ID, &createdAt, &artifact.Kind, &artifact.Source, &artifact.Name, &artifact.SizeBytes, &artifact.SHA256); err != nil {
			return nil, err
		}
		artifact.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *Store) ListIncidents(ctx context.Context, limit int) ([]incident.Summary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, incidentSummaryQuery+` ORDER BY created_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]incident.Summary, 0)
	for rows.Next() {
		item, err := scanIncidentSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) IncidentSnapshot(ctx context.Context, id int64) (*incident.Snapshot, error) {
	var metadata []byte
	row := s.db.QueryRowContext(ctx, incidentSummaryQuery+` WHERE id=$1`, id)
	summary, err := scanIncidentSummary(row, &metadata)
	if err == sql.ErrNoRows {
		return nil, incident.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	events, err := s.incidentEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.incidentArtifacts(ctx, id)
	if err != nil {
		return nil, err
	}
	var stored struct {
		MaxEvents    int            `json:"max_events"`
		SourceCounts map[string]int `json:"source_counts"`
	}
	_ = json.Unmarshal(metadata, &stored)
	createdAt, _ := time.Parse(time.RFC3339Nano, summary.CreatedAt)
	return buildSnapshot(createdAt, summary, events, artifacts, stored.MaxEvents, stored.SourceCounts), nil
}

const incidentSummaryQuery = `SELECT id,created_at,title,summary,severity,status,anchor_event_id,window_start,window_end,
session_id,trace_id,turn_ref,event_count,artifact_count,truncated,snapshot_metadata FROM incidents`

func scanIncidentSummary(row rowScanner, metadataTarget ...*[]byte) (incident.Summary, error) {
	var item incident.Summary
	var createdAt, windowStart, windowEnd time.Time
	var anchor sql.NullInt64
	var metadata []byte
	err := row.Scan(&item.ID, &createdAt, &item.Title, &item.Summary, &item.Severity, &item.Status, &anchor,
		&windowStart, &windowEnd, &item.SessionID, &item.TraceID, &item.TurnRef, &item.EventCount, &item.ArtifactCount, &item.Truncated, &metadata)
	if err != nil {
		return incident.Summary{}, err
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	item.WindowStart = windowStart.UTC().Format(time.RFC3339Nano)
	item.WindowEnd = windowEnd.UTC().Format(time.RFC3339Nano)
	if anchor.Valid {
		item.AnchorEventID = anchor.Int64
	}
	if len(metadataTarget) > 0 {
		*metadataTarget[0] = metadata
	}
	return item, nil
}

func (s *Store) incidentEvents(ctx context.Context, id int64) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT snapshot_json FROM incident_events WHERE incident_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]model.Event, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var event model.Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("decode incident event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) incidentArtifacts(ctx context.Context, id int64) ([]incident.ArtifactManifest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT manifest_json FROM incident_artifacts WHERE incident_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]incident.ArtifactManifest, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var artifact incident.ArtifactManifest
		if err := json.Unmarshal(raw, &artifact); err != nil {
			return nil, fmt.Errorf("decode incident artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func buildSnapshot(createdAt time.Time, summary incident.Summary, events []model.Event, artifacts []incident.ArtifactManifest, maxEvents int, counts map[string]int) *incident.Snapshot {
	return &incident.Snapshot{SchemaVersion: incident.SnapshotSchemaVersion, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		Incident: summary, Events: events, Artifacts: artifacts,
		Provenance: incident.Provenance{DataSource: "postgresql/local", DatabaseTimezone: "UTC",
			WindowStart: summary.WindowStart, WindowEnd: summary.WindowEnd, MaxEvents: maxEvents, Truncated: summary.Truncated,
			SourceCounts: counts, PrivacyPolicy: privacyPolicy}}
}

func sourceCounts(events []model.Event) map[string]int {
	counts := map[string]int{}
	for _, event := range events {
		counts[event.Source]++
	}
	return counts
}

func nullableArtifactID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
