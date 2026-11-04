type ChartLine = {
  name: string
  color: string
  values: Array<number | null>
}

export function SeriesChart({
  label,
  lines,
  formatValue,
}: {
  label: string
  lines: ChartLine[]
  formatValue: (value: number) => string
}) {
  const width = 640
  const height = 168
  const padLeft = 8
  const padRight = 8
  const padTop = 16
  const padBottom = 12
  const count = lines[0]?.values.length ?? 0
  let min = Number.POSITIVE_INFINITY
  let max = Number.NEGATIVE_INFINITY
  for (const line of lines) {
    for (const value of line.values) {
      if (value == null || Number.isNaN(value)) continue
      min = Math.min(min, value)
      max = Math.max(max, value)
    }
  }
  if (!Number.isFinite(min) || count === 0) {
    return null
  }
  if (min === max) {
    const span = Math.max(Math.abs(max) * 0.2, 1)
    min -= span
    max += span
  } else {
    const pad = (max - min) * 0.1
    min -= pad
    max += pad
  }
  const innerWidth = width - padLeft - padRight
  const innerHeight = height - padTop - padBottom
  const xAt = (index: number) =>
    count === 1 ? padLeft + innerWidth / 2 : padLeft + (index / (count - 1)) * innerWidth
  const yAt = (value: number) => padTop + ((max - value) / (max - min)) * innerHeight

  const summary = lines
    .map((line) => {
      const latest = lastNumber(line.values)
      return latest == null ? `${line.name} has no points` : `${line.name} latest ${formatValue(latest)}`
    })
    .join(". ")

  return (
    <div>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={`${label}. ${summary}`}
        className="mt-3 h-40 w-full"
      >
        <line
          x1={padLeft}
          x2={width - padRight}
          y1={yAt(max)}
          y2={yAt(max)}
          stroke="var(--color-line)"
        />
        <line
          x1={padLeft}
          x2={width - padRight}
          y1={yAt((min + max) / 2)}
          y2={yAt((min + max) / 2)}
          stroke="var(--color-line)"
        />
        <text x={padLeft} y={12} fill="var(--color-muted)" fontSize="10" fontFamily="ui-monospace, monospace">
          {formatValue(max)}
        </text>
        {lines.map((line) => (
          <g key={line.name}>
            {runs(line.values).map((run) =>
              run.indexes.length === 1 ? (
                <circle
                  key={`${line.name}-${run.indexes[0]}`}
                  cx={xAt(run.indexes[0])}
                  cy={yAt(run.values[0])}
                  r={3}
                  fill={line.color}
                />
              ) : (
                <path
                  key={`${line.name}-${run.indexes[0]}`}
                  d={run.indexes
                    .map((index, position) => {
                      const command = position === 0 ? "M" : "L"
                      return `${command}${xAt(index).toFixed(2)} ${yAt(run.values[position]).toFixed(2)}`
                    })
                    .join(" ")}
                  fill="none"
                  stroke={line.color}
                  strokeWidth={1.75}
                  strokeLinejoin="round"
                  strokeLinecap="round"
                />
              ),
            )}
          </g>
        ))}
      </svg>
      {lines.length > 1 ? (
        <ul className="mt-2 flex flex-wrap gap-4">
          {lines.map((line) => (
            <li key={line.name} className="flex items-center gap-2 font-mono text-[10px] tracking-[0.12em] text-muted uppercase">
              <span className="inline-block h-1.5 w-4" style={{ background: line.color }} aria-hidden="true" />
              {line.name}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

function lastNumber(values: Array<number | null>) {
  for (let index = values.length - 1; index >= 0; index -= 1) {
    const value = values[index]
    if (value != null && !Number.isNaN(value)) return value
  }
  return null
}

function runs(values: Array<number | null>) {
  const groups: Array<{ indexes: number[]; values: number[] }> = []
  let current: { indexes: number[]; values: number[] } | null = null
  values.forEach((value, index) => {
    if (value == null || Number.isNaN(value)) {
      if (current) groups.push(current)
      current = null
      return
    }
    if (!current) current = { indexes: [], values: [] }
    current.indexes.push(index)
    current.values.push(value)
  })
  if (current) groups.push(current)
  return groups
}
