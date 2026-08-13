import { useQuery } from "@tanstack/react-query"
import { ArrowUpRight, Database, Radio, ShieldCheck } from "lucide-react"
import { Link } from "react-router-dom"
import { api } from "../../api/client"
import { ErrorState, LoadingState, StatusDot } from "../../components/ui"

export function OverviewPage() {
  const overview = useQuery({ queryKey: ["overview"], queryFn: api.overview })
  const services = useQuery({ queryKey: ["services"], queryFn: api.services })
  if (overview.isLoading || services.isLoading) return <LoadingState />
  if (overview.error) return <ErrorState error={overview.error} retry={() => overview.refetch()} />
  if (services.error) return <ErrorState error={services.error} retry={() => services.refetch()} />
  const value = overview.data!
  const serviceItems = services.data!.items
  const online = serviceItems.filter(item => item.status === "online").length
  const maxSource = Math.max(1, ...Object.values(value.by_source))
  return <div className="page-stack">
    <section className="summary-strip">
      <Link to="/events" className="summary-cell"><span>事件总量</span><strong>{value.total_events.toLocaleString()}</strong><ArrowUpRight size={15} /></Link>
      <Link to="/events?level=error,fault" className="summary-cell critical"><span>错误</span><strong>{value.errors}</strong><ArrowUpRight size={15} /></Link>
      <Link to="/events?level=warning" className="summary-cell warning"><span>警告</span><strong>{value.warnings}</strong><ArrowUpRight size={15} /></Link>
      <Link to="/services" className="summary-cell"><span>在线服务</span><strong>{online}/{serviceItems.length}</strong><ArrowUpRight size={15} /></Link>
      <Link to="/captures" className="summary-cell"><span>执行中采集</span><strong>{value.active_captures}</strong><ArrowUpRight size={15} /></Link>
    </section>
    <div className="overview-grid">
      <section className="panel source-panel"><div className="section-heading"><div><p className="eyebrow">INGEST DISTRIBUTION</p><h2>数据来源</h2></div><Database size={18} /></div>
        <div className="source-bars">{Object.entries(value.by_source).sort((a,b) => b[1]-a[1]).map(([source, count]) => <Link to={`/events?source=${encodeURIComponent(source)}`} className="source-row" key={source}>
          <div><strong>{source}</strong><span>{count.toLocaleString()} 条</span></div><div className="bar-track"><i style={{ width: `${Math.max(3, count / maxSource * 100)}%` }} /></div>
        </Link>)}</div>
      </section>
      <section className="panel"><div className="section-heading"><div><p className="eyebrow">RUNTIME MATRIX</p><h2>服务矩阵</h2></div><Radio size={18} /></div>
        <div className="service-compact">{serviceItems.slice(0, 8).map(service => <div className="compact-row" key={`${service.source}-${service.profile}`}><StatusDot status={service.status} /><strong>{service.name}</strong><span>{service.profile}</span><small>{service.latency_ms} ms</small></div>)}</div>
      </section>
    </div>
    <section className="evidence-band"><ShieldCheck size={20} /><div><strong>证据链处于本地受控范围</strong><span>{value.artifacts} 个附件，数据源 {value.meta.data_source}，数据库时区 {value.meta.timezone}</span></div><Link to="/artifacts">查看证据</Link></section>
  </div>
}
