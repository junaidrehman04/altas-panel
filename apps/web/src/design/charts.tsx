import { useLayoutEffect, useRef, useState, type MouseEvent, type ReactNode } from 'react';
import type { Label } from './data';

export function useWidth<T extends HTMLElement>(initial = 600) {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(initial);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(el.clientWidth || initial);
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w) setWidth(w);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [initial]);
  return [ref, width] as const;
}

export type Series = {
  key: string;
  label: string;
  color: string;
  values: (number | null)[];
  dashed?: boolean;
  /** Draw as a plain line above the stack instead of contributing to it. */
  overlay?: boolean;
  hideInLegend?: boolean;
};
export type Threshold = { value: number; label: string; tone: 'warning' | 'critical' | 'neutral' };
export type Annotation = { index: number; label: string };

export function niceMax(v: number) {
  if (v <= 0) return 1;
  const p = 10 ** Math.floor(Math.log10(v));
  const n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 4 ? 4 : n <= 5 ? 5 : n <= 8 ? 8 : 10) * p;
}

function linePath(vals: (number | null)[], x: (i: number) => number, y: (v: number) => number) {
  let d = '';
  let pen = false;
  vals.forEach((v, i) => {
    if (v == null) { pen = false; return; }
    d += `${pen ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`;
    pen = true;
  });
  return d;
}

function bandPath(upper: (number | null)[], lower: (number | null)[], x: (i: number) => number, y: (v: number) => number) {
  const idx = upper.map((v, i) => (v != null && lower[i] != null ? i : -1)).filter((i) => i >= 0);
  if (!idx.length) return '';
  const top = idx.map((i, k) => `${k ? 'L' : 'M'}${x(i).toFixed(1)},${y(upper[i] as number).toFixed(1)}`).join('');
  const bottom = [...idx].reverse().map((i) => `L${x(i).toFixed(1)},${y(lower[i] as number).toFixed(1)}`).join('');
  return `${top}${bottom}Z`;
}

export function TimeSeries({
  series, labels, format = (v) => String(Math.round(v)), unit = '', height = 180, stacked = false, thresholds = [], annotations = [], yMax, band, gaps = [], legend = true, legendExtra,
}: {
  series: Series[]; labels: Label[]; format?: (v: number) => string; unit?: string; height?: number; stacked?: boolean; thresholds?: Threshold[]; annotations?: Annotation[]; yMax?: number;
  band?: { upper: (number | null)[]; lower: (number | null)[]; label: string }; gaps?: [number, number][]; legend?: boolean; legendExtra?: ReactNode;
}) {
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const [drag, setDrag] = useState<number | null>(null);
  const [sel, setSel] = useState<[number, number] | null>(null);
  const n = labels.length;
  const pl = 46, pr = 10, pt = 12, pb = 22;
  const iw = Math.max(10, w - pl - pr), ih = height - pt - pb;

  const plotted: (number | null)[][] = [];
  series.forEach((s, k) => {
    if (!stacked || s.overlay) { plotted[k] = s.values; return; }
    let prev = -1;
    for (let j = k - 1; j >= 0; j--) if (!series[j].overlay) { prev = j; break; }
    plotted[k] = s.values.map((v, i) => (v == null ? null : v + (prev >= 0 ? plotted[prev][i] ?? 0 : 0)));
  });
  const all = [...plotted.flat(), ...(band?.upper ?? [])].filter((v): v is number => v != null);
  const max = yMax ?? niceMax(Math.max(1, ...all, ...thresholds.map((t) => t.value)) * 1.06);
  const x = (i: number) => pl + (i / (n - 1)) * iw;
  const y = (v: number) => pt + ih - (Math.min(v, max) / max) * ih;
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((f) => f * max);
  const xStep = Math.max(1, Math.round((n - 1) / Math.max(2, Math.min(6, Math.floor(iw / 90)))));

  const idxAt = (e: MouseEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    return Math.max(0, Math.min(n - 1, Math.round(((e.clientX - r.left - pl) / iw) * (n - 1))));
  };
  const firstStack = series.findIndex((s) => !s.overlay);

  const tipRight = hover != null && x(hover) > w * 0.6;
  return (
    <div className="ts">
      {legend && (
        <div className="legend">
          {series.filter((s) => !s.hideInLegend).map((s) => (
            <span key={s.key} className="legend-item">
              <i className={s.dashed ? 'dashed' : ''} style={{ background: s.dashed ? undefined : s.color, borderColor: s.color }} />
              {s.label}
              <b>{format(hover != null ? s.values[hover] ?? 0 : lastVisible(s.values))}{unit && <small>{unit}</small>}</b>
            </span>
          ))}
          {band && <span className="legend-item"><i className="band" />{band.label}</span>}
          {thresholds.map((t) => <span key={t.label} className={`legend-item threshold-${t.tone}`}><i className="thr" />{t.label}</span>)}
          {legendExtra}
          {sel && (
            <span className="selection-chip">
              {labels[sel[0]].full} → {labels[sel[1]].axis}
              <button onClick={() => setSel(null)} aria-label="Clear selected range">Clear</button>
            </span>
          )}
        </div>
      )}
      <div className="ts-plot" ref={ref} style={{ height }}>
        <svg width={w} height={height} role="img" aria-label={series.map((s) => s.label).join(', ')}
          onMouseMove={(e) => { const i = idxAt(e); setHover(i); if (drag != null) setSel([Math.min(drag, i), Math.max(drag, i)]); }}
          onMouseDown={(e) => { const i = idxAt(e); setDrag(i); setSel(null); }}
          onMouseUp={() => { setSel((s) => (s && s[1] - s[0] < 2 ? null : s)); setDrag(null); }}
          onMouseLeave={() => { setHover(null); setDrag(null); }}>
          <defs>
            <pattern id="hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><path d="M0 0v6" stroke="var(--grid-strong)" strokeWidth="2" /></pattern>
          </defs>
          {ticks.map((t, i) => (
            <g key={i}>
              <line x1={pl} x2={w - pr} y1={y(t)} y2={y(t)} className={i ? 'grid' : 'axis-line'} />
              <text x={pl - 7} y={y(t) + 3.5} textAnchor="end" className="tick">{format(t)}</text>
            </g>
          ))}
          {labels.map((l, i) => (i % xStep === 0 || i === n - 1) && (i === n - 1 || n - 1 - i >= xStep * 0.6) ? (
            <text key={i} x={x(i)} y={height - 6} textAnchor={i === 0 ? 'start' : i === n - 1 ? 'end' : 'middle'} className="tick">{l.axis}</text>
          ) : null)}
          {gaps.map(([a, b]) => (
            <g key={a}>
              <rect x={x(a - 0.5)} y={pt} width={x(b + 0.5) - x(a - 0.5)} height={ih} fill="url(#hatch)" />
              <text x={(x(a) + x(b)) / 2} y={pt + 12} textAnchor="middle" className="tick strong">No data</text>
            </g>
          ))}
          {sel && <rect x={x(sel[0])} y={pt} width={x(sel[1]) - x(sel[0])} height={ih} className="selection" />}
          {band && <path d={bandPath(band.upper, band.lower, x, y)} className="band-area" />}
          {stacked && [...series].map((s, k) => ({ s, k })).reverse().map(({ s, k }) => {
            if (s.overlay) return null;
            let prev = -1;
            for (let j = k - 1; j >= 0; j--) if (!series[j].overlay) { prev = j; break; }
            const lower = prev >= 0 ? plotted[prev] : plotted[k].map((v) => (v == null ? null : 0));
            return <path key={s.key} d={bandPath(plotted[k], lower, x, y)} fill={s.color} opacity={k === firstStack ? 0.32 : 0.26} />;
          })}
          {series.map((s, k) => (
            <path key={s.key} d={linePath(plotted[k], x, y)} fill="none" stroke={s.color} strokeWidth={s.overlay ? 1.8 : 1.5} strokeDasharray={s.dashed ? '4 4' : undefined} strokeLinejoin="round" />
          ))}
          {!stacked && series.length === 1 && !series[0].dashed && (
            <path d={`${linePath(plotted[0], x, y)}`} fill="none" stroke="transparent" />
          )}
          {thresholds.map((t) => (
            <g key={t.label} className={`thr-line thr-${t.tone}`}>
              <line x1={pl} x2={w - pr} y1={y(t.value)} y2={y(t.value)} />
              <text x={w - pr - 4} y={y(t.value) - 4} textAnchor="end">{t.label}</text>
            </g>
          ))}
          {annotations.map((a) => (
            <g key={a.label} className="annotation">
              <line x1={x(a.index)} x2={x(a.index)} y1={pt} y2={pt + ih} />
              <rect x={x(a.index) - 4} y={pt - 1} width="8" height="8" transform={`rotate(45 ${x(a.index)} ${pt + 3})`} />
            </g>
          ))}
          {hover != null && (
            <g>
              <line x1={x(hover)} x2={x(hover)} y1={pt} y2={pt + ih} className="crosshair" />
              {series.map((s, k) => plotted[k][hover] != null && <circle key={s.key} cx={x(hover)} cy={y(plotted[k][hover] as number)} r="3" fill="var(--surface)" stroke={s.color} strokeWidth="1.6" />)}
            </g>
          )}
        </svg>
        {hover != null && (
          <div className="tooltip" style={{ left: x(hover) + (tipRight ? -12 : 12), transform: tipRight ? 'translateX(-100%)' : undefined }}>
            <div className="tooltip-time">{labels[hover].full}</div>
            {annotations.filter((a) => Math.abs(a.index - hover) <= 1).map((a) => <div key={a.label} className="tooltip-note">{a.label}</div>)}
            {series.map((s) => (
              <div key={s.key} className="tooltip-row">
                <i style={{ background: s.color }} />
                <span>{s.label}</span>
                <b>{s.values[hover] == null ? '—' : `${format(s.values[hover] as number)}${unit ? ` ${unit}` : ''}`}</b>
              </div>
            ))}
            {band && band.upper[hover] != null && <div className="tooltip-row"><i className="band" /><span>{band.label}</span><b>{format(band.lower[hover] as number)}–{format(band.upper[hover] as number)}</b></div>}
          </div>
        )}
      </div>
    </div>
  );
}

function lastVisible(vals: (number | null)[]) {
  for (let i = vals.length - 1; i >= 0; i--) if (vals[i] != null) return vals[i] as number;
  return 0;
}

export type Bin = { label: string; from: number; to: number; count: number };
export function Histogram({ bins, markers, threshold, height = 170, unit = 'ms', countLabel = 'requests' }: { bins: Bin[]; markers: { value: number; label: string; color: string }[]; threshold?: { value: number; label: string }; height?: number; unit?: string; countLabel?: string }) {
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const pl = 46, pr = 10, pt = 18, pb = 22;
  const iw = Math.max(10, w - pl - pr), ih = height - pt - pb;
  const total = bins.reduce((a, b) => a + b.count, 0);
  const max = niceMax(Math.max(...bins.map((b) => b.count)) * 1.05);
  const bw = iw / bins.length;
  const valueX = (v: number) => {
    const i = bins.findIndex((b) => v >= b.from && v < b.to);
    if (i < 0) return pl + iw;
    const b = bins[i];
    return pl + bw * (i + (v - b.from) / (b.to - b.from));
  };
  const y = (v: number) => pt + ih - (v / max) * ih;
  return (
    <div className="ts">
      <div className="legend">
        {markers.map((m) => <span key={m.label} className="legend-item"><i style={{ background: m.color }} />{m.label}<b>{m.value >= 1000 ? `${(m.value / 1000).toFixed(2)} s` : `${Math.round(m.value)} ${unit}`}</b></span>)}
        {threshold && <span className="legend-item threshold-warning"><i className="thr" />{threshold.label}</span>}
      </div>
      <div className="ts-plot" ref={ref} style={{ height }}>
        <svg width={w} height={height} onMouseLeave={() => setHover(null)} role="img" aria-label="Latency distribution histogram">
          {[0, 0.5, 1].map((f) => (
            <g key={f}>
              <line x1={pl} x2={w - pr} y1={y(f * max)} y2={y(f * max)} className={f ? 'grid' : 'axis-line'} />
              <text x={pl - 7} y={y(f * max) + 3.5} textAnchor="end" className="tick">{f * max >= 1000 ? `${Math.round((f * max) / 1000)}k` : Math.round(f * max)}</text>
            </g>
          ))}
          {threshold && <rect x={valueX(threshold.value)} y={pt} width={pl + iw - valueX(threshold.value)} height={ih} className="over-threshold" />}
          {bins.map((b, i) => (
            <rect key={i} x={pl + i * bw + 1} y={y(b.count)} width={Math.max(1, bw - 2)} height={pt + ih - y(b.count)} rx="1"
              className={`hbar ${threshold && b.from >= threshold.value ? 'hbar-over' : ''} ${hover === i ? 'hbar-hover' : ''}`} onMouseEnter={() => setHover(i)} />
          ))}
          {bins.map((b, i) => (i % Math.ceil(bins.length / Math.max(3, Math.floor(iw / 60))) === 0 ? <text key={i} x={pl + i * bw} y={height - 6} className="tick">{b.from >= 1000 ? `${b.from / 1000}s` : b.from}</text> : null))}
          <text x={w - pr} y={height - 6} textAnchor="end" className="tick">{unit}</text>
          {markers.map((m) => (
            <g key={m.label}>
              <line x1={valueX(m.value)} x2={valueX(m.value)} y1={pt - 4} y2={pt + ih} stroke={m.color} strokeWidth="1.5" strokeDasharray="3 2" />
              <text x={valueX(m.value)} y={pt - 7} textAnchor="middle" className="tick strong" fill={m.color}>{m.label}</text>
            </g>
          ))}
        </svg>
        {hover != null && (
          <div className="tooltip" style={{ left: Math.min(pl + hover * bw + bw + 8, w - 170) }}>
            <div className="tooltip-time">{bins[hover].label} {unit}</div>
            <div className="tooltip-row"><span>{countLabel}</span><b>{bins[hover].count.toLocaleString('en-US')}</b></div>
            <div className="tooltip-row"><span>share</span><b>{((bins[hover].count / total) * 100).toFixed(1)}%</b></div>
          </div>
        )}
      </div>
    </div>
  );
}

const HEAT_STEPS = [5, 20, 40, 60, 75, 90];
export function Heatmap({ data, rowLabels, colLabels, format = (v) => `${Math.round(v)}%`, cell = 14, highlightRows = [] }: { data: number[][]; rowLabels: string[]; colLabels: Label[]; format?: (v: number) => string; cell?: number; highlightRows?: number[] }) {
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<[number, number] | null>(null);
  const pl = 46, pb = 20;
  const cols = data[0]?.length ?? 1;
  const cw = (w - pl) / cols;
  const h = data.length * cell + pb;
  const level = (v: number) => HEAT_STEPS.filter((s) => v >= s).length;
  return (
    <div className="heatmap" ref={ref}>
      <svg width={w} height={h} onMouseLeave={() => setHover(null)} role="img" aria-label="Per-core CPU utilisation heatmap">
        {data.map((row, r) => (
          <g key={r}>
            <text x={pl - 8} y={r * cell + cell / 2 + 3.5} textAnchor="end" className={`tick ${highlightRows.includes(r) ? 'strong warn-text' : ''}`}>{rowLabels[r]}</text>
            {row.map((v, c) => (
              <rect key={c} x={pl + c * cw + 0.5} y={r * cell + 0.5} width={Math.max(1, cw - 1)} height={cell - 1} className={`heat h${level(v)} ${hover && hover[0] === r && hover[1] === c ? 'heat-hover' : ''}`} onMouseEnter={() => setHover([r, c])} />
            ))}
          </g>
        ))}
        {colLabels.map((l, c) => (c % Math.ceil(cols / 5) === 0 || c === cols - 1 ? <text key={c} x={pl + c * cw + (c === cols - 1 ? cw : 0)} y={h - 5} textAnchor={c === cols - 1 ? 'end' : 'start'} className="tick">{l.axis}</text> : null))}
      </svg>
      {hover && (
        <div className="tooltip" style={{ left: Math.min(pl + hover[1] * cw + cw + 6, w - 160), top: hover[0] * cell - 4 }}>
          <div className="tooltip-time">{colLabels[hover[1]].full}</div>
          <div className="tooltip-row"><span>{rowLabels[hover[0]]}</span><b>{format(data[hover[0]][hover[1]])}</b></div>
        </div>
      )}
    </div>
  );
}
export function HeatLegend({ label = 'Utilisation' }: { label?: string }) {
  return (
    <div className="heat-legend">
      <span>{label}</span>
      {[0, 1, 2, 3, 4, 5, 6].map((l) => <i key={l} className={`heat-swatch h${l}`} />)}
      <span className="mono">0 → 100%</span>
    </div>
  );
}

export type Part = { key: string; label: string; value: number; color: string; detail?: string };
export function ProportionBar({ parts, active, onToggle, format = (v) => v.toLocaleString('en-US'), unit = '' }: { parts: Part[]; active?: Set<string>; onToggle?: (k: string) => void; format?: (v: number) => string; unit?: string }) {
  const total = parts.reduce((a, b) => a + b.value, 0);
  return (
    <div className="proportion">
      <div className="proportion-bar" role="img" aria-label={parts.map((p) => `${p.label} ${((p.value / total) * 100).toFixed(1)}%`).join(', ')}>
        {parts.map((p) => {
          const pct = (p.value / total) * 100;
          const dim = active && !active.has(p.key);
          return (
            <span key={p.key} style={{ width: `${pct}%`, background: p.color }} className={dim ? 'dim' : ''} title={`${p.label}: ${pct.toFixed(1)}%`}>
              {pct > 9 && <em>{pct.toFixed(1)}%</em>}
            </span>
          );
        })}
      </div>
      <div className="proportion-legend">
        {parts.map((p) => {
          const pct = (p.value / total) * 100;
          const on = !active || active.has(p.key);
          const Tag = onToggle ? 'button' : 'div';
          return (
            <Tag key={p.key} className={`prop-item ${on ? '' : 'off'}`} onClick={onToggle ? () => onToggle(p.key) : undefined} aria-pressed={onToggle ? on : undefined}>
              <i style={{ background: p.color }} />
              <span>{p.label}</span>
              <b>{pct.toFixed(1)}%</b>
              <small>{format(p.value)}{unit}</small>
              {p.detail && <em>{p.detail}</em>}
            </Tag>
          );
        })}
      </div>
    </div>
  );
}

export function RankedBars({ items, max }: { items: { key: string; label: ReactNode; value: number; display: string; sub?: string; tone?: 'warning' | 'critical' | 'cyan' }[]; max?: number }) {
  const m = max ?? Math.max(...items.map((i) => i.value));
  return (
    <div className="ranked">
      {items.map((it, i) => (
        <div key={it.key} className="ranked-row">
          <span className="rank mono">{i + 1}</span>
          <div className="ranked-main">
            <div className="ranked-text"><span>{it.label}</span><b className="mono">{it.display}</b></div>
            <div className="ranked-track"><i className={it.tone ? `tone-bg-${it.tone}` : ''} style={{ width: `${(it.value / m) * 100}%` }} /></div>
            {it.sub && <small>{it.sub}</small>}
          </div>
        </div>
      ))}
    </div>
  );
}

export function Capacity({ value, max, warn, crit, label, display, sub }: { value: number; max: number; warn: number; crit: number; label?: ReactNode; display?: ReactNode; sub?: ReactNode }) {
  const pct = (value / max) * 100;
  const tone = pct >= crit ? 'critical' : pct >= warn ? 'warning' : 'ok';
  return (
    <div className={`capacity cap-${tone}`}>
      {(label || display) && <div className="capacity-text"><span>{label}</span><b className="mono">{display ?? `${pct.toFixed(1)}%`}</b></div>}
      <div className="capacity-track" role="meter" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100}>
        <i style={{ width: `${Math.min(100, pct)}%` }} />
        <span className="cap-mark" style={{ left: `${warn}%` }} title={`Warning ${warn}%`} />
        <span className="cap-mark crit" style={{ left: `${crit}%` }} title={`Critical ${crit}%`} />
      </div>
      {sub && <small>{sub}</small>}
    </div>
  );
}
