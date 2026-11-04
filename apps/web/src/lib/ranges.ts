export const rangeOptions = [
  { id: "live", label: "Live" },
  { id: "24h", label: "24h" },
  { id: "7d", label: "7d" },
  { id: "30d", label: "30d" },
  { id: "month", label: "Months" },
] as const

export type RangeId = (typeof rangeOptions)[number]["id"]
