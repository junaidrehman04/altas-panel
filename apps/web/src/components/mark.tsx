import { cn } from "../lib/utils"

export function Mark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("h-8 w-8", className)} aria-hidden="true">
      <rect x="0.5" y="0.5" width="31" height="31" fill="none" className="stroke-line" strokeWidth="1" />
      <path d="M6 25 L16 6 L26 25" fill="none" className="stroke-live" strokeWidth="2" />
      <path d="M10.5 18.5 H21.5" className="stroke-signal" strokeWidth="2" />
      <rect x="14.5" y="22" width="3" height="3" className="fill-signal" />
    </svg>
  )
}

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <span className="flex items-center gap-2.5">
      <Mark />
      <span className="text-[17px] font-semibold tracking-tight">Altas</span>
      {compact ? null : (
        <span className="hidden border border-line px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline">
          PANEL v0.1.0
        </span>
      )}
    </span>
  )
}
