package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codex-remote/admin-platform/internal/model"
	"github.com/codex-remote/admin-platform/internal/store"
)

type Parser struct{ Store *store.Store }

const maxArtifactBytes int64 = 4 * 1024 * 1024 * 1024

type iphoneExport struct {
	Manifest map[string]any `json:"manifest"`
	Events   []struct {
		Timestamp string          `json:"timestamp"`
		SessionID string          `json:"session_id"`
		Sequence  int64           `json:"sequence"`
		Level     string          `json:"level"`
		Category  string          `json:"category"`
		Event     string          `json:"event"`
		TraceID   string          `json:"trace_id"`
		TurnRef   string          `json:"turn_ref"`
		Fields    json.RawMessage `json:"fields"`
	} `json:"events"`
	MetricKitReports []any `json:"metrickit_reports"`
}

func (p Parser) ImportIPhoneExport(ctx context.Context, data []byte, artifactID int64) (int, error) {
	var payload iphoneExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, fmt.Errorf("decode diagnostics export: %w", err)
	}
	if len(payload.Events) == 0 && payload.Manifest == nil {
		return 0, fmt.Errorf("not a CodexRemote diagnostics export")
	}
	events := make([]model.Event, 0, len(payload.Events)+len(payload.MetricKitReports))
	for _, item := range payload.Events {
		fingerprint := hashStrings("iphone-app", item.SessionID, fmt.Sprint(item.Sequence), item.Event, item.Timestamp)
		events = append(events, model.Event{Timestamp: item.Timestamp, Source: "iphone-app", Level: item.Level,
			Category: item.Category, Name: item.Event, SessionID: item.SessionID, TraceID: item.TraceID,
			TurnRef: item.TurnRef, Sequence: item.Sequence, Fields: item.Fields, ArtifactID: &artifactID, Fingerprint: fingerprint})
	}
	for index, report := range payload.MetricKitReports {
		fields, _ := json.Marshal(report)
		events = append(events, model.Event{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Source: "iphone-system",
			Level: "notice", Category: "metrickit", Name: "metrickit.report", Fields: fields, ArtifactID: &artifactID,
			Fingerprint: hashStrings("metrickit", fmt.Sprint(artifactID), fmt.Sprint(index))})
	}
	return p.Store.InsertEvents(ctx, events)
}

func ParseSlogLine(line []byte, source, profile, path string, offset int64) (model.Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return model.Event{}, err
	}
	fields := map[string]any{}
	for key, value := range raw {
		if key != "time" && key != "level" && key != "msg" {
			fields[key] = value
		}
	}
	encoded, _ := json.Marshal(fields)
	message, _ := raw["msg"].(string)
	level, _ := raw["level"].(string)
	if strings.EqualFold(level, "warn") {
		level = "warning"
	}
	timestamp, _ := raw["time"].(string)
	traceID := stringField(raw, "trace_id")
	turnRef := stringField(raw, "turn_ref")
	name := slogEventName(source, message)
	return model.Event{Timestamp: timestamp, Source: source, Profile: profile, Level: strings.ToLower(level),
		Category: source, Name: name, Message: message, TraceID: traceID, TurnRef: turnRef, Fields: encoded,
		Fingerprint: hashStrings(source, profile, string(line))}, nil
}

func ParseIPhoneLine(line []byte, profile, path string, offset int64) (model.Event, error) {
	var raw struct {
		Timestamp string          `json:"timestamp"`
		SessionID string          `json:"session_id"`
		Sequence  int64           `json:"sequence"`
		Level     string          `json:"level"`
		Category  string          `json:"category"`
		Event     string          `json:"event"`
		TraceID   string          `json:"trace_id"`
		TurnRef   string          `json:"turn_ref"`
		Fields    json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return model.Event{}, err
	}
	if raw.Event == "" || raw.SessionID == "" {
		return model.Event{}, fmt.Errorf("missing iPhone event or session_id")
	}
	return model.Event{Timestamp: raw.Timestamp, Source: "iphone-app", Profile: profile, Level: raw.Level,
		Category: raw.Category, Name: raw.Event, SessionID: raw.SessionID, TraceID: raw.TraceID,
		TurnRef: raw.TurnRef, Sequence: raw.Sequence, Fields: raw.Fields,
		Fingerprint: hashStrings("iphone-app", raw.SessionID, fmt.Sprint(raw.Sequence), raw.Event, raw.Timestamp)}, nil
}

func slogEventName(source, message string) string {
	value := strings.ToLower(message)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
			b.WriteByte('_')
		}
	}
	return source + "." + strings.Trim(b.String(), "_")
}

func stringField(values map[string]any, key string) string {
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func hashStrings(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}

func SaveArtifact(directory, name string, source io.Reader) (string, int64, string, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", 0, "", err
	}
	base := filepath.Base(name)
	if base == "." || base == string(filepath.Separator) {
		base = "diagnostics.json"
	}
	path := filepath.Join(directory, fmt.Sprintf("%d-%s", time.Now().UnixNano(), base))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, "", err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(source, maxArtifactBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", 0, "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", 0, "", closeErr
	}
	if size > maxArtifactBytes {
		_ = os.Remove(path)
		return "", 0, "", fmt.Errorf("artifact exceeds 4 GiB")
	}
	return path, size, hex.EncodeToString(hash.Sum(nil)), nil
}
