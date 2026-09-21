package collector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codex-remote/admin-platform/internal/device"
	"github.com/codex-remote/admin-platform/internal/ingest"
	"github.com/codex-remote/admin-platform/internal/model"
)

const (
	maxBatchEvents = 500
	maxBatchBytes  = 4 * 1024 * 1024
	maxLineBytes   = 256 * 1024
)

type Config struct {
	AdminURL       string
	IngestToken    string
	StateDir       string
	WorkspaceRoot  string
	SimulatorUDID  string
	IPhoneBundleID string
	Interval       time.Duration
	CollectorID    string
}

type LogFile struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Profile string `json:"profile"`
	Format  string `json:"format"`
}

type checkpoint struct {
	Identity string `json:"identity"`
	Offset   int64  `json:"offset"`
}

type stateFile struct {
	Version int                   `json:"version"`
	Files   map[string]checkpoint `json:"files"`
}

type pendingBatch struct {
	Version    int           `json:"version"`
	CreatedAt  string        `json:"created_at"`
	Events     []model.Event `json:"events"`
	Path       string        `json:"path"`
	Checkpoint checkpoint    `json:"checkpoint"`
}

type Collector struct {
	config         Config
	client         *http.Client
	state          stateFile
	captureMu      sync.Mutex
	captureRunning bool
	lastHeartbeat  time.Time
}

func New(config Config) (*Collector, error) {
	if config.AdminURL == "" || config.StateDir == "" {
		return nil, fmt.Errorf("admin URL and state directory are required")
	}
	if config.Interval <= 0 {
		config.Interval = 2 * time.Second
	}
	if config.CollectorID == "" {
		config.CollectorID = "local-mac"
	}
	if err := os.MkdirAll(filepath.Join(config.StateDir, "spool"), 0o700); err != nil {
		return nil, err
	}
	collector := &Collector{config: config, client: &http.Client{Timeout: 10 * time.Second}, state: stateFile{Version: 1, Files: map[string]checkpoint{}}}
	if data, err := os.ReadFile(filepath.Join(config.StateDir, "checkpoints.json")); err == nil {
		if err := json.Unmarshal(data, &collector.state); err != nil {
			return nil, fmt.Errorf("decode collector checkpoints: %w", err)
		}
		if collector.state.Files == nil {
			collector.state.Files = map[string]checkpoint{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return collector, nil
}

func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()
	for {
		if err := c.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("collector cycle failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Collector) RunOnce(ctx context.Context) error {
	pending, err := filepath.Glob(filepath.Join(c.config.StateDir, "spool", "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(pending)
	if time.Since(c.lastHeartbeat) >= 10*time.Second {
		c.heartbeat(ctx, len(pending))
		c.lastHeartbeat = time.Now()
	}
	c.startCapture(ctx)
	if len(pending) > 0 {
		return c.flush(ctx, pending[0])
	}
	for _, file := range c.logFiles(ctx) {
		created, err := c.stage(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			slog.Warn("log file collection failed", "path", file.Path, "error", err)
			continue
		}
		if created != "" {
			return c.flush(ctx, created)
		}
	}
	return nil
}

func (c *Collector) heartbeat(ctx context.Context, pending int) {
	hostname, _ := os.Hostname()
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	devices, discoveryErr := device.SafeList(discoveryCtx)
	cancel()
	detailsPayload := map[string]any{"simulator": c.config.SimulatorUDID, "bundle_id": c.config.IPhoneBundleID, "devices": devices}
	if discoveryErr != nil {
		detailsPayload["device_discovery_error"] = discoveryErr.Error()
	}
	details, _ := json.Marshal(detailsPayload)
	payload, _ := json.Marshal(map[string]any{"id": c.config.CollectorID, "hostname": hostname, "pending_batches": pending, "details": json.RawMessage(details)})
	request, err := c.request(ctx, http.MethodPost, "/api/v1/collector/heartbeat", bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err == nil {
		response.Body.Close()
	}
}

func (c *Collector) startCapture(ctx context.Context) {
	c.captureMu.Lock()
	if c.captureRunning {
		c.captureMu.Unlock()
		return
	}
	c.captureMu.Unlock()
	request, err := c.request(ctx, http.MethodGet, "/api/v1/collector/captures/next?collector_id="+url.QueryEscape(c.config.CollectorID), nil)
	if err != nil {
		return
	}
	response, err := c.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return
	}
	var payload struct {
		Capture *model.Capture `json:"capture"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&payload) != nil || payload.Capture == nil {
		return
	}
	c.captureMu.Lock()
	if c.captureRunning {
		c.captureMu.Unlock()
		return
	}
	c.captureRunning = true
	c.captureMu.Unlock()
	go func(capture model.Capture) {
		defer func() { c.captureMu.Lock(); c.captureRunning = false; c.captureMu.Unlock() }()
		c.runCapture(context.Background(), capture)
	}(*payload.Capture)
}

func (c *Collector) runCapture(ctx context.Context, capture model.Capture) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	leaseDone := make(chan struct{})
	defer close(leaseDone)
	go c.renewCaptureLease(ctx, capture.ID, leaseDone)
	output := filepath.Join(c.config.StateDir, "captures", fmt.Sprint(capture.ID))
	rawDeviceID, err := device.Resolve(ctx, capture.DeviceID)
	if err != nil {
		c.reportCapture(context.Background(), capture.ID, "failed", err.Error(), nil)
		return
	}
	err = device.Sysdiagnose(ctx, rawDeviceID, output, capture.FullLogs)
	if err != nil {
		c.reportCapture(context.Background(), capture.ID, "failed", err.Error(), nil)
		return
	}
	path, err := device.NewestArtifact(output)
	if err != nil {
		c.reportCapture(context.Background(), capture.ID, "failed", err.Error(), nil)
		return
	}
	artifactID, err := c.uploadArtifact(ctx, path, "sysdiagnose", "iphone-system")
	if err != nil {
		c.reportCapture(context.Background(), capture.ID, "failed", err.Error(), nil)
		return
	}
	c.reportCapture(context.Background(), capture.ID, "completed", "", &artifactID)
}

func (c *Collector) renewCaptureLease(ctx context.Context, id int64, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			path := fmt.Sprintf("/api/v1/collector/captures/%d/renew?collector_id=%s", id, url.QueryEscape(c.config.CollectorID))
			request, err := c.request(ctx, http.MethodPost, path, nil)
			if err != nil {
				continue
			}
			response, err := c.client.Do(request)
			if err == nil {
				response.Body.Close()
			}
		}
	}
}

func (c *Collector) uploadArtifact(ctx context.Context, path, kind, source string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	go func() {
		part, createErr := writer.CreateFormFile("file", filepath.Base(path))
		if createErr == nil {
			_, createErr = io.Copy(part, file)
		}
		if createErr == nil {
			createErr = writer.WriteField("kind", kind)
		}
		if createErr == nil {
			createErr = writer.WriteField("source", source)
		}
		if closeErr := writer.Close(); createErr == nil {
			createErr = closeErr
		}
		_ = pipeWriter.CloseWithError(createErr)
	}()
	request, err := c.request(ctx, http.MethodPost, "/api/v1/artifacts", pipeReader)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := (&http.Client{Timeout: 30 * time.Minute}).Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return 0, fmt.Errorf("artifact upload returned %s: %s", response.Status, body)
	}
	var result struct {
		Artifact model.Artifact `json:"artifact"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Artifact.ID, nil
}

func (c *Collector) reportCapture(ctx context.Context, id int64, status, message string, artifactID *int64) {
	payload, _ := json.Marshal(map[string]any{"status": status, "error": message, "artifact_id": artifactID})
	request, err := c.request(ctx, http.MethodPatch, fmt.Sprintf("/api/v1/collector/captures/%d", id), bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err == nil {
		response.Body.Close()
	}
}

func (c *Collector) logFiles(ctx context.Context) []LogFile {
	files := make([]LogFile, 0, 24)
	for _, profile := range []string{"debug", "simulator", "iphone"} {
		files = append(files, rotatedLogFiles(filepath.Join(c.config.WorkspaceRoot, "relay-server", ".run", profile, "relay.log"), "relay-server", profile)...)
		files = append(files, rotatedLogFiles(filepath.Join(c.config.WorkspaceRoot, "mac-agent", ".run", profile, "mac-agent.log"), "mac-agent", profile)...)
	}
	if diagnostics := c.simulatorDiagnosticsPath(ctx); diagnostics != "" {
		matches, _ := filepath.Glob(filepath.Join(diagnostics, "events*.jsonl"))
		sort.Slice(matches, func(i, j int) bool { return rotationIndex(matches[i]) > rotationIndex(matches[j]) })
		for _, path := range matches {
			files = append(files, LogFile{Path: path, Source: "iphone-app", Profile: "simulator", Format: "iphone"})
		}
	}
	return files
}

func rotatedLogFiles(base, source, profile string) []LogFile {
	matches, _ := filepath.Glob(base + "*")
	var paths []string
	for _, path := range matches {
		name := filepath.Base(path)
		if name == filepath.Base(base) || strings.HasPrefix(name, filepath.Base(base)+".") && !strings.HasSuffix(name, ".gz") {
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				paths = append(paths, path)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	files := make([]LogFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, LogFile{Path: path, Source: source, Profile: profile, Format: "slog"})
	}
	return files
}

func rotationIndex(path string) int {
	base := filepath.Base(path)
	if base == "events.jsonl" {
		return 0
	}
	var index int
	if _, err := fmt.Sscanf(base, "events.%d.jsonl", &index); err != nil {
		return 0
	}
	return index
}

func (c *Collector) simulatorDiagnosticsPath(ctx context.Context) string {
	if c.config.IPhoneBundleID == "" {
		return ""
	}
	udid := c.config.SimulatorUDID
	if udid == "" {
		udid = "booted"
	}
	command := exec.CommandContext(ctx, "xcrun", "simctl", "get_app_container", udid, c.config.IPhoneBundleID, "data")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(string(output)), "Library", "Application Support", "Diagnostics")
}

func (c *Collector) stage(config LogFile) (string, error) {
	file, err := os.Open(config.Path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	identity := fileIdentity(info)
	previous := c.state.Files[config.Path]
	offset := previous.Offset
	if previous.Identity != "" && (previous.Identity != identity || info.Size() < offset) {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}
	reader := bufio.NewReaderSize(file, 64*1024)
	cursor := offset
	events := make([]model.Event, 0, maxBatchEvents)
	batchBytes := 0
	for len(events) < maxBatchEvents {
		line, consumed, complete, oversized, readErr := readBoundedLine(reader, maxLineBytes)
		if complete {
			lineStart := cursor
			cursor += consumed
			batchBytes += len(line)
			if len(line) > 0 {
				event, parseErr := parseLine(line, config, lineStart)
				if oversized {
					parseErr = fmt.Errorf("structured log line exceeds %d bytes", maxLineBytes)
				}
				if parseErr != nil {
					event = parseFailure(config, lineStart, line, parseErr)
				}
				events = append(events, event)
			}
		}
		if batchBytes >= maxBatchBytes {
			break
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", readErr
		}
	}
	if cursor == offset {
		return "", nil
	}
	batch := pendingBatch{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Events: events, Path: config.Path, Checkpoint: checkpoint{Identity: identity, Offset: cursor}}
	data, err := json.Marshal(batch)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", config.Path, identity, cursor)))
	path := filepath.Join(c.config.StateDir, "spool", fmt.Sprintf("%d-%s.json", time.Now().UnixNano(), hex.EncodeToString(hash[:6])))
	if err := writeAtomic(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func readBoundedLine(reader *bufio.Reader, limit int) ([]byte, int64, bool, bool, error) {
	line := make([]byte, 0, min(limit, 64*1024))
	var consumed int64
	oversized := false
	for {
		fragment, err := reader.ReadSlice('\n')
		consumed += int64(len(fragment))
		if len(line) < limit {
			remaining := limit - len(line)
			if len(fragment) > remaining {
				line = append(line, fragment[:remaining]...)
				oversized = true
			} else {
				line = append(line, fragment...)
			}
		} else if len(fragment) > 0 {
			oversized = true
		}
		if err == nil {
			line = bytes.TrimSuffix(line, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
			return line, consumed, true, oversized, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			return line, consumed, false, oversized, io.EOF
		}
		return line, consumed, false, oversized, err
	}
}

func (c *Collector) flush(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var batch pendingBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"events": batch.Events})
	if err != nil {
		return err
	}
	request, err := c.request(ctx, http.MethodPost, "/api/v1/ingest/events", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return fmt.Errorf("admin ingest returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	c.state.Files[batch.Path] = batch.Checkpoint
	stateData, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(c.config.StateDir, "checkpoints.json"), stateData, 0o600); err != nil {
		return err
	}
	return os.Remove(path)
}

func (c *Collector) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.AdminURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if c.config.IngestToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.config.IngestToken)
	}
	return request, nil
}

func parseLine(line []byte, config LogFile, offset int64) (model.Event, error) {
	if config.Format == "iphone" {
		return ingest.ParseIPhoneLine(line, config.Profile, config.Path, offset)
	}
	return ingest.ParseSlogLine(line, config.Source, config.Profile, config.Path, offset)
}

func parseFailure(config LogFile, offset int64, line []byte, cause error) model.Event {
	hash := sha256.Sum256(append([]byte(config.Path+fmt.Sprint(offset)), line...))
	fields, _ := json.Marshal(map[string]any{"path": filepath.Base(config.Path), "offset": offset, "error": cause.Error(), "line_bytes": len(line)})
	return model.Event{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Source: "diagnostics-collector", Profile: config.Profile,
		Level: "warning", Category: "collector", Name: "collector.parse_failed", Message: "Structured log line could not be parsed",
		Fields: fields, Fingerprint: hex.EncodeToString(hash[:])}
}

func fileIdentity(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
	}
	return fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, mode); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
