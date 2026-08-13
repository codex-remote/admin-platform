import { describe, expect, it } from "vitest"
import { navigationGroups } from "./navigation"

describe("capability navigation", () => {
  it("exposes six available and two planned capabilities", () => {
    const items = navigationGroups.flatMap(group => group.items)
    expect(items.filter(item => item.status === "available")).toHaveLength(6)
    expect(items.filter(item => item.status === "planned")).toHaveLength(2)
  })

  it("never assigns routes to planned capabilities", () => {
    const planned = navigationGroups.flatMap(group => group.items).filter(item => item.status === "planned")
    expect(planned.map(item => item.id)).toEqual(["users", "behavior"])
    expect(planned.every(item => !("route" in item))).toBe(true)
  })
})
