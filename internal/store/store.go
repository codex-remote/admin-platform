package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/incident"
	"github.com/ai-coding-remote/admin-platform/internal/model"
	"github.com/ai-coding-remote/admin-platform/internal/privacy"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct{ db *sql.DB }

func Open(databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect PostgreSQL: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("initialize migration ledger: %w", err)
	}
	files, err := migrationFiles()
	if err != nil {
		return err
	}
	for _, name := range files {
		data, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		migrationVersion, err := strconv.ParseInt(strings.SplitN(filepath.Base(name), "_", 2)[0], 10, 64)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration filename %q: %w", name, err)
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, migrationVersion).Scan(&exists); err != nil {
			tx.Rollback()
			return err
		}
		if exists {
			if err := tx.Commit(); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %03d: %w", migrationVersion, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1) ON CONFLICT DO NOTHING`, migrationVersion); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func migrationFiles() ([]string, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".sql" {
			continue
		}
		prefix := strings.SplitN(name, "_", 2)[0]
		if _, err := strconv.ParseInt(prefix, 10, 64); err != nil {
			return nil, fmt.Errorf("migration filename %q must start with a number", name)
		}
		files = append(files, "migrations/"+name)
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) Health(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) RuntimeInfo(ctx context.Context) (string, string, error) {
	var user, timezone string
	err := s.db.QueryRowContext(ctx, `SELECT current_user, current_setting('TimeZone')`).Scan(&user, &timezone)
	return user, timezone, err
}

func (s *Store) InsertEvents(ctx context.Context, events []model.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO events
(timestamp,received_at,source,profile,level,category,event,message,session_id,trace_id,turn_ref,sequence,fields_json,artifact_id,fingerprint)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15) ON CONFLICT (fingerprint) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	inserted := 0
	for i := range events {
		events[i].Normalize()
		privacy.SanitizeEvent(&events[i])
		if !json.Valid(events[i].Fields) {
			events[i].Fields = json.RawMessage(`{}`)
		}
		timestamp, err := parseTimestamp(events[i].Timestamp)
		if err != nil {
			return 0, fmt.Errorf("event %d timestamp: %w", i, err)
		}
		receivedAt, err := parseTimestamp(events[i].ReceivedAt)
		if err != nil {
			return 0, fmt.Errorf("event %d received_at: %w", i, err)
		}
		result, err := stmt.ExecContext(ctx, timestamp, receivedAt, events[i].Source,
			events[i].Profile, strings.ToLower(events[i].Level), events[i].Category, events[i].Name,
			events[i].Message, events[i].SessionID, events[i].TraceID, events[i].TurnRef,
			events[i].Sequence, string(events[i].Fields), events[i].ArtifactID, events[i].Fingerprint)
		if err != nil {
			return 0, err
		}
		if n, _ := result.RowsAffected(); n > 0 {
			inserted++
		}
	}
	return inserted, tx.Commit()
}

func (s *Store) Event(ctx context.Context, id int64) (model.Event, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,timestamp,received_at,source,profile,level,category,event,message,
session_id,trace_id,turn_ref,sequence,fields_json,artifact_id FROM events WHERE id=$1`, id)
	event, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return model.Event{}, incident.ErrNotFound
	}
	return event, err
}

func scanEvent(row rowScanner) (model.Event, error) {
	var event model.Event
	var timestamp, receivedAt time.Time
	var fields []byte
	var artifact sql.NullInt64
	err := row.Scan(&event.ID, &timestamp, &receivedAt, &event.Source, &event.Profile, &event.Level, &event.Category, &event.Name, &event.Message,
		&event.SessionID, &event.TraceID, &event.TurnRef, &event.Sequence, &fields, &artifact)
	if err != nil {
		return model.Event{}, err
	}
	event.Timestamp = timestamp.UTC().Format(time.RFC3339Nano)
	event.ReceivedAt = receivedAt.UTC().Format(time.RFC3339Nano)
	event.Fields = json.RawMessage(fields)
	if artifact.Valid {
		event.ArtifactID = &artifact.Int64
	}
	return event, nil
}

func (s *Store) Overview(ctx context.Context) (map[string]any, error) {
	result := map[string]any{}
	var total, warnings, errorsCount int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
COUNT(*) FILTER (WHERE level='warning'), COUNT(*) FILTER (WHERE level IN ('error','fault')) FROM events`).Scan(&total, &warnings, &errorsCount)
	if err != nil {
		return nil, err
	}
	result["total_events"], result["warnings"], result["errors"] = total, warnings, errorsCount
	rows, err := s.db.QueryContext(ctx, `SELECT source,COUNT(*) FROM events GROUP BY source ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySource := map[string]int64{}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		bySource[key] = count
	}
	result["by_source"] = bySource
	var artifacts, captures int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifacts").Scan(&artifacts); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM captures WHERE status IN ('queued','running')").Scan(&captures); err != nil {
		return nil, err
	}
	result["artifacts"], result["active_captures"] = artifacts, captures
	return result, rows.Err()
}

func (s *Store) CreateArtifact(ctx context.Context, artifact *model.Artifact) error {
	if artifact.CreatedAt == "" {
		artifact.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	createdAt, err := parseTimestamp(artifact.CreatedAt)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(artifact.Metadata)) {
		artifact.Metadata = `{}`
	}
	return s.db.QueryRowContext(ctx, `INSERT INTO artifacts(created_at,kind,source,name,path,size_bytes,sha256,metadata_json)
VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb) RETURNING id`, createdAt, artifact.Kind, artifact.Source, artifact.Name,
		artifact.Path, artifact.SizeBytes, artifact.SHA256, artifact.Metadata).Scan(&artifact.ID)
}

func (s *Store) ListArtifacts(ctx context.Context, limit int) ([]model.Artifact, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,kind,source,name,path,size_bytes,sha256,metadata_json FROM artifacts ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Artifact
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, artifact)
	}
	return out, rows.Err()
}

func (s *Store) Artifact(ctx context.Context, id int64) (model.Artifact, error) {
	return scanArtifact(s.db.QueryRowContext(ctx, `SELECT id,created_at,kind,source,name,path,size_bytes,sha256,metadata_json FROM artifacts WHERE id=$1`, id))
}

type rowScanner interface{ Scan(...any) error }

func scanArtifact(row rowScanner) (model.Artifact, error) {
	var artifact model.Artifact
	var createdAt time.Time
	var metadata []byte
	err := row.Scan(&artifact.ID, &createdAt, &artifact.Kind, &artifact.Source, &artifact.Name, &artifact.Path, &artifact.SizeBytes, &artifact.SHA256, &metadata)
	artifact.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	artifact.Metadata = string(metadata)
	return artifact, err
}

func (s *Store) CreateCapture(ctx context.Context, capture *model.Capture) error {
	if capture.CreatedAt == "" {
		capture.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	createdAt, err := parseTimestamp(capture.CreatedAt)
	if err != nil {
		return err
	}
	return s.db.QueryRowContext(ctx, `INSERT INTO captures(created_at,device_id,device_name,status,full_logs,output_path)
VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, createdAt, capture.DeviceID, capture.DeviceName, capture.Status, capture.FullLogs, capture.OutputPath).Scan(&capture.ID)
}

func (s *Store) UpdateCapture(ctx context.Context, capture model.Capture) error {
	_, err := s.db.ExecContext(ctx, `UPDATE captures SET started_at=COALESCE($1,started_at),finished_at=COALESCE($2,finished_at),status=$3,error=$4,artifact_id=$5 WHERE id=$6`,
		nullTimestamp(capture.StartedAt), nullTimestamp(capture.FinishedAt), capture.Status, capture.Error, capture.ArtifactID, capture.ID)
	return err
}

func (s *Store) ListCaptures(ctx context.Context) ([]model.Capture, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,started_at,finished_at,device_id,device_name,status,full_logs,output_path,error,artifact_id FROM captures ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Capture
	for rows.Next() {
		var capture model.Capture
		var createdAt time.Time
		var startedAt, finishedAt sql.NullTime
		var artifact sql.NullInt64
		if err := rows.Scan(&capture.ID, &createdAt, &startedAt, &finishedAt, &capture.DeviceID, &capture.DeviceName,
			&capture.Status, &capture.FullLogs, &capture.OutputPath, &capture.Error, &artifact); err != nil {
			return nil, err
		}
		capture.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		if startedAt.Valid {
			capture.StartedAt = startedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if finishedAt.Valid {
			capture.FinishedAt = finishedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if artifact.Valid {
			capture.ArtifactID = &artifact.Int64
		}
		out = append(out, capture)
	}
	return out, rows.Err()
}

func (s *Store) ClaimCapture(ctx context.Context, collectorID string) (*model.Capture, error) {
	if collectorID == "" {
		return nil, fmt.Errorf("collector id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var capture model.Capture
	var createdAt, startedAt time.Time
	err = tx.QueryRowContext(ctx, `WITH candidate AS (
  SELECT id FROM captures
  WHERE status='queued' OR (status='running' AND lease_expires_at < now())
  ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE captures SET status='running', started_at=COALESCE(started_at,now()), claimed_by=$1,
  lease_expires_at=now()+interval '30 minutes', attempts=attempts+1
WHERE id=(SELECT id FROM candidate)
RETURNING id,created_at,started_at,device_id,device_name,status,full_logs,output_path`, collectorID).Scan(
		&capture.ID, &createdAt, &startedAt, &capture.DeviceID, &capture.DeviceName, &capture.Status, &capture.FullLogs, &capture.OutputPath)
	if err == sql.ErrNoRows {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	capture.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	capture.StartedAt = startedAt.UTC().Format(time.RFC3339Nano)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &capture, nil
}

func (s *Store) RenewCaptureLease(ctx context.Context, id int64, collectorID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE captures SET lease_expires_at=now()+interval '30 minutes'
WHERE id=$1 AND status='running' AND claimed_by=$2`, id, collectorID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) CollectorHeartbeat(ctx context.Context, id, hostname string, pending int, details json.RawMessage) error {
	if !json.Valid(details) {
		details = json.RawMessage(`{}`)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO collectors(id,updated_at,hostname,status,pending_batches,details_json)
VALUES($1,now(),$2,'online',$3,$4::jsonb)
ON CONFLICT(id) DO UPDATE SET updated_at=excluded.updated_at,hostname=excluded.hostname,status='online',pending_batches=excluded.pending_batches,details_json=excluded.details_json`,
		id, hostname, pending, string(details))
	return err
}

func (s *Store) CollectorStatuses(ctx context.Context) ([]model.CollectorStatus, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,updated_at,hostname,status,pending_batches,details_json FROM collectors ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var statuses []model.CollectorStatus
	for rows.Next() {
		var status model.CollectorStatus
		var updatedAt time.Time
		var details []byte
		if err := rows.Scan(&status.ID, &updatedAt, &status.Hostname, &status.Status, &status.PendingBatches, &details); err != nil {
			return nil, err
		}
		status.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		status.Details = details
		if time.Since(updatedAt) > 30*time.Second {
			status.Status = "offline"
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (s *Store) DeviceInventory(ctx context.Context) ([]model.Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT details_json FROM collectors WHERE updated_at > now()-interval '30 seconds' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var devices []model.Device
	for rows.Next() {
		var details []byte
		if err := rows.Scan(&details); err != nil {
			return nil, err
		}
		var payload struct {
			Devices []model.Device `json:"devices"`
		}
		if json.Unmarshal(details, &payload) != nil {
			continue
		}
		for _, candidate := range payload.Devices {
			if candidate.ID == "" || seen[candidate.ID] {
				continue
			}
			seen[candidate.ID] = true
			devices = append(devices, candidate)
		}
	}
	return devices, rows.Err()
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339 timestamp %q", value)
	}
	return parsed.UTC(), nil
}

func nullTimestamp(value string) any {
	if value == "" {
		return nil
	}
	parsed, err := parseTimestamp(value)
	if err != nil {
		return nil
	}
	return parsed
}

func IntParam(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
