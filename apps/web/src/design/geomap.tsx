import { useState } from 'react';
import { useWidth } from './charts';
import { fmtInt } from './data';

export type Mix = { human: number; bot: number; ai: number; mal: number };
export type City = { city: string; cc: string; country: string; lat: number; lon: number; rps: number; asn: string; mix: Mix; anomaly?: string };
export type Cls = keyof Mix;

// Simplified land masses (lon, lat) — accurate enough for regional reading at panel scale.
const LAND: [number, number][][] = [
  [[-168, 65], [-162, 70], [-140, 70], [-120, 72], [-95, 72], [-80, 73], [-65, 62], [-55, 52], [-66, 45], [-75, 38], [-81, 31], [-80, 25], [-90, 29], [-97, 26], [-97, 20], [-87, 21], [-83, 10], [-78, 8], [-90, 14], [-105, 20], [-110, 23], [-117, 32], [-124, 40], [-124, 48], [-135, 58], [-150, 60], [-165, 60]],
  [[-52, 60], [-42, 60], [-20, 70], [-20, 80], [-60, 82], [-72, 77], [-55, 70]],
  [[-78, 8], [-60, 10], [-50, 0], [-35, -5], [-38, -15], [-48, -25], [-58, -35], [-65, -42], [-68, -55], [-74, -50], [-72, -35], [-71, -18], [-81, -5], [-80, 2]],
  [[-10, 36], [-9, 43], [-2, 44], [-5, 48], [2, 51], [8, 54], [10, 58], [5, 62], [15, 69], [28, 71], [40, 67], [40, 55], [30, 45], [28, 41], [22, 40], [18, 40], [12, 44], [8, 44], [3, 43], [-1, 37]],
  [[-17, 15], [-17, 21], [-10, 30], [-5, 36], [10, 37], [20, 32], [32, 31], [35, 28], [43, 12], [51, 12], [40, -2], [40, -15], [35, -25], [20, -35], [17, -29], [12, -17], [13, -5], [9, 4], [-8, 5], [-15, 10]],
  [[28, 41], [36, 36], [35, 31], [43, 13], [52, 16], [58, 22], [60, 25], [67, 24], [73, 18], [77, 8], [80, 15], [88, 22], [92, 22], [98, 16], [100, 6], [104, 1], [106, 10], [110, 20], [120, 30], [122, 40], [130, 43], [140, 50], [143, 58], [160, 60], [170, 65], [180, 68], [180, 72], [140, 73], [110, 77], [80, 73], [60, 70], [40, 67], [40, 55], [30, 45]],
  [[-5, 50], [1, 51], [-1, 55], [-3, 58], [-6, 57]],
  [[130, 31], [136, 34], [141, 38], [142, 44], [140, 41], [133, 35]],
  [[114, -22], [122, -18], [131, -12], [137, -12], [142, -11], [146, -19], [153, -25], [150, -37], [140, -38], [132, -32], [115, -34]],
  [[95, 5], [105, -6], [115, -8], [120, -8], [106, -2], [100, 2]],
  [[44, -25], [50, -15], [48, -13], [43, -22]],
  [[167, -46], [174, -41], [178, -38], [173, -35], [172, -41]],
];

export const CLASS_META: { key: Cls; label: string; color: string }[] = [
  { key: 'human', label: 'Human', color: 'var(--c-human)' },
  { key: 'bot', label: 'Trusted bots', color: 'var(--c-bot)' },
  { key: 'ai', label: 'AI crawlers', color: 'var(--c-ai)' },
  { key: 'mal', label: 'Malicious', color: 'var(--c-mal)' },
];

export function GeoMap({ cities, active, mode, selected, onSelect, neutral = false }: { cities: City[]; active: Set<Cls>; mode: 'volume' | 'malicious'; selected: string | null; onSelect: (c: string | null) => void; neutral?: boolean }) {
  const [ref, w] = useWidth<HTMLDivElement>(800);
  const h = Math.round(w * 0.44);
  const [hover, setHover] = useState<City | null>(null);
  if (cities.length === 0) return <div className="geomap" ref={ref} />;
  const proj = (lon: number, lat: number): [number, number] => [((lon + 180) / 360) * w, ((84 - lat) / 142) * h];
  const val = (c: City) => c.rps * [...active].reduce((a, k) => a + c.mix[k], 0);
  const maxV = Math.max(1, ...cities.map((c) => c.rps));
  const radius = (v: number) => 2.5 + Math.sqrt(v / maxV) * 17;
  const malLevel = (m: number) => (m >= 0.4 ? 4 : m >= 0.25 ? 3 : m >= 0.12 ? 2 : m >= 0.05 ? 1 : 0);
  const dominant = (c: City): Cls => (['human', 'bot', 'ai', 'mal'] as Cls[]).filter((k) => active.has(k)).sort((a, b) => c.mix[b] - c.mix[a])[0] ?? 'human';
  const sorted = [...cities].sort((a, b) => val(b) - val(a));
  const hp = hover ? proj(hover.lon, hover.lat) : null;

  return (
    <div className="geomap" ref={ref}>
      <svg width={w} height={h} role="img" aria-label="Geo-IP traffic distribution map" onClick={() => onSelect(null)}>
        {[-60, -30, 0, 30, 60].map((lat) => <line key={lat} x1={0} x2={w} y1={proj(0, lat)[1]} y2={proj(0, lat)[1]} className={lat === 0 ? 'graticule eq' : 'graticule'} />)}
        {[-120, -60, 0, 60, 120].map((lon) => <line key={lon} y1={0} y2={h} x1={proj(lon, 0)[0]} x2={proj(lon, 0)[0]} className="graticule" />)}
        {LAND.map((poly, i) => <path key={i} className="land" d={`M${poly.map(([lo, la]) => proj(lo, la).map((v) => v.toFixed(1)).join(',')).join('L')}Z`} />)}
        {sorted.map((c) => {
          const [x, y] = proj(c.lon, c.lat);
          const v = val(c);
          const fill = neutral && mode === 'volume' ? 'var(--c-muted-line)' : mode === 'volume' ? CLASS_META.find((m) => m.key === dominant(c))!.color : undefined;
          return (
            <g key={c.city} transform={`translate(${x} ${y})`} className={`marker ${selected === c.city ? 'selected' : ''} ${selected && selected !== c.city ? 'faded' : ''}`}
              onMouseEnter={() => setHover(c)} onMouseLeave={() => setHover(null)} onClick={(e) => { e.stopPropagation(); onSelect(selected === c.city ? null : c.city); }}>
              {c.anomaly && <circle r={radius(v) + 5} className="anomaly-ring" />}
              <circle r={radius(v)} className={mode === 'malicious' ? `mal-l${malLevel(c.mix.mal)}` : 'vol'} style={fill ? { fill } : undefined} />
              <circle r="1.6" className="marker-core" />
            </g>
          );
        })}
      </svg>
      {hover && hp && (
        <div className="tooltip map-tip" style={{ left: Math.min(hp[0] + 16, w - 230), top: Math.max(4, hp[1] - 30) }}>
          <div className="tooltip-time">{hover.city}, {hover.country} <span className="mono">{hover.cc}</span></div>
          <div className="tooltip-row"><span>Top ASN</span><b>{hover.asn}</b></div>
          <div className="tooltip-row"><span>Requests/s (filtered)</span><b>{fmtInt(val(hover))}</b></div>
          <div className="tip-mix">{CLASS_META.map((m) => <i key={m.key} style={{ width: `${hover.mix[m.key] * 100}%`, background: m.color }} />)}</div>
          <div className="tip-mix-legend">{CLASS_META.map((m) => <span key={m.key}><i style={{ background: m.color }} />{Math.round(hover.mix[m.key] * 100)}%</span>)}</div>
          {hover.anomaly && <div className="tooltip-note">{hover.anomaly}</div>}
          <div className="tip-hint">Click to filter tables by location</div>
        </div>
      )}
      <div className="map-legend">
        {neutral && mode === 'volume' ? (
          <div className="legend-group"><span className="legend-title">Request volume</span><span className="legend-item"><i style={{ background: 'var(--c-muted-line)' }} />Unclassified</span></div>
        ) : mode === 'volume' ? (
          <div className="legend-group"><span className="legend-title">Dominant class</span>{CLASS_META.filter((m) => active.has(m.key)).map((m) => <span key={m.key} className="legend-item"><i style={{ background: m.color }} />{m.label}</span>)}</div>
        ) : (
          <div className="legend-group"><span className="legend-title">Malicious share</span>{['<5%', '5–12%', '12–25%', '25–40%', '≥40%'].map((l, i) => <span key={l} className="legend-item"><i className={`mal-swatch mal-l${i}`} />{l}</span>)}</div>
        )}
        <div className="legend-group size-scale"><span className="legend-title">Requests/s</span>{[200, 1000, 4000].map((v) => <span key={v} className="legend-item"><svg width={radius(v) * 2 + 2} height={radius(v) * 2 + 2}><circle cx={radius(v) + 1} cy={radius(v) + 1} r={radius(v)} className="size-circle" /></svg>{v >= 1000 ? `${v / 1000}k` : v}</span>)}</div>
        <div className="legend-group"><span className="legend-item"><svg width="16" height="16"><circle cx="8" cy="8" r="6" className="anomaly-ring" /></svg>Anomaly vs 7-day baseline</span></div>
      </div>
    </div>
  );
}
