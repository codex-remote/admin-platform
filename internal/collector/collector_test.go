package collector

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReadBoundedLineCapsMemoryAndConsumesOversizedRecord(t *testing.T) {
	input := strings.Repeat("x", 1024) + "\nnext\n"
	reader := bufio.NewReaderSize(strings.NewReader(input), 32)
	line, consumed, complete, oversized, err := readBoundedLine(reader, 128)
	if err != nil {
		t.Fatal(err)
	}
	if !complete || !oversized || len(line) != 128 || consumed != 1025 {
		t.Fatalf("unexpected result: len=%d consumed=%d complete=%v oversized=%v", len(line), consumed, complete, oversized)
	}
	next, _, complete, oversized, err := readBoundedLine(reader, 128)
	if err != nil || !complete || oversized || string(next) != "next" {
		t.Fatalf("unexpected next line: %q %v", next, err)
	}
}

func TestRotatedLogFilesOldestFirst(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "relay.log")
	for _, path := range []string{base, base + ".1", base + ".2", base + ".gz"} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := rotatedLogFiles(base, "relay-server", "simulator")
	if len(files) != 3 || files[0].Path != base+".2" || files[2].Path != base {
		t.Fatalf("unexpected order: %+v", files)
	}
}

func TestCollectorRetriesSpoolBeforeAdvancingCheckpoint(t *testing.T) {
	var mu sync.Mutex
	status := http.StatusServiceUnavailable
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ingest/events" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"capture":null}`))
			return
		}
		mu.Lock()
		defer mu.Unlock()
		requests++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"inserted":1}`))
	}))
	defer server.Close()

	root := t.TempDir()
	logPath := filepath.Join(root, "relay-server", ".run", "debug", "relay.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	line := []byte("{\"time\":\"2026-08-13T01:02:03Z\",\"level\":\"INFO\",\"msg\":\"relay started\"}\n")
	if err := os.WriteFile(logPath, line, 0o600); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, "state")
	value, err := New(Config{AdminURL: server.URL, StateDir: stateDir, WorkspaceRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.RunOnce(t.Context()); err == nil {
		t.Fatal("expected first upload to fail")
	}
	if _, ok := value.state.Files[logPath]; ok {
		t.Fatal("checkpoint advanced before acknowledgement")
	}
	spool, _ := filepath.Glob(filepath.Join(stateDir, "spool", "*.json"))
	if len(spool) != 1 {
		t.Fatalf("expected one durable spool file, got %d", len(spool))
	}

	mu.Lock()
	status = http.StatusOK
	mu.Unlock()
	if err := value.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := value.state.Files[logPath].Offset; got != int64(len(line)) {
		t.Fatalf("checkpoint=%d want=%d", got, len(line))
	}
	spool, _ = filepath.Glob(filepath.Join(stateDir, "spool", "*.json"))
	if len(spool) != 0 {
		t.Fatal("acknowledged spool was not removed")
	}
	if err := value.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 2 {
		t.Fatalf("unexpected duplicate upload; requests=%d", gotRequests)
	}
}

func TestCollectorLeavesPartialLineUnread(t *testing.T) {
	var batches []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ingest/events" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"capture":null}`))
			return
		}
		var payload struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		batches = append(batches, len(payload.Events))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	root := t.TempDir()
	logPath := filepath.Join(root, "mac-agent", ".run", "debug", "mac-agent.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	complete := "{\"time\":\"2026-08-13T01:02:03Z\",\"level\":\"INFO\",\"msg\":\"agent started\"}\n"
	partial := "{\"time\":\"2026-08-13T01:02:04Z\",\"level\":\"INFO\",\"msg\":\"agent connected\"}"
	if err := os.WriteFile(logPath, []byte(complete+partial), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := New(Config{AdminURL: server.URL, StateDir: filepath.Join(root, "state"), WorkspaceRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := value.state.Files[logPath].Offset; got != int64(len(complete)) {
		t.Fatalf("checkpoint consumed partial line: %d", got)
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := value.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || batches[0] != 1 || batches[1] != 1 {
		t.Fatalf("unexpected batches: %v", batches)
	}
}
