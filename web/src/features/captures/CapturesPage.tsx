import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { HardDriveDownload, Smartphone } from "lucide-react"
import { api } from "../../api/client"
import { EmptyState, ErrorState, LoadingState, StatusDot, formatTime } from "../../components/ui"

export function CapturesPage() {
  const queryClient = useQueryClient(); const [fullLogs, setFullLogs] = useState(false)
  const devices = useQuery({ queryKey: ["devices"], queryFn: api.devices })
  const captures = useQuery({ queryKey: ["captures"], queryFn: api.captures })
  const create = useMutation({ mutationFn: (device: { id: string; name: string }) => api.createCapture({ device_id: device.id, device_name: device.name, full_logs: fullLogs }), onSuccess: () => queryClient.invalidateQueries({ queryKey: ["captures"] }) })
  if (devices.isLoading || captures.isLoading) return <LoadingState />
  if (devices.error) return <ErrorState error={devices.error} retry={() => devices.refetch()} />
  if (captures.error) return <ErrorState error={captures.error} retry={() => captures.refetch()} />
  return <div className="page-stack"><section className="capture-console"><div className="section-heading"><div><p className="eyebrow">ALLOWLISTED COREDEVICE</p><h2>连接设备</h2></div><Smartphone size={19} /></div>
    <div className="device-grid">{devices.data!.items.map(device => <div className="device-item" key={device.id}><div className="device-icon"><Smartphone size={20} /></div><div><strong>{device.name || device.marketing_name}</strong><span>{device.marketing_name} · iOS {device.os_version}</span><small>{device.connected ? "已连接" : "未连接"} · {device.paired ? "已配对" : "未配对"}</small></div><button className="primary-button" disabled={!device.connected || create.isPending} onClick={() => create.mutate({ id: device.id, name: device.name || device.marketing_name })}><HardDriveDownload size={15} />采集</button></div>)}</div>
    {devices.data!.items.length === 0 && <EmptyState>Collector 尚未发现已连接设备</EmptyState>}
    <label className="check-control"><input type="checkbox" checked={fullLogs} onChange={event => setFullLogs(event.target.checked)} /><span>包含完整系统日志</span><small>会产生更大的 sysdiagnose 文件，并需要更长时间</small></label>{create.error && <ErrorState error={create.error} />}
  </section>
  <section className="panel table-panel"><div className="section-heading"><div><p className="eyebrow">CAPTURE QUEUE</p><h2>采集任务</h2></div></div><table className="data-table"><thead><tr><th>状态</th><th>设备</th><th>创建时间</th><th>范围</th><th>结果</th></tr></thead><tbody>{captures.data!.items.map(item => <tr key={item.id}><td><StatusDot status={item.status} /> {item.status}</td><td><strong>{item.device_name}</strong><small className="mono">{item.device_id}</small></td><td className="mono">{formatTime(item.created_at)}</td><td>{item.full_logs ? "完整系统日志" : "标准诊断"}</td><td>{item.error || (item.artifact_id ? `证据 #${item.artifact_id}` : "-")}</td></tr>)}</tbody></table>{captures.data!.items.length === 0 && <EmptyState>暂无采集任务</EmptyState>}</section></div>
}
