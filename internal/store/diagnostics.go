package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/codex-remote/admin-platform/internal/model"
)

func eventWhere(q model.EventQuery) (string, []any) {
	clauses := []string{"timestamp >= $1", "timestamp <= $2"}
	args := []any{q.From.UTC(), q.To.UTC()}
	addValues := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		placeholders := make([]string, 0, len(values))
		for _, value := range values {
			args = append(args, value)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		clauses = append(clauses, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ",")))
	}
	addValues("source", q.Sources)
	addValues("profile", q.Profiles)
	addValues("level", q.Levels)
	addValues("category", q.Categories)
	addValues("event", q.Names)
	addValue := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addValue("session_id", q.SessionID)
	addValue("trace_id", q.TraceID)
	addValue("turn_ref", q.TurnRef)
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		clauses = append(clauses, "(event ILIKE "+placeholder+" OR message ILIKE "+placeholder+" OR fields_json::text ILIKE "+placeholder+")")
	}
	if !q.CursorAt.IsZero() && q.CursorID > 0 {
		args = append(args, q.CursorAt.UTC(), q.CursorID)
		clauses = append(clauses, fmt.Sprintf("(timestamp,id) < ($%d,$%d)", len(args)-1, len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (s *Store) QueryEvents(ctx context.Context, q model.EventQuery) ([]model.Event, error) {
	if q.Limit <= 0 || q.Limit > 501 {
		q.Limit = 201
	}
	where, args := eventWhere(q)
	args = append(args, q.Limit)
	query := `SELECT id,timestamp,received_at,source,profile,level,category,event,message,
session_id,trace_id,turn_ref,sequence,fields_json,artifact_id FROM events WHERE ` + where +
		fmt.Sprintf(` ORDER BY timestamp DESC,id DESC LIMIT $%d`, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Event, 0, q.Limit)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) EventFacets(ctx context.Context, q model.EventQuery) (model.EventFacets, error) {
	where, args := eventWhere(q)
	load := func(column string) ([]model.FacetValue, error) {
		query := fmt.Sprintf(`SELECT %s,COUNT(*) FROM events WHERE %s AND %s <> '' GROUP BY %s ORDER BY COUNT(*) DESC,%s LIMIT 100`, column, where, column, column, column)
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		values := make([]model.FacetValue, 0)
		for rows.Next() {
			var value model.FacetValue
			if err := rows.Scan(&value.Value, &value.Count); err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, rows.Err()
	}
	var result model.EventFacets
	var err error
	if result.Sources, err = load("source"); err != nil {
		return result, err
	}
	if result.Profiles, err = load("profile"); err != nil {
		return result, err
	}
	if result.Levels, err = load("level"); err != nil {
		return result, err
	}
	if result.Categories, err = load("category"); err != nil {
		return result, err
	}
	if result.Names, err = load("event"); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) EventHistogram(ctx context.Context, q model.EventQuery, interval time.Duration) ([]model.HistogramBucket, error) {
	where, args := eventWhere(q)
	seconds := int64(interval.Seconds())
	args = append(args, seconds)
	query := `SELECT to_timestamp(floor(extract(epoch FROM timestamp)/$` + fmt.Sprint(len(args)) + `)*$` + fmt.Sprint(len(args)) + `) AS bucket,
COUNT(*),COUNT(*) FILTER (WHERE level IN ('error','fault')),COUNT(*) FILTER (WHERE level='warning')
FROM events WHERE ` + where + ` GROUP BY bucket ORDER BY bucket`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]model.HistogramBucket, 0)
	for rows.Next() {
		var bucket model.HistogramBucket
		var start time.Time
		if err := rows.Scan(&start, &bucket.Count, &bucket.Errors, &bucket.Warnings); err != nil {
			return nil, err
		}
		bucket.Start = start.UTC().Format(time.RFC3339Nano)
		values = append(values, bucket)
	}
	return values, rows.Err()
}
