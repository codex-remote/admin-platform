import { useEffect, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { NavLink, Navigate, Outlet, Route, Routes, useLocation } from "react-router-dom"
import { navigationGroups } from "./navigation"
import { ArtifactsPage } from "../features/artifacts/ArtifactsPage"
import { CapturesPage } from "../features/captures/CapturesPage"
import { EventsPage } from "../features/events/EventsPage"
import { IncidentsPage } from "../features/incidents/IncidentsPage"
import { OverviewPage } from "../features/overview/OverviewPage"
import { ServicesPage } from "../features/services/ServicesPage"

const titles: Record<string, string> = {
  overview: "运行总览", events: "事件探索", incidents: "事故", captures: "设备采集", artifacts: "证据库", services: "服务状态",
}

function Shell() {
  const [collapsed, setCollapsed] = useState(false)
  const [live, setLive] = useState<"connecting" | "live" | "offline">("connecting")
  const location = useLocation()
  const queryClient = useQueryClient()
  const segment = location.pathname.split("/")[1] || "overview"

  useEffect(() => {
    const stream = new EventSource("/api/v1/stream")
    stream.addEventListener("ready", () => setLive("live"))
    stream.addEventListener("changed", () => {
      for (const key of ["overview", "events", "facets", "histogram"] as const) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    })
    stream.onerror = () => setLive("offline")
    return () => stream.close()
  }, [queryClient])

  return <div className={`app-shell ${collapsed ? "is-collapsed" : ""}`}>
    <aside className="sidebar">
      <div className="brand-mark"><span className="brand-signal" aria-hidden="true"><img src="/brand-mark.png" alt="" /></span><div className="brand-copy"><strong>CodexRemote</strong><small>Admin</small></div></div>
      <nav aria-label="主导航">
        {navigationGroups.map(group => <div className="nav-group" key={group.label}>
          <div className="nav-group-label">{group.label}</div>
          {group.items.map(item => {
            const Icon = item.icon
            if (item.status === "planned") return <div key={item.id} className="nav-item nav-planned" tabIndex={0} aria-label={`${item.label}，规划中`} title="此模块尚未开放">
              <Icon size={17} /><span className="nav-label">{item.label}</span><small className="planned-label">规划中</small>
            </div>
            return <NavLink key={item.id} to={item.route} className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`} title={item.label}>
              <Icon size={17} /><span className="nav-label">{item.label}</span>
            </NavLink>
          })}
        </div>)}
      </nav>
      <button className="collapse-button" onClick={() => setCollapsed(value => !value)} title={collapsed ? "展开侧栏" : "收起侧栏"} aria-label={collapsed ? "展开侧栏" : "收起侧栏"}>
        {collapsed ? <ChevronRight size={17} /> : <ChevronLeft size={17} />}
      </button>
    </aside>
    <section className="workbench">
      <header className="context-bar">
        <div><p className="eyebrow">LOCAL / POSTGRESQL</p><h1>{titles[segment] ?? "CodexRemote Admin"}</h1></div>
        <div className="context-actions">
          <span className={`live-status live-${live}`}><i />{live === "live" ? "实时" : live === "offline" ? "已断开" : "连接中"}</span>
        </div>
      </header>
      <main className="content"><Outlet /></main>
    </section>
  </div>
}

export function App() {
  return <Routes>
    <Route element={<Shell />}>
      <Route index element={<Navigate to="/overview" replace />} />
      <Route path="overview" element={<OverviewPage />} />
      <Route path="events" element={<EventsPage />} />
      <Route path="incidents" element={<IncidentsPage />} />
      <Route path="incidents/:incidentId" element={<IncidentsPage />} />
      <Route path="captures" element={<CapturesPage />} />
      <Route path="artifacts" element={<ArtifactsPage />} />
      <Route path="services" element={<ServicesPage />} />
      <Route path="*" element={<Navigate to="/overview" replace />} />
    </Route>
  </Routes>
}
