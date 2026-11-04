import { rangeOptions, type RangeId } from "../lib/ranges"
import { cn } from "../lib/utils"

export function RangePicker({
  value,
  onChange,
}: {
  value: RangeId
  onChange: (range: RangeId) => void
}) {
  return (
    <div role="group" aria-label="Time range" className="flex flex-wrap gap-px border border-line bg-line">
      {rangeOptions.map((option) => {
        const selected = value === option.id
        return (
          <button
            key={option.id}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(option.id)}
            className={cn(
              "px-3 py-2 font-mono text-[11px] tracking-[0.14em] uppercase",
              selected ? "bg-base text-ink shadow-[inset_0_-2px_0_var(--color-accent)]" : "bg-card text-muted hover:text-ink",
            )}
          >
            {option.label}
          </button>
        )
      })}
    </div>
  )
}
