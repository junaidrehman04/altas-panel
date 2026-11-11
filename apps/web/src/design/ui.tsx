import { useEffect, useRef, useState, type ReactNode } from 'react';

export type IconName =
  | 'globe' | 'activity' | 'cpu' | 'server' | 'shield' | 'bell' | 'search' | 'sun' | 'moon' | 'chevron' | 'down'
  | 'arrow' | 'check' | 'warning' | 'info' | 'terminal' | 'database' | 'mail' | 'lock' | 'key' | 'eye' | 'eyeOff'
  | 'logout' | 'refresh' | 'pause' | 'play' | 'x' | 'filter' | 'download' | 'ban' | 'clock' | 'fan' | 'thermo'
  | 'power' | 'disk' | 'network' | 'memory' | 'layers' | 'external' | 'file' | 'box' | 'calendar' | 'help' | 'menu' | 'user' | 'command';

const P: Record<IconName, ReactNode> = {
  globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c2.8 3 2.8 15 0 18M12 3c-2.8 3-2.8 15 0 18" /></>,
  activity: <path d="M3 12h4l2.5-7 5 14 2.5-7H21" />,
  cpu: <><rect x="6" y="6" width="12" height="12" rx="1.5" /><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4" /><rect x="9.5" y="9.5" width="5" height="5" /></>,
  server: <><rect x="3" y="4" width="18" height="7" rx="1.5" /><rect x="3" y="13" width="18" height="7" rx="1.5" /><path d="M7 7.5h.01M7 16.5h.01" /></>,
  shield: <path d="M12 3 19.5 6v5.5c0 4.6-3.2 8.2-7.5 9.5-4.3-1.3-7.5-4.9-7.5-9.5V6z" />,
  bell: <path d="M18 9a6 6 0 0 0-12 0c0 6-2.5 7.5-2.5 7.5h17S18 15 18 9M10 20a2 2 0 0 0 4 0" />,
  search: <><circle cx="11" cy="11" r="6.5" /><path d="m20 20-4.2-4.2" /></>,
  sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>,
  moon: <path d="M20 14.5A8.5 8.5 0 0 1 9.5 4 8.5 8.5 0 1 0 20 14.5z" />,
  chevron: <path d="m9 6 6 6-6 6" />,
  down: <path d="m6 9 6 6 6-6" />,
  arrow: <path d="M5 12h14M13 6l6 6-6 6" />,
  check: <path d="m5 12.5 4.5 4.5L19 7" />,
  warning: <><path d="M10.3 4.2 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z" /><path d="M12 9.5v4M12 17h.01" /></>,
  info: <><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></>,
  terminal: <><rect x="3" y="4" width="18" height="16" rx="2" /><path d="m7 9 3 3-3 3M13 15h4" /></>,
  database: <><ellipse cx="12" cy="5.5" rx="7.5" ry="2.8" /><path d="M4.5 5.5v13c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8v-13M4.5 12c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8" /></>,
  mail: <><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m3.5 6.5 8.5 6.5 8.5-6.5" /></>,
  lock: <><rect x="4.5" y="10.5" width="15" height="10" rx="2" /><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3" /></>,
  key: <><circle cx="8" cy="15" r="4" /><path d="m11 12 8.5-8.5M16 6l2.5 2.5M13.5 8.5 16 11" /></>,
  eye: <><path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z" /><circle cx="12" cy="12" r="3" /></>,
  eyeOff: <><path d="M3 3l18 18M10.6 5.1A10 10 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.2 4M6.6 6.6C3.8 8.4 2 12 2 12s3.6 7 10 7c1.7 0 3.2-.5 4.5-1.2M9.9 9.9a3 3 0 0 0 4.2 4.2" /></>,
  logout: <path d="M9 4H5v16h4M15 8l4 4-4 4M9 12h10" />,
  refresh: <path d="M20 11a8 8 0 0 0-14.6-4.5L4 8M4 4v4h4M4 13a8 8 0 0 0 14.6 4.5L20 16M20 20v-4h-4" />,
  pause: <path d="M9 5v14M15 5v14" />,
  play: <path d="M7 4.5v15l12-7.5z" />,
  x: <path d="M6 6l12 12M18 6 6 18" />,
  filter: <path d="M3 5h18l-7 8.5V19l-4 2v-7.5z" />,
  download: <path d="M12 3v12M7 10l5 5 5-5M4 20h16" />,
  ban: <><circle cx="12" cy="12" r="9" /><path d="m5.6 5.6 12.8 12.8" /></>,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  fan: <><circle cx="12" cy="12" r="2" /><path d="M12 10c0-4 1-7 4-7s3 4-2 7.5M14 12c4 0 7 1 7 4s-4 3-7.5-2M12 14c0 4-1 7-4 7s-3-4 2-7.5M10 12c-4 0-7-1-7-4s4-3 7.5 2" /></>,
  thermo: <path d="M14 14.8V5a2 2 0 0 0-4 0v9.8a4 4 0 1 0 4 0zM12 9v7" />,
  power: <path d="M12 3v8M6.4 6.4a8 8 0 1 0 11.2 0" />,
  disk: <><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 14h18M7 16.5h.01M11 16.5h.01" /></>,
  network: <><rect x="9" y="3" width="6" height="5" rx="1" /><rect x="3" y="16" width="6" height="5" rx="1" /><rect x="15" y="16" width="6" height="5" rx="1" /><path d="M12 8v4M6 16v-4h12v4" /></>,
  memory: <><rect x="3" y="7" width="18" height="10" rx="1.5" /><path d="M7 10.5v3M11 10.5v3M15 10.5v3M6 17v3M10 17v3M14 17v3M18 17v3" /></>,
  layers: <path d="m12 3 9 5-9 5-9-5zM3 13l9 5 9-5" />,
  external: <path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />,
  file: <path d="M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8zM14 3v5h5" />,
  box: <path d="m12 3 8 4.5v9L12 21l-8-4.5v-9zM4 7.5l8 4.5 8-4.5M12 12v9" />,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 10h18M8 3v4M16 3v4" /></>,
  help: <><circle cx="12" cy="12" r="9" /><path d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5V14M12 17h.01" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21c0-4 3.6-6.5 8-6.5s8 2.5 8 6.5" /></>,
  command: <path d="M9 6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3z" />,
  menu: <path d="M4 7h16M4 12h16M4 17h16" />,
};

export function Icon({ name, size = 16, className = '' }: { name: IconName; size?: number; className?: string }) {
  return (
    <svg className={`icon ${className}`} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {P[name]}
    </svg>
  );
}

/**
 * ALTAS mark: an "A" built from two load-bearing struts meeting at a node,
 * with the crossbar drawn as a stacked server slot — infrastructure carrying weight.
 */
export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true" className="logo-mark">
      <rect width="32" height="32" rx="8" fill="#1D4ED8" />
      <path d="M8.5 24.5 16 8l7.5 16.5" fill="none" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round" />
      <rect x="11.6" y="17.6" width="8.8" height="3.4" rx="1.2" fill="#67E8F9" />
      <circle cx="16" cy="8" r="2.4" fill="#fff" />
    </svg>
  );
}

export function Wordmark({ size = 'md' }: { size?: 'md' | 'lg' }) {
  return (
    <span className={`wordmark wordmark-${size}`}>
      <Logo size={size === 'lg' ? 44 : 26} />
      <span className="wordmark-text"><b>ALTAS</b><em>Panel</em></span>
    </span>
  );
}

export function InfoTip({ children, label = 'About this metric' }: { children: ReactNode; label?: string }) {
  return (
    <span className="infotip">
      <button type="button" aria-label={label} className="infotip-btn"><Icon name="info" size={14} /></button>
      <span role="tooltip" className="infotip-pop">{children}</span>
    </span>
  );
}

export function Tabs<T extends string>({ value, onChange, items, label }: { value: T; onChange: (v: T) => void; items: { value: T; label: string; badge?: number }[]; label: string }) {
  return (
    <div className="tabs" role="tablist" aria-label={label}>
      {items.map((it) => (
        <button key={it.value} role="tab" aria-selected={value === it.value} className={value === it.value ? 'on' : ''} onClick={() => onChange(it.value)}>
          {it.label}
          {it.badge ? <span className="tab-badge">{it.badge}</span> : null}
        </button>
      ))}
    </div>
  );
}

export type Tone = 'ok' | 'warning' | 'critical' | 'info' | 'neutral' | 'muted';
export function Dot({ tone = 'ok' }: { tone?: Tone }) {
  return <span className={`dot dot-${tone}`} aria-hidden="true" />;
}
export function Badge({ tone = 'neutral', children }: { tone?: Tone; children: ReactNode }) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export type SourceKind = 'live' | 'historical' | 'measured' | 'estimated' | 'unavailable' | 'calculated';
const SOURCE_TEXT: Record<SourceKind, string> = {
  live: 'Live',
  historical: 'Historical',
  measured: 'Measured',
  estimated: 'Estimated',
  unavailable: 'Unavailable',
  calculated: 'Calculated',
};
export function SourceTag({ kind, detail }: { kind: SourceKind; detail?: string }) {
  return (
    <span className={`source source-${kind}`} title={detail ? `${SOURCE_TEXT[kind]} · ${detail}` : SOURCE_TEXT[kind]}>
      <i />
      {SOURCE_TEXT[kind]}
    </span>
  );
}

export function Segmented<T extends string>({ value, options, onChange, label }: { value: T; options: { value: T; label: ReactNode }[]; onChange: (v: T) => void; label: string }) {
  return (
    <div className="segmented" role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} role="radio" aria-checked={value === o.value} className={value === o.value ? 'on' : ''} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Spinner({ size = 14, className = '' }: { size?: number; className?: string }) {
  return <span className={`spinner ${className}`} style={{ width: size, height: size }} aria-hidden="true" />;
}

export function Skeleton({ height = 140 }: { height?: number }) {
  return (
    <div className="skeleton" style={{ height }} aria-busy="true" aria-label="Loading">
      <span className="skeleton-label"><Spinner size={12} />Loading data…</span>
      <span style={{ width: '38%' }} />
      <span style={{ width: '100%', flex: 1 }} />
    </div>
  );
}

export type PanelState = 'ready' | 'loading' | 'unavailable' | 'empty';

export function Panel({
  title, definition, meta, actions, state = 'ready', stateTitle, stateText, stateAction, className = '', children, flush = false, minHeight,
}: {
  title: ReactNode; definition?: ReactNode; meta?: ReactNode; actions?: ReactNode; state?: PanelState; stateTitle?: string; stateText?: ReactNode; stateAction?: ReactNode; className?: string; children?: ReactNode; flush?: boolean; minHeight?: number;
}) {
  return (
    <section className={`panel ${className}`}>
      <header className="panel-head">
        <div className="panel-title">
          <h3>{title}</h3>
          {definition && <InfoTip>{definition}</InfoTip>}
        </div>
        <div className="panel-tools">
          {meta}
          {actions}
        </div>
      </header>
      <div className={flush ? 'panel-body flush' : 'panel-body'} style={{ minHeight }}>
        {state === 'loading' && <Skeleton height={minHeight ? minHeight - 28 : 140} />}
        {(state === 'unavailable' || state === 'empty') && (
          <div className={`panel-state state-${state}`}>
            <Icon name={state === 'unavailable' ? 'ban' : 'info'} size={18} />
            <b>{stateTitle ?? (state === 'unavailable' ? 'Data unavailable' : 'No data in selected range')}</b>
            {stateText && <p>{stateText}</p>}
            {stateAction}
          </div>
        )}
        {state === 'ready' && children}
      </div>
    </section>
  );
}

export function Dialog({
  open, title, children, confirmLabel, requireText, onConfirm, onClose, tone = 'danger',
}: {
  open: boolean; title: string; children: ReactNode; confirmLabel: string; requireText?: string; onConfirm: () => void; onClose: () => void; tone?: 'danger' | 'primary';
}) {
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useEffect(() => {
    if (!open) return;
    setText('');
    setBusy(false);
    const h = (e: KeyboardEvent) => e.key === 'Escape' && closeRef.current();
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, [open]);
  if (!open) return null;
  const ok = !requireText || text.trim() === requireText;
  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div className="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dialog-title" onMouseDown={(e) => e.stopPropagation()}>
        <div className="dialog-head">
          <span className={`dialog-icon ${tone}`}><Icon name={tone === 'danger' ? 'warning' : 'info'} size={18} /></span>
          <h2 id="dialog-title">{title}</h2>
          <button className="icon-btn ghost" aria-label="Close dialog" onClick={onClose}><Icon name="x" /></button>
        </div>
        <div className="dialog-body">
          {children}
          {requireText && (
            <label className="field">
              <span>Type <code>{requireText}</code> to confirm</span>
              <input autoFocus value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} autoComplete="off" />
            </label>
          )}
        </div>
        <div className="dialog-foot">
          <button className="btn" onClick={onClose} autoFocus={!requireText} disabled={busy}>Cancel</button>
          <button className={`btn ${tone === 'danger' ? 'btn-danger' : 'btn-primary'}`} disabled={!ok || busy} aria-busy={busy}
            onClick={() => { setBusy(true); window.setTimeout(() => { onConfirm(); onClose(); }, 900); }}>
            {busy ? <><Spinner size={13} />Applying…</> : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

export function useToast() {
  const [toast, setToast] = useState<string | null>(null);
  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 3600);
    return () => clearTimeout(t);
  }, [toast]);
  const node = toast ? (
    <div className="toast" role="status"><Icon name="check" /><span>{toast}</span><button aria-label="Dismiss" onClick={() => setToast(null)}><Icon name="x" size={14} /></button></div>
  ) : null;
  return { show: setToast, node };
}

export function Stat({ label, value, unit, delta, deltaTone = 'ok', context, source }: { label: string; value: string; unit?: string; delta?: string; deltaTone?: Tone; context?: ReactNode; source?: SourceKind }) {
  return (
    <div className="stat">
      <div className="stat-label">{label}{source && <SourceTag kind={source} />}</div>
      <div className="stat-value"><b>{value}</b>{unit && <small>{unit}</small>}{delta && <span className={`delta tone-${deltaTone}`}>{delta}</span>}</div>
      {context && <div className="stat-context">{context}</div>}
    </div>
  );
}
