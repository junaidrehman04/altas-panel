import { useEffect, useRef, useState } from "react"
import type { AlertItem } from "../lib/panel"
import { Icon, Segmented, type IconName } from "./ui"

export type Alert = AlertItem

type Filter = "all" | "unread" | "critical"

export function NotificationsMenu({
  items,
  onRead,
  onDismiss,
  onOpen,
}: {
  items: Alert[]
  onRead: (ids: string[] | "all") => void
  onDismiss: (id: string) => void
  onOpen: (alert: Alert) => void
}) {
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState<Filter>("all")
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false)
    }
    document.addEventListener("mousedown", onDoc)
    window.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("mousedown", onDoc)
      window.removeEventListener("keydown", onKey)
    }
  }, [open])

  const unread = items.filter((i) => !i.read).length
  const critical = items.filter((i) => i.sev === "critical").length
  const shown = items.filter((i) => (filter === "unread" ? !i.read : filter === "critical" ? i.sev === "critical" : true))

  return (
    <div className="notif" ref={ref}>
      <button className={`icon-btn ${open ? "pressed" : ""}`} aria-label={`Notifications, ${unread} unread`} aria-expanded={open} aria-haspopup="dialog" onClick={() => setOpen((v) => !v)}>
        <Icon name="bell" />
        {unread > 0 && <span className="count">{unread}</span>}
      </button>
      {open && (
        <div className="notif-pop" role="dialog" aria-label="Notifications">
          <div className="notif-head">
            <div><b>Notifications</b><small>{unread ? `${unread} unread` : "All caught up"}</small></div>
            <button className="link" disabled={!unread} onClick={() => onRead("all")}>Mark all read</button>
          </div>
          <div className="notif-filters">
            <Segmented label="Filter notifications" value={filter} onChange={setFilter} options={[{ value: "all", label: `All ${items.length}` }, { value: "unread", label: `Unread ${unread}` }, { value: "critical", label: `Critical ${critical}` }]} />
          </div>
          {shown.length === 0 ? (
            <div className="notif-empty"><span className="state-icon ok"><Icon name="check" size={18} /></span><b>You're all caught up</b><p>New alerts from your servers will appear here.</p></div>
          ) : (
            <ul className="notif-list">
              {shown.map((a) => (
                <li key={a.id} className={`notif-item ${a.read ? "" : "unread"}`}>
                  <button className="notif-main" onClick={() => { onRead([a.id]); setOpen(false); onOpen(a) }}>
                    <span className={`notif-icon sev-${a.sev}`}><Icon name={a.icon as IconName} size={15} /></span>
                    <span className="notif-text">
                      <b>{a.title}</b>
                      <span>{a.detail}</span>
                      <small><span className="mono">{a.host}</span> · {a.time}</small>
                    </span>
                    {!a.read && <i className="unread-dot" aria-label="Unread" />}
                  </button>
                  <button className="notif-dismiss" aria-label={`Dismiss ${a.title}`} onClick={() => onDismiss(a.id)}><Icon name="x" size={13} /></button>
                </li>
              ))}
            </ul>
          )}
          <div className="notif-foot"><Icon name="clock" size={12} />Alert rules evaluated from the latest host sample</div>
        </div>
      )}
    </div>
  )
}
