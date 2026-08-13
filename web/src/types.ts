export type QueryMeta = {
  data_source: string
  timezone: string
  from?: string
  to?: string
  query_time_ms: number
}

export type ListEnvelope<T> = {
  items: T[]
  next_cursor?: string
  truncated: boolean
  meta: QueryMeta
}

export type EventRecord = {
  id: number
  timestamp: string
  received_at: string
  source: string
  profile?: string
  level: string
  category: string
  event: string
  message?: string
  session_id?: string
  trace_id?: string
  turn_ref?: string
  sequence?: number
  fields: Record<string, unknown>
  artifact_id?: number
}

export type FacetValue = { value: string; count: number }
export type EventFacets = Record<"source" | "profile" | "level" | "category" | "event", FacetValue[]>
export type HistogramBucket = { start: string; count: number; errors: number; warnings: number }

export type Overview = {
  total_events: number
  warnings: number
  errors: number
  artifacts: number
  active_captures: number
  by_source: Record<string, number>
  meta: QueryMeta
}

export type Service = {
  name: string
  source: string
  profile: string
  status: string
  url: string
  latency_ms: number
  detail?: Record<string, unknown>
}

export type IncidentSummary = {
  id: number
  created_at: string
  title: string
  summary?: string
  severity: string
  status: string
  anchor_event_id?: number
  window_start: string
  window_end: string
  event_count: number
  artifact_count: number
  truncated: boolean
}

export type IncidentSnapshot = {
  schema_version: string
  created_at: string
  incident: IncidentSummary
  events: EventRecord[]
  artifacts: ArtifactRecord[]
  provenance: {
    data_source: string
    database_timezone: string
    privacy_policy: string
    source_counts: Record<string, number>
    truncated: boolean
  }
}

export type Device = {
  id: string
  name: string
  marketing_name: string
  os_version: string
  os_build: string
  product_type: string
  connected: boolean
  paired: boolean
  platform: string
}

export type Capture = {
  id: number
  created_at: string
  started_at?: string
  finished_at?: string
  device_id: string
  device_name: string
  status: string
  full_logs: boolean
  error?: string
  artifact_id?: number
}

export type ArtifactRecord = {
  id: number
  created_at: string
  kind: string
  source: string
  name: string
  size_bytes: number
  sha256: string
  metadata: string
}
