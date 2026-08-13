import type { ReactNode } from "react"
import { AlertTriangle, LoaderCircle, RefreshCw } from "lucide-react"

export function LoadingState({ label = "正在读取数据" }: { label?: string }) {
  return <div className="state-message"><LoaderCircle className="spin" size={18} /><span>{label}</span></div>
}

export function ErrorState({ error, retry }: { error: Error; retry?: () => void }) {
  return <div className="state-message state-error"><AlertTriangle size={18} /><span>{error.message}</span>{retry && <button className="icon-button" onClick={retry} title="重试"><RefreshCw size={16} /></button>}</div>
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <div className="empty-state">{children}</div>
}

export function StatusDot({ status }: { status: string }) {
  return <span className={`status-dot status-${status}`} aria-label={status} />
}

export function Level({ value }: { value: string }) {
  return <span className={`level level-${value}`}>{value}</span>
}

export function formatTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(new Date(value))
}

export function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(1)} GB`
}
