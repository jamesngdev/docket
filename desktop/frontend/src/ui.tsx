// Small presentational building blocks shared by every view.
import { ReactNode, useEffect, useState } from "react";
import { X } from "lucide-react";

type BtnProps = {
  children: ReactNode;
  onClick?: () => void;
  kind?: "primary" | "ghost" | "danger" | "default";
  small?: boolean;
  disabled?: boolean;
  title?: string;
};

export function Button({ children, onClick, kind = "default", small, disabled, title }: BtnProps) {
  return (
    <button className={`btn btn-${kind}${small ? " btn-sm" : ""}`} onClick={onClick} disabled={disabled} title={title}>
      {children}
    </button>
  );
}

const STATUS_TONE: Record<string, string> = {
  done: "ok", approved: "ok", answered: "ok", running: "accent", processing: "accent",
  wait_review: "warn", pending: "info", open: "warn", draft: "warn", backlog: "muted",
  outdated: "muted", skipped: "muted", stopped: "muted", failed: "err",
};

export function Badge({ children, tone }: { children: ReactNode; tone?: string }) {
  const t = tone ?? STATUS_TONE[String(children)] ?? "muted";
  return <span className={`badge badge-${t}`}>{children}</span>;
}

export function Dot({ tone }: { tone: string }) {
  return <span className={`dot dot-${tone}`} />;
}

export function Empty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="empty">
      <div className="empty-title">{title}</div>
      {hint && <div className="empty-hint">{hint}</div>}
    </div>
  );
}

export function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  useEffect(() => {
    const k = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, [onClose]);
  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <div className={`modal${wide ? " modal-wide" : ""}`} onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <span>{title}</span>
          <button className="icon-btn" onClick={onClose}><X size={16} /></button>
        </div>
        <div className="modal-body">{children}</div>
      </div>
    </div>
  );
}

/** EditableText shows text and switches to a textarea on Edit; Save calls onSave. */
export function EditableText({ value, onSave, rows = 6, placeholder }: {
  value: string; onSave: (v: string) => Promise<void> | void; rows?: number; placeholder?: string;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  useEffect(() => { if (!editing) setDraft(value); }, [value, editing]);
  if (!editing) {
    return (
      <div className="editable" onDoubleClick={() => setEditing(true)} title="Double-click to edit">
        {value ? <pre className="prose">{value}</pre> : <span className="muted">{placeholder ?? "Empty — double-click to edit"}</span>}
        <button className="edit-link" onClick={() => setEditing(true)}>Edit</button>
      </div>
    );
  }
  return (
    <div className="editable editing">
      <textarea autoFocus rows={rows} value={draft} onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") setEditing(false);
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) { onSave(draft); setEditing(false); }
        }} />
      <div className="row gap-sm">
        <Button kind="primary" small onClick={async () => { await onSave(draft); setEditing(false); }}>Save ⌘↵</Button>
        <Button kind="ghost" small onClick={() => setEditing(false)}>Cancel</Button>
      </div>
    </div>
  );
}

export function Section({ title, right, children }: { title: string; right?: ReactNode; children: ReactNode }) {
  return (
    <section className="section">
      <div className="section-head"><span>{title}</span>{right}</div>
      {children}
    </section>
  );
}
