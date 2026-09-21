package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codex-remote/admin-platform/internal/incident"
)

type fakeIncidentService struct {
	created incident.CreateInput
	result  *incident.Snapshot
	err     error
}

func (f *fakeIncidentService) Create(_ context.Context, input incident.CreateInput) (*incident.Snapshot, error) {
	f.created = input
	return f.result, f.err
}
func (f *fakeIncidentService) List(context.Context, int) ([]incident.Summary, error) {
	return nil, f.err
}
func (f *fakeIncidentService) Snapshot(context.Context, int64) (*incident.Snapshot, error) {
	return f.result, f.err
}

func TestCreateIncidentHTTPContract(t *testing.T) {
	service := &fakeIncidentService{result: &incident.Snapshot{SchemaVersion: incident.SnapshotSchemaVersion, Incident: incident.Summary{ID: 7}}}
	server := &Server{incidents: service, hub: newEventHub()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(`{"title":"OOM","anchor_event_id":42}`))
	response := httptest.NewRecorder()
	server.createIncident(response, request)
	if response.Code != http.StatusCreated || service.created.AnchorEventID != 42 || !strings.Contains(response.Body.String(), incident.SnapshotSchemaVersion) {
		t.Fatalf("unexpected response %d %s, input=%+v", response.Code, response.Body.String(), service.created)
	}
}

func TestIncidentHTTPErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{{"validation", incident.ValidationError{Message: "bad"}, http.StatusBadRequest}, {"missing", incident.ErrNotFound, http.StatusNotFound}, {"internal", errors.New("db"), http.StatusInternalServerError}} {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{incidents: &fakeIncidentService{err: test.err}, hub: newEventHub()}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(`{"title":"OOM","anchor_event_id":42}`))
			response := httptest.NewRecorder()
			server.createIncident(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
