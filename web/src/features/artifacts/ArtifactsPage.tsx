import { useQuery } from "@tanstack/react-query"
import { Download, Fingerprint } from "lucide-react"
import { api } from "../../api/client"
import { EmptyState, ErrorState, LoadingState, formatBytes, formatTime } from "../../components/ui"

export function ArtifactsPage() {
  const artifacts = useQuery({ queryKey: ["artifacts"], queryFn: api.artifacts })
  if (artifacts.isLoading) return <LoadingState />
  if (artifacts.error) return <ErrorState error={artifacts.error} retry={() => artifacts.refetch()} />
  return <section className="panel table-panel full-height"><div className="section-heading"><div><p className="eyebrow">VERIFIED ARTIFACTS</p><h2>诊断证据</h2></div><span className="count-label">{artifacts.data!.items.length} 项</span></div>
    <table className="data-table"><thead><tr><th>创建时间</th><th>名称</th><th>类型</th><th>来源</th><th>大小</th><th>SHA-256</th><th /></tr></thead><tbody>{artifacts.data!.items.map(item => <tr key={item.id}><td className="mono">{formatTime(item.created_at)}</td><td><strong>{item.name}</strong></td><td>{item.kind}</td><td>{item.source}</td><td>{formatBytes(item.size_bytes)}</td><td className="hash-cell"><Fingerprint size={14} /><span className="mono">{item.sha256}</span></td><td><a className="icon-button" href={`/api/v1/artifacts/${item.id}`} title="下载"><Download size={16} /></a></td></tr>)}</tbody></table>{artifacts.data!.items.length === 0 && <EmptyState>还没有诊断附件</EmptyState>}
  </section>
}
