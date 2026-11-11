import { useEffect, useRef, useState } from 'react';
import { Icon, type IconName } from './ui';

export type Command = { id: string; label: string; group: string; icon: IconName; hint?: string; keywords?: string; run: () => void };

export function CommandPalette({ open, onClose, commands }: { open: boolean; onClose: () => void; commands: Command[] }) {
  const [q, setQ] = useState('');
  const [idx, setIdx] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => { if (open) { setQ(''); setIdx(0); } }, [open]);
  useEffect(() => { listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }); }, [idx]);
  if (!open) return null;

  // Every typed word must appear somewhere in the command text ("apm perf" → APM · Performance).
  const words = q.trim().toLowerCase().split(/\s+/).filter(Boolean);
  const list = commands.filter((c) => { const hay = `${c.label} ${c.group} ${c.keywords ?? ''}`.toLowerCase(); return words.every((w) => hay.includes(w)); });
  const run = (c: Command | undefined) => { if (!c) return; onClose(); c.run(); };

  return (
    <div className="palette-backdrop" onMouseDown={onClose}>
      <div className="palette" role="dialog" aria-modal="true" aria-label="Command palette" onMouseDown={(e) => e.stopPropagation()}>
        <div className="palette-input">
          <Icon name="search" size={16} />
          <input autoFocus value={q} placeholder="Search pages, servers, sections or actions…" aria-label="Search commands"
            onChange={(e) => { setQ(e.target.value); setIdx(0); }}
            onKeyDown={(e) => {
              if (e.key === 'ArrowDown') { e.preventDefault(); setIdx((i) => Math.min(list.length - 1, i + 1)); }
              if (e.key === 'ArrowUp') { e.preventDefault(); setIdx((i) => Math.max(0, i - 1)); }
              if (e.key === 'Enter') { e.preventDefault(); run(list[idx]); }
              if (e.key === 'Escape') onClose();
            }} />
          <kbd>esc</kbd>
        </div>
        <div className="palette-list" ref={listRef} role="listbox">
          {list.length === 0 && <div className="palette-empty">No results for “{q}”</div>}
          {list.map((c, i) => (
            <div key={c.id}>
              {(i === 0 || list[i - 1].group !== c.group) && <div className="palette-group">{c.group}</div>}
              <button role="option" aria-selected={i === idx} data-active={i === idx} className={`palette-item ${i === idx ? 'on' : ''}`} onMouseEnter={() => setIdx(i)} onClick={() => run(c)}>
                <Icon name={c.icon} size={15} />
                <span>{c.label}</span>
                {c.hint && <kbd>{c.hint}</kbd>}
              </button>
            </div>
          ))}
        </div>
        <div className="palette-foot"><span><kbd>↑</kbd><kbd>↓</kbd> navigate</span><span><kbd>↵</kbd> open</span><span><kbd>g</kbd> then <kbd>e</kbd>/<kbd>a</kbd>/<kbd>h</kbd> jump to page</span></div>
      </div>
    </div>
  );
}
