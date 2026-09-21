package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codex-remote/admin-platform/internal/incident"
	"github.com/codex-remote/admin-platform/internal/model"
)

const (
	defaultEventWindow = 30 * time.Minute
	maxEventWindow     = 24 * time.Hour
	defaultEventLimit  = 200
)

type queryMeta struct {
	DataSource  string `json:"data_source"`
	Timezone    string `json:"timezone"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	QueryTimeMS int64  `json:"query_time_ms"`
}

type listEnvelope[T any] struct {
	Items      []T       `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
	Truncated  bool      `json:"truncated"`
	Meta       queryMeta `json:"meta"`
}

func (s *Server) apiV1Overview(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	value, err := s.store.Overview(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	value["meta"] = localQueryMeta(started, time.Time{}, time.Time{})
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) apiV1Events(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	query, err := parseEventQuery(r, started.UTC())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	requestedLimit := query.Limit
	query.Limit++
	items, err := s.store.QueryEvents(r.Context(), query)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	truncated := len(items) > requestedLimit
	if truncated {
		items = items[:requestedLimit]
	}
	next := ""
	if truncated && len(items) > 0 {
		next = encodeEventCursor(items[len(items)-1])
	}
	writeJSON(w, http.StatusOK, listEnvelope[model.Event]{Items: nonNil(items), NextCursor: next, Truncated: truncated, Meta: localQueryMeta(started, query.From, query.To)})
}

func (s *Server) apiV1Event(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_event_id", "event id must be a positive integer")
		return
	}
	item, err := s.store.Event(r.Context(), id)
	if err != nil {
		if err == incident.ErrNotFound {
			writeAPIError(w, http.StatusNotFound, "event_not_found", "event not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item, "meta": localQueryMeta(started, time.Time{}, time.Time{})})
}

func (s *Server) apiV1Facets(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	query, err := parseEventQuery(r, started.UTC())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	query.CursorAt, query.CursorID = time.Time{}, 0
	values, err := s.store.EventFacets(r.Context(), query)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"facets": values, "meta": localQueryMeta(started, query.From, query.To)})
}

func (s *Server) apiV1Histogram(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	query, err := parseEventQuery(r, started.UTC())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	query.CursorAt, query.CursorID = time.Time{}, 0
	interval := histogramInterval(query.To.Sub(query.From))
	buckets, err := s.store.EventHistogram(r.Context(), query, interval)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"buckets": nonNil(buckets), "interval_seconds": int64(interval.Seconds()), "meta": localQueryMeta(started, query.From, query.To)})
}

func parseEventQuery(r *http.Request, now time.Time) (model.EventQuery, error) {
	values := r.URL.Query()
	to, err := parseQueryTime(values.Get("to"), now)
	if err != nil {
		return model.EventQuery{}, fmt.Errorf("to: %w", err)
	}
	from, err := parseQueryTime(values.Get("from"), to.Add(-defaultEventWindow))
	if err != nil {
		return model.EventQuery{}, fmt.Errorf("from: %w", err)
	}
	if !from.Before(to) {
		return model.EventQuery{}, fmt.Errorf("from must be before to")
	}
	if to.Sub(from) > maxEventWindow {
		return model.EventQuery{}, fmt.Errorf("time range must not exceed 24 hours")
	}
	limit := storeLimit(values.Get("limit"), defaultEventLimit)
	query := model.EventQuery{
		Sources: queryValues(values["source"]), Profiles: queryValues(values["profile"]), Levels: lowerValues(queryValues(values["level"])),
		Categories: queryValues(values["category"]), Names: queryValues(values["event"]), Search: strings.TrimSpace(values.Get("q")),
		SessionID: values.Get("session_id"), TraceID: values.Get("trace_id"), TurnRef: values.Get("turn_ref"),
		From: from, To: to, Limit: limit,
	}
	if cursor := values.Get("cursor"); cursor != "" {
		query.CursorAt, query.CursorID, err = decodeEventCursor(cursor)
		if err != nil {
			return model.EventQuery{}, err
		}
	}
	return query, nil
}

func parseQueryTime(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback.UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be RFC3339")
	}
	return parsed.UTC(), nil
}

func queryValues(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				result = append(result, part)
			}
		}
	}
	return result
}

func lowerValues(values []string) []string {
	for i := range values {
		values[i] = strings.ToLower(values[i])
	}
	return values
}

func encodeEventCursor(event model.Event) string {
	raw := event.Timestamp + "\n" + strconv.FormatInt(event.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeEventCursor(value string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("cursor is invalid")
	}
	parts := strings.Split(string(raw), "\n")
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("cursor is invalid")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("cursor is invalid")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return time.Time{}, 0, fmt.Errorf("cursor is invalid")
	}
	return timestamp.UTC(), id, nil
}

func histogramInterval(window time.Duration) time.Duration {
	switch {
	case window <= time.Hour:
		return time.Minute
	case window <= 6*time.Hour:
		return 5 * time.Minute
	case window <= 12*time.Hour:
		return 15 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func localQueryMeta(started time.Time, from, to time.Time) queryMeta {
	meta := queryMeta{DataSource: "postgresql/local", Timezone: "UTC", QueryTimeMS: time.Since(started).Milliseconds()}
	if !from.IsZero() {
		meta.From = from.UTC().Format(time.RFC3339Nano)
	}
	if !to.IsZero() {
		meta.To = to.UTC().Format(time.RFC3339Nano)
	}
	return meta
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
