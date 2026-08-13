import { useQuery } from "@tanstack/react-query"
import { Activity, Clock, RadioTower } from "lucide-react"
import { api } from "../../api/client"
import { formatTime } from "../../components/format"
import { EmptyState, ErrorState, LoadingState, StatusDot } from "../../components/ui"

export function ServicesPage() {
  const services = useQuery({ queryKey: ["services"], queryFn: api.services, refetchInterval: 10_000 })
  if (services.isLoading) return <LoadingState />
  if (services.error) return <ErrorState error={services.error} retry={() => services.refetch()} />
  return <div className="service-page"><section className="service-matrix"><div className="matrix-header"><span>组件</span><span>环境</span><span>状态</span><span>延迟</span><span>运行信息</span></div>{services.data!.items.map(service => <div className="matrix-row" key={`${service.source}-${service.profile}`}><div><span className="service-symbol"><RadioTower size={17} /></span><strong>{service.name}</strong><small>{service.source}</small></div><span className="mono">{service.profile}</span><span><StatusDot status={service.status} /> {service.status}</span><span className="mono"><Clock size={13} /> {service.latency_ms} ms</span><span className="service-detail">{service.detail?.updated_at ? `更新于 ${formatTime(String(service.detail.updated_at))}` : service.detail?.pending_batches !== undefined ? `积压 ${String(service.detail.pending_batches)}` : service.url}</span></div>)}</section>
    {services.data!.items.length === 0 && <EmptyState>没有服务状态</EmptyState>}<div className="service-footnote"><Activity size={16} />每 10 秒主动检查一次；实时事件通过独立 SSE 通道通知。</div>
  </div>
}
