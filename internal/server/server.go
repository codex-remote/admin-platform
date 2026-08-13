package server

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/incident"
	"github.com/ai-coding-remote/admin-platform/internal/ingest"
	"github.com/ai-coding-remote/admin-platform/internal/model"
	"github.com/ai-coding-remote/admin-platform/internal/store"
)

//go:embed web-dist
var webAssets embed.FS

type Config struct {
	WorkspaceRoot string
	DataDir       string
	IngestToken   string
}

type Server struct {
	store     *store.Store
	config    Config
	parser    ingest.Parser
	incidents incidentService
	hub       *eventHub
}

func New(db *store.Store, config Config) *Server {
	return &Server{store: db, config: config, parser: ingest.Parser{Store: db}, incidents: incident.NewService(db), hub: newEventHub()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/overview", s.apiV1Overview)
	mux.HandleFunc("GET /api/v1/diagnostics/events", s.apiV1Events)
	mux.HandleFunc("GET /api/v1/diagnostics/events/{id}", s.apiV1Event)
	mux.HandleFunc("GET /api/v1/diagnostics/facets", s.apiV1Facets)
	mux.HandleFunc("GET /api/v1/diagnostics/histogram", s.apiV1Histogram)
	mux.HandleFunc("POST /api/v1/ingest/events", s.ingestEvents)
	mux.HandleFunc("POST /api/v1/collector/heartbeat", s.collectorHeartbeat)
	mux.HandleFunc("GET /api/v1/collector/captures/next", s.claimCapture)
	mux.HandleFunc("POST /api/v1/collector/captures/{id}/renew", s.renewCapture)
	mux.HandleFunc("PATCH /api/v1/collector/captures/{id}", s.completeCapture)
	mux.HandleFunc("GET /api/v1/stream", s.stream)
	mux.HandleFunc("GET /api/v1/services", s.services)
	mux.HandleFunc("GET /api/v1/devices", s.devices)
	mux.HandleFunc("POST /api/v1/captures", s.createCapture)
	mux.HandleFunc("GET /api/v1/captures", s.captures)
	mux.HandleFunc("POST /api/v1/artifacts", s.uploadArtifact)
	mux.HandleFunc("GET /api/v1/artifacts", s.artifacts)
	mux.HandleFunc("GET /api/v1/artifacts/{id}", s.downloadArtifact)
	mux.HandleFunc("GET /api/v1/incidents", s.listIncidents)
	mux.HandleFunc("POST /api/v1/incidents", s.createIncident)
	mux.HandleFunc("GET /api/v1/incidents/{id}", s.getIncident)
	mux.HandleFunc("GET /api/v1/incidents/{id}/snapshot", s.downloadIncidentSnapshot)
	mux.HandleFunc("GET /api/healthz", s.health)
	static, _ := fs.Sub(webAssets, "web-dist")
	fileServer := http.FileServer(http.FS(static))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if _, err := fs.Stat(static, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
	return securityHeaders(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Health(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "database": "unavailable"})
		return
	}
	user, timezone, err := s.store.RuntimeInfo(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "database": "postgresql", "database_user": user, "database_timezone": timezone})
}

func (s *Server) ingestEvents(w http.ResponseWriter, r *http.Request) {
	if s.config.IngestToken != "" && r.Header.Get("Authorization") != "Bearer "+s.config.IngestToken {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "invalid ingest token")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024*1024)
	var payload struct {
		Events []model.Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if len(payload.Events) > 1000 {
		writeAPIError(w, http.StatusBadRequest, "batch_too_large", "maximum 1000 events per request")
		return
	}
	for i := range payload.Events {
		if payload.Events[i].Source == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_event", "source is required")
			return
		}
		if payload.Events[i].Fingerprint == "" {
			payload.Events[i].Fingerprint = fingerprint(payload.Events[i])
		}
	}
	inserted, err := s.store.InsertEvents(r.Context(), payload.Events)
	if err == nil && inserted > 0 {
		s.hub.publish()
	}
	respond(w, map[string]any{"inserted": inserted}, err)
}

func (s *Server) services(w http.ResponseWriter, r *http.Request) {
	type service struct {
		Name      string `json:"name"`
		Source    string `json:"source"`
		Profile   string `json:"profile"`
		Status    string `json:"status"`
		URL       string `json:"url"`
		LatencyMS int64  `json:"latency_ms"`
		Detail    any    `json:"detail,omitempty"`
	}
	profiles := []struct {
		name string
		port int
	}{{"debug", 18765}, {"simulator", 18767}, {"iphone", 18768}}
	out := make([]service, 0, len(profiles)*2)
	client := http.Client{Timeout: 1200 * time.Millisecond}
	for _, profile := range profiles {
		url := fmt.Sprintf("http://127.0.0.1:%d/status", profile.port)
		started := time.Now()
		response, err := client.Get(url)
		latency := time.Since(started).Milliseconds()
		status := "offline"
		var detail any
		if err == nil {
			defer response.Body.Close()
			var raw any
			if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&raw) == nil {
				detail = raw
			}
			if response.StatusCode/100 == 2 {
				status = "online"
			} else {
				status = "degraded"
			}
		}
		out = append(out, service{Name: "Relay", Source: "relay-server", Profile: profile.name, Status: status, URL: url, LatencyMS: latency, Detail: detail})
		agentStatus := "offline"
		if values, ok := detail.(map[string]any); ok {
			if connected, _ := values["agent_connected"].(bool); connected {
				agentStatus = "online"
			}
		}
		out = append(out, service{Name: "Mac Agent", Source: "mac-agent", Profile: profile.name, Status: agentStatus, URL: url, LatencyMS: latency})
	}
	collectors, err := s.store.CollectorStatuses(r.Context())
	if err != nil {
		respond(w, nil, err)
		return
	}
	for _, collector := range collectors {
		out = append(out, service{Name: "Collector", Source: "diagnostics-collector", Profile: collector.ID,
			Status: collector.Status, URL: "local", Detail: map[string]any{"pending_batches": collector.PendingBatches, "hostname": collector.Hostname, "updated_at": collector.UpdatedAt}})
	}
	respond(w, listEnvelope[service]{Items: nonNil(out), Meta: localQueryMeta(time.Now(), time.Time{}, time.Time{})}, nil)
}

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.DeviceInventory(r.Context())
	respond(w, listEnvelope[model.Device]{Items: nonNil(values), Meta: localQueryMeta(time.Now(), time.Time{}, time.Time{})}, err)
}

func (s *Server) createCapture(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
		FullLogs   bool   `json:"full_logs"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.DeviceID == "" || request.DeviceName == "" {
		writeError(w, http.StatusBadRequest, "device_id and device_name are required")
		return
	}
	capture := model.Capture{DeviceID: request.DeviceID, DeviceName: request.DeviceName, Status: "queued", FullLogs: request.FullLogs}
	if err := s.store.CreateCapture(r.Context(), &capture); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, capture)
}

func (s *Server) collectorHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeCollector(w, r) {
		return
	}
	var request struct {
		ID             string          `json:"id"`
		Hostname       string          `json:"hostname"`
		PendingBatches int             `json:"pending_batches"`
		Details        json.RawMessage `json:"details"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.ID == "" {
		writeError(w, http.StatusBadRequest, "collector id is required")
		return
	}
	err := s.store.CollectorHeartbeat(r.Context(), request.ID, request.Hostname, request.PendingBatches, request.Details)
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (s *Server) claimCapture(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeCollector(w, r) {
		return
	}
	capture, err := s.store.ClaimCapture(r.Context(), r.URL.Query().Get("collector_id"))
	if err == nil && capture != nil {
		s.hub.publish()
	}
	respond(w, map[string]any{"capture": capture}, err)
}

func (s *Server) renewCapture(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeCollector(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid capture id")
		return
	}
	ok, err := s.store.RenewCaptureLease(r.Context(), id, r.URL.Query().Get("collector_id"))
	if err == nil && !ok {
		writeError(w, http.StatusConflict, "capture lease is no longer owned by collector")
		return
	}
	respond(w, map[string]any{"ok": ok}, err)
}

func (s *Server) completeCapture(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeCollector(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid capture id")
		return
	}
	var request struct {
		Status     string `json:"status"`
		Error      string `json:"error"`
		ArtifactID *int64 `json:"artifact_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Status != "completed" && request.Status != "failed" {
		writeError(w, http.StatusBadRequest, "status must be completed or failed")
		return
	}
	capture := model.Capture{ID: id, Status: request.Status, Error: request.Error, ArtifactID: request.ArtifactID,
		FinishedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	err = s.store.UpdateCapture(r.Context(), capture)
	if err == nil {
		s.hub.publish()
	}
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (s *Server) captures(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.ListCaptures(r.Context())
	respond(w, listEnvelope[model.Capture]{Items: nonNil(values), Meta: localQueryMeta(time.Now(), time.Time{}, time.Time{})}, err)
}

func (s *Server) uploadArtifact(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeCollector(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024*1024*1024+1024*1024)
	if err := r.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	kind, source := r.FormValue("kind"), r.FormValue("source")
	if kind == "" {
		kind = "diagnostics-export"
	}
	if source == "" {
		source = "iphone-app"
	}
	if kind != "diagnostics-export" && kind != "sysdiagnose" {
		writeError(w, http.StatusBadRequest, "unsupported artifact kind")
		return
	}
	if source != "iphone-app" && source != "iphone-system" {
		writeError(w, http.StatusBadRequest, "unsupported artifact source")
		return
	}
	path, size, sum, err := ingest.SaveArtifact(filepath.Join(s.config.DataDir, "artifacts"), header.Filename, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	artifact := model.Artifact{Kind: kind, Source: source, Name: filepath.Base(header.Filename), Path: path, SizeBytes: size, SHA256: sum, Metadata: `{}`}
	if err := s.store.CreateArtifact(r.Context(), &artifact); err != nil {
		_ = os.Remove(path)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	inserted := 0
	if kind == "diagnostics-export" {
		data, err := os.ReadFile(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		inserted, err = s.parser.ImportIPhoneExport(r.Context(), data, artifact.ID)
	}
	if err == nil {
		s.hub.publish()
	}
	respond(w, map[string]any{"artifact": artifact, "inserted": inserted}, err)
}

func (s *Server) authorizeCollector(w http.ResponseWriter, r *http.Request) bool {
	if s.config.IngestToken != "" && r.Header.Get("Authorization") != "Bearer "+s.config.IngestToken {
		writeError(w, http.StatusUnauthorized, "invalid ingest token")
		return false
	}
	return true
}

func (s *Server) artifacts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.ListArtifacts(r.Context(), store.IntParam(r.URL.Query().Get("limit"), 50))
	respond(w, listEnvelope[model.Artifact]{Items: nonNil(values), Meta: localQueryMeta(time.Now(), time.Time{}, time.Time{})}, err)
}
func (s *Server) downloadArtifact(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid artifact id")
		return
	}
	artifact, err := s.store.Artifact(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.Name))
	if kind := mime.TypeByExtension(filepath.Ext(artifact.Name)); kind != "" {
		w.Header().Set("Content-Type", kind)
	}
	http.ServeFile(w, r, artifact.Path)
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	channel, cancel := s.hub.subscribe()
	defer cancel()
	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-channel:
			fmt.Fprint(w, "event: changed\ndata: {}\n\n")
			flusher.Flush()
		case <-time.After(20 * time.Second):
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

type eventHub struct {
	mu          sync.Mutex
	next        int
	subscribers map[int]chan struct{}
}

func newEventHub() *eventHub { return &eventHub{subscribers: map[int]chan struct{}{}} }
func (h *eventHub) subscribe() (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.next
	h.next++
	ch := make(chan struct{}, 1)
	h.subscribers[id] = ch
	return ch, func() { h.mu.Lock(); delete(h.subscribers, id); close(ch); h.mu.Unlock() }
}
func (h *eventHub) publish() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	code := "request_failed"
	switch status {
	case http.StatusBadRequest:
		code = "invalid_request"
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusConflict:
		code = "conflict"
	case http.StatusRequestEntityTooLarge:
		code = "payload_too_large"
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		code = "internal_error"
	}
	writeAPIError(w, status, code, message)
}
func fingerprint(event model.Event) string {
	data, _ := json.Marshal(event)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
