export type Range = "1h" | "6h" | "24h" | "7d" | "30d"
export type Label = { axis: string; full: string }

export const RANGE_OPTIONS: { value: Range; label: string }[] = [
  { value: "1h", label: "1H" },
  { value: "6h", label: "6H" },
  { value: "24h", label: "24H" },
  { value: "7d", label: "7D" },
  { value: "30d", label: "30D" },
]

export const fmt = (v: number, d = 1) =>
  v.toLocaleString("en-US", { maximumFractionDigits: d, minimumFractionDigits: d })
export const fmtInt = (v: number) => Math.round(v).toLocaleString("en-US")
export const compact = (v: number) =>
  v >= 1e6 ? `${fmt(v / 1e6, 1)}M` : v >= 1e4 ? `${fmt(v / 1e3, 1)}k` : fmtInt(v)

export function gb(bytes: number, digits = 1) {
  return fmt(bytes / (1024 * 1024 * 1024), digits)
}
