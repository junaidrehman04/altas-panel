import type { ComponentProps } from "react"
import { cn } from "../../lib/utils"

export function Input({ className, ...props }: ComponentProps<"input">) {
  return (
    <input
      className={cn(
        "flex h-10 w-full border border-line bg-base px-3 text-sm text-ink placeholder:text-muted",
        className,
      )}
      {...props}
    />
  )
}
