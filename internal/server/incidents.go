package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/codex-remote/admin-platform/internal/incident"
)

type incidentService interface {
	Create(context.Context, incident.CreateInput) (*incident.Snapshot, error)
	List(context.Context, int) ([]incident.Summary, error)
	Snapshot(context.Context, int64) (*incident.Snapshot, error)
}

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	items, err := s.incidents.List(r.Context(), storeLimit(r.URL.Query().Get("limit"), 50))
	respond(w, listEnvelope[incident.Summary]{Items: nonNil(items), Meta: localQueryMeta(time.Now(), time.Time{}, time.Time{})}, err)
}

func (s *Server) createIncident(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input incident.CreateInput
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := s.incidents.Create(r.Context(), input)
	if writeIncidentError(w, err) {
		return
	}
	s.hub.publish()
	writeJSON(w, http.StatusCreated, snapshot)
}

func (s *Server) getIncident(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.incidentSnapshot(r)
	if writeIncidentError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) downloadIncidentSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.incidentSnapshot(r)
	if writeIncidentError(w, err) {
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="incident-%d-snapshot.json"`, snapshot.Incident.ID))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) incidentSnapshot(r *http.Request) (*incident.Snapshot, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, incident.ValidationError{Message: "invalid incident id"}
	}
	return s.incidents.Snapshot(r.Context(), id)
}

func writeIncidentError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var validation incident.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, validation.Error())
	case errors.Is(err, incident.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return true
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func storeLimit(value string, fallback int) int {
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 || limit > 200 {
		return fallback
	}
	return limit
}
