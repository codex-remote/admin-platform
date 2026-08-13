import { useQuery } from "@tanstack/react-query"
import { Download, FileWarning, X } from "lucide-react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { api } from "../../api/client"
import { formatTime } from "../../components/format"
import { EmptyState, ErrorState, Level, LoadingState } from "../../components/ui"

export function IncidentsPage() {
  const { incidentId } = useParams(); const navigate = useNavigate()
  const list = useQuery({ queryKey: ["incidents"], queryFn: api.incidents })
  const selected = useQuery({ queryKey: ["incident", incidentId], queryFn: () => api.incident(Number(incidentId)), enabled: Boolean(incidentId) })
  if (list.isLoading) return <LoadingState />
  if (list.error) return <ErrorState error={list.error} retry={() => list.refetch()} />
  return <div className="split-page">
    <section className="list-pane"><div className="pane-heading"><div><p className="eyebrow">IMMUTABLE EVIDENCE</p><h2>事故记录</h2></div><span>{list.data!.items.length}</span></div>
      <div className="incident-list">{list.data!.items.map(item => <Link to={`/incidents/${item.id}`} key={item.id} className={`incident-item ${Number(incidentId) === item.id ? "active" : ""}`}><div><Level value={item.severity} /><time>{formatTime(item.created_at)}</time></div><strong>{item.title}</strong><span>{item.event_count} 个事件 · {item.artifact_count} 个附件</span></Link>)}</div>
      {list.data!.items.length === 0 && <EmptyState>还没有事故记录。请从事件详情建立事故。</EmptyState>}
    </section>
    <section className="detail-pane">{!incidentId && <div className="selection-empty"><FileWarning size={28} /><strong>选择一条事故查看固定证据</strong></div>}
      {selected.isLoading && <LoadingState />}{selected.error && <ErrorState error={selected.error} />}{selected.data && <>
        <div className="incident-header"><div><p className="eyebrow">INCIDENT #{selected.data.incident.id}</p><h2>{selected.data.incident.title}</h2><span>{selected.data.incident.window_start} — {selected.data.incident.window_end}</span></div><div className="header-actions"><a className="icon-button bordered" href={`/api/v1/incidents/${selected.data.incident.id}/snapshot`} title="下载快照"><Download size={17} /></a><button className="icon-button" onClick={() => navigate("/incidents")} title="关闭"><X size={18} /></button></div></div>
        <div className="provenance-strip"><span>Schema {selected.data.schema_version}</span><span>{selected.data.provenance.data_source}</span><span>{selected.data.provenance.database_timezone}</span><span>{selected.data.provenance.truncated ? "已截断" : "完整窗口"}</span></div>
        <div className="timeline">{selected.data.events.map(event => <div className="timeline-event" key={event.id}><i /><time>{formatTime(event.timestamp)}</time><Level value={event.level} /><div><strong>{event.event}</strong><span>{event.source} / {event.profile || "default"}</span></div></div>)}</div>
      </>}</section>
  </div>
}
