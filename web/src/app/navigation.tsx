import type { LucideIcon } from "lucide-react"
import { Activity, Archive, Gauge, Server, Sparkles, Telescope, UserRound, UsersRound } from "lucide-react"

type AvailableCapability = { id: string; label: string; status: "available"; route: string; icon: LucideIcon }
type PlannedCapability = { id: string; label: string; status: "planned"; icon: LucideIcon }
export type VisibleCapability = AvailableCapability | PlannedCapability

export const navigationGroups: { label: string; items: VisibleCapability[] }[] = [
  { label: "可观测性", items: [
    { id: "overview", label: "总览", status: "available", route: "/overview", icon: Gauge },
    { id: "events", label: "事件", status: "available", route: "/events", icon: Activity },
    { id: "incidents", label: "事故", status: "available", route: "/incidents", icon: Telescope },
  ] },
  { label: "采集与证据", items: [
    { id: "captures", label: "采集", status: "available", route: "/captures", icon: Sparkles },
    { id: "artifacts", label: "证据库", status: "available", route: "/artifacts", icon: Archive },
  ] },
  { label: "系统", items: [
    { id: "services", label: "服务", status: "available", route: "/services", icon: Server },
  ] },
  { label: "产品", items: [
    { id: "users", label: "用户", status: "planned", icon: UserRound },
    { id: "behavior", label: "用户行为", status: "planned", icon: UsersRound },
  ] },
]
