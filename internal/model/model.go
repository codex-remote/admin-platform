package model

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID          int64           `json:"id"`
	Timestamp   string          `json:"timestamp"`
	ReceivedAt  string          `json:"received_at"`
	Source      string          `json:"source"`
	Profile     string          `json:"profile,omitempty"`
	Level       string          `json:"level"`
	Category    string          `json:"category"`
	Name        string          `json:"event"`
	Message     string          `json:"message,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	TraceID     string          `json:"trace_id,omitempty"`
	TurnRef     string          `json:"turn_ref,omitempty"`
	Sequence    int64           `json:"sequence,omitempty"`
	Fields      json.RawMessage `json:"fields"`
	ArtifactID  *int64          `json:"artifact_id,omitempty"`
	Fingerprint string          `json:"-"`
}

func (e *Event) Normalize() {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.ReceivedAt == "" {
		e.ReceivedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.Level == "" {
		e.Level = "info"
	}
	if e.Level == "warn" {
		e.Level = "warning"
	}
	if e.Category == "" {
		e.Category = e.Source
	}
	if len(e.Fields) == 0 {
		e.Fields = json.RawMessage(`{}`)
	}
}

type EventQuery struct {
	Sources    []string
	Profiles   []string
	Levels     []string
	Categories []string
	Names      []string
	Search     string
	SessionID  string
	TraceID    string
	TurnRef    string
	From       time.Time
	To         time.Time
	CursorAt   time.Time
	CursorID   int64
	Limit      int
}

type FacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type EventFacets struct {
	Sources    []FacetValue `json:"source"`
	Profiles   []FacetValue `json:"profile"`
	Levels     []FacetValue `json:"level"`
	Categories []FacetValue `json:"category"`
	Names      []FacetValue `json:"event"`
}

type HistogramBucket struct {
	Start    string `json:"start"`
	Count    int64  `json:"count"`
	Errors   int64  `json:"errors"`
	Warnings int64  `json:"warnings"`
}

type Artifact struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Name      string `json:"name"`
	Path      string `json:"-"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	Metadata  string `json:"metadata"`
}

type Capture struct {
	ID         int64  `json:"id"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Status     string `json:"status"`
	FullLogs   bool   `json:"full_logs"`
	OutputPath string `json:"-"`
	Error      string `json:"error,omitempty"`
	ArtifactID *int64 `json:"artifact_id,omitempty"`
}

type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MarketingName string `json:"marketing_name"`
	OSVersion     string `json:"os_version"`
	OSBuild       string `json:"os_build"`
	ProductType   string `json:"product_type"`
	Connected     bool   `json:"connected"`
	Paired        bool   `json:"paired"`
	Platform      string `json:"platform"`
}

type CollectorStatus struct {
	ID             string          `json:"id"`
	UpdatedAt      string          `json:"updated_at"`
	Hostname       string          `json:"hostname"`
	Status         string          `json:"status"`
	PendingBatches int             `json:"pending_batches"`
	Details        json.RawMessage `json:"details"`
}
