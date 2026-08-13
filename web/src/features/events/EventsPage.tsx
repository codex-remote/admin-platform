import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Check, ChevronRight, Copy, FilterX, Pause, Play, Search, X } from "lucide-react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { api } from "../../api/client"
import { EmptyState, ErrorState, Level, LoadingState, formatTime } from "../../components/ui"
import type { EventRecord, FacetValue, HistogramBucket } from "../../types"

const facetLabels = { source: "来源", profile: "环境", level: "级别", category: "分类", event: "事件" } as const
type FacetKey = keyof typeof facetLabels

function queryParams(searchParams: URLSearchParams) {
  const params = new URLSearchParams(searchParams)
  if (!params.has("from") && !params.has("to")) {
    const minutes = Math.min(1440, Math.max(1, Number(params.get("range") ?? 30) || 30))
    const to = new Date()
    params.set("to", to.toISOString())
    params.set("from", new Date(to.getTime() - minutes * 60_000).toISOString())
  }
  params.delete("range")
  params.set("limit", "200")
  return params
}

export function EventsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const queryKey = searchParams.toString()
  const [selected, setSelected] = useState<EventRecord | null>(null)
  const [paused, setPaused] = useState(false)
  const events = useQuery({ queryKey: ["events", queryKey, paused], queryFn: () => api.events(queryParams(searchParams)), enabled: !paused, placeholderData: previous => previous })
  const facetParams = () => { const value = queryParams(searchParams); value.delete("cursor"); value.delete("limit"); return value }
  const facets = useQuery({ queryKey: ["facets", queryKey], queryFn: () => api.facets(facetParams()), enabled: !paused, placeholderData: previous => previous })
  const histogram = useQuery({ queryKey: ["histogram", queryKey], queryFn: () => api.histogram(facetParams()), enabled: !paused, placeholderData: previous => previous })

  const setFilter = (key: FacetKey, value: string) => {
    const next = new URLSearchParams(searchParams)
    const values = new Set((next.get(key) ?? "").split(",").filter(Boolean))
    values.has(value) ? values.delete(value) : values.add(value)
    if (values.size) next.set(key, [...values].join(",")); else next.delete(key)
    next.delete("cursor")
    setSearchParams(next)
  }
  const setRange = (minutes: number) => {
    const next = new URLSearchParams(searchParams)
    next.set("range", String(minutes)); next.delete("from"); next.delete("to"); next.delete("cursor"); setSearchParams(next)
  }
  const search = (value: string) => { const next = new URLSearchParams(searchParams); value ? next.set("q", value) : next.delete("q"); next.delete("cursor"); setSearchParams(next, { replace: true }) }

  return <div className="events-layout">
    <aside className="facet-rail">
      <div className="facet-title"><span>筛选维度</span><button className="icon-button" onClick={() => setSearchParams({})} title="清除筛选"><FilterX size={15} /></button></div>
      {facets.isLoading && <LoadingState label="读取维度" />}
      {facets.error && <ErrorState error={facets.error} />}
      {facets.data && (Object.keys(facetLabels) as FacetKey[]).map(key => <FacetGroup key={key} label={facetLabels[key]} values={facets.data.facets[key]} selected={(searchParams.get(key) ?? "").split(",")} toggle={value => setFilter(key, value)} />)}
    </aside>
    <section className="events-workspace">
      <div className="query-toolbar">
        <label className="query-input"><Search size={16} /><input value={searchParams.get("q") ?? ""} onChange={event => search(event.target.value)} placeholder="搜索事件、消息或字段" /></label>
        <div className="range-control">{[[15,"15m"],[30,"30m"],[60,"1h"],[360,"6h"],[1440,"24h"]].map(([minutes,label]) => <button key={label} onClick={() => setRange(Number(minutes))}>{label}</button>)}</div>
        <button className={`tool-button ${paused ? "is-paused" : ""}`} onClick={() => setPaused(value => !value)}>{paused ? <Play size={15} /> : <Pause size={15} />}{paused ? "恢复" : "暂停"}</button>
      </div>
      {histogram.data && <Histogram buckets={histogram.data.buckets} />}
      <div className="event-table-wrap">
        <table className="data-table event-table"><thead><tr><th>时间</th><th>级别</th><th>来源</th><th>环境</th><th>事件</th><th>关联</th><th /></tr></thead>
          <tbody>{events.data?.items.map(item => <tr key={item.id} className={selected?.id === item.id ? "selected" : ""} onClick={() => setSelected(item)} tabIndex={0} onKeyDown={event => event.key === "Enter" && setSelected(item)}>
            <td className="mono time-cell">{formatTime(item.timestamp)}</td><td><Level value={item.level} /></td><td>{item.source}</td><td className="muted">{item.profile || "-"}</td><td><strong>{item.event}</strong><small>{item.message}</small></td><td className="mono muted">{item.trace_id || item.session_id || item.turn_ref || "-"}</td><td><ChevronRight size={15} /></td>
          </tr>)}</tbody>
        </table>
        {events.isLoading && <LoadingState label="读取事件" />}
        {events.error && <ErrorState error={events.error} retry={() => events.refetch()} />}
        {events.data?.items.length === 0 && <EmptyState>当前时间范围没有匹配事件</EmptyState>}
      </div>
      <div className="table-footer"><span>{events.data?.items.length ?? 0} 条 · {events.data?.meta.query_time_ms ?? 0} ms · UTC</span>{events.data?.next_cursor && <button className="secondary-button" onClick={() => { const next = new URLSearchParams(searchParams); next.set("cursor", events.data!.next_cursor!); setSearchParams(next) }}>继续加载</button>}</div>
    </section>
    {selected && <EventDetail event={selected} close={() => setSelected(null)} />}
  </div>
}

function FacetGroup({ label, values, selected, toggle }: { label: string; values: FacetValue[]; selected: string[]; toggle: (value: string) => void }) {
  return <div className="facet-group"><h3>{label}</h3>{values.slice(0, 12).map(value => <button key={value.value} className={selected.includes(value.value) ? "selected" : ""} onClick={() => toggle(value.value)}><span>{selected.includes(value.value) && <Check size={12} />}{value.value}</span><small>{value.count}</small></button>)}</div>
}

function Histogram({ buckets }: { buckets: HistogramBucket[] }) {
  const max = Math.max(1, ...buckets.map(item => item.count))
  return <div className="histogram" aria-label="事件趋势">{buckets.length === 0 ? <span className="histogram-empty">时间窗口内无事件</span> : buckets.map(bucket => <div className="histogram-column" key={bucket.start} title={`${formatTime(bucket.start)} · ${bucket.count} 条`}><i className="hist-all" style={{ height: `${Math.max(3, bucket.count/max*100)}%` }} /><i className="hist-warning" style={{ height: `${bucket.warnings/max*100}%` }} /><i className="hist-error" style={{ height: `${bucket.errors/max*100}%` }} /></div>)}</div>
}

function EventDetail({ event, close }: { event: EventRecord; close: () => void }) {
  const navigate = useNavigate(); const queryClient = useQueryClient(); const [copied, setCopied] = useState(false)
  const create = useMutation({ mutationFn: () => api.createIncident({ title: `${event.source}: ${event.event}`, severity: event.level === "error" || event.level === "fault" ? "critical" : "warning", anchor_event_id: event.id }), onSuccess: snapshot => { queryClient.invalidateQueries({ queryKey: ["incidents"] }); navigate(`/incidents/${snapshot.incident.id}`) } })
  const copy = async () => { await navigator.clipboard.writeText(JSON.stringify(event, null, 2)); setCopied(true); setTimeout(() => setCopied(false), 1200) }
  return <aside className="detail-drawer" aria-label="事件详情"><div className="drawer-heading"><div><p className="eyebrow">EVENT #{event.id}</p><h2>{event.event}</h2></div><button className="icon-button" onClick={close} title="关闭"><X size={18} /></button></div>
    <div className="detail-actions"><button className="primary-button" onClick={() => create.mutate()} disabled={create.isPending}>建立事故</button><button className="icon-button bordered" onClick={copy} title="复制 JSON">{copied ? <Check size={16} /> : <Copy size={16} />}</button></div>
    {create.error && <ErrorState error={create.error} />}
    <dl className="detail-list"><div><dt>时间</dt><dd className="mono">{event.timestamp}</dd></div><div><dt>来源</dt><dd>{event.source} / {event.profile || "default"}</dd></div><div><dt>级别</dt><dd><Level value={event.level} /></dd></div><div><dt>分类</dt><dd>{event.category}</dd></div>{event.session_id && <div><dt>Session</dt><dd className="mono">{event.session_id}</dd></div>}{event.trace_id && <div><dt>Trace</dt><dd className="mono">{event.trace_id}</dd></div>}</dl>
    <div className="raw-json"><div className="raw-heading">结构化字段</div><pre>{JSON.stringify(event.fields, null, 2)}</pre></div>
  </aside>
}
