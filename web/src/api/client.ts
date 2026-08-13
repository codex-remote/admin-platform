import type {
  ArtifactRecord,
  Capture,
  Device,
  EventFacets,
  EventRecord,
  HistogramBucket,
  IncidentSnapshot,
  IncidentSummary,
  ListEnvelope,
  Overview,
  QueryMeta,
  Service,
} from "../types"

type APIErrorBody = { error?: { code?: string; message?: string } | string }

export class APIError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { ...init, headers: { Accept: "application/json", ...init?.headers } })
  if (!response.ok) {
    let body: APIErrorBody = {}
    try { body = await response.json() as APIErrorBody } catch { /* response is not JSON */ }
    const error = typeof body.error === "string" ? body.error : body.error?.message
    const code = typeof body.error === "object" ? body.error?.code : undefined
    throw new APIError(response.status, code ?? "request_failed", error ?? `请求失败 (${response.status})`)
  }
  return response.json() as Promise<T>
}

export const api = {
  overview: () => request<Overview>("/api/v1/overview"),
  services: () => request<ListEnvelope<Service>>("/api/v1/services"),
  events: (params: URLSearchParams) => request<ListEnvelope<EventRecord>>(`/api/v1/diagnostics/events?${params}`),
  event: (id: number) => request<{ item: EventRecord; meta: QueryMeta }>(`/api/v1/diagnostics/events/${id}`),
  facets: (params: URLSearchParams) => request<{ facets: EventFacets; meta: QueryMeta }>(`/api/v1/diagnostics/facets?${params}`),
  histogram: (params: URLSearchParams) => request<{ buckets: HistogramBucket[]; interval_seconds: number; meta: QueryMeta }>(`/api/v1/diagnostics/histogram?${params}`),
  incidents: () => request<ListEnvelope<IncidentSummary>>("/api/v1/incidents"),
  incident: (id: number) => request<IncidentSnapshot>(`/api/v1/incidents/${id}`),
  createIncident: (input: { title: string; severity: string; anchor_event_id: number }) => request<IncidentSnapshot>("/api/v1/incidents", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
  }),
  devices: () => request<ListEnvelope<Device>>("/api/v1/devices"),
  captures: () => request<ListEnvelope<Capture>>("/api/v1/captures"),
  createCapture: (input: { device_id: string; device_name: string; full_logs: boolean }) => request<Capture>("/api/v1/captures", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
  }),
  artifacts: () => request<ListEnvelope<ArtifactRecord>>("/api/v1/artifacts"),
}
