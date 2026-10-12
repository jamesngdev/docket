import { Check, ExternalLink, MessageSquare, Square, TerminalSquare, Trash2, X } from "lucide-react";
import { api, Ticket } from "../api";
import type { Ctx } from "../App";
import { Badge, Button, EditableText } from "../ui";

/** TicketDrawer shows one ticket with inline editing and every action on it. */
export default function TicketDrawer({ ctx, t, onClose }: { ctx: Ctx; t: Ticket; onClose: () => void }) {
  const { snap, repo, run } = ctx;
  const runOf = (snap.Runs ?? []).find((r) => r.ID === t.LockedBy && r.Status === "running");
  const feature = (snap.Features ?? []).find((f) => f.ID === t.FeatureID);
  const save = (field: string) => (v: string) => run("Saved", () => api.UpdateTicket(repo, t.ID, field, v));

  return (
    <aside className="drawer">
      <div className="drawer-head">
        <span className="mono muted">#{t.ID}</span>
        <Badge>{t.Status}</Badge>
        {t.NeedsUser && <Badge tone="warn">needs you</Badge>}
        {t.PRConflict && <Badge tone="err">conflict {t.ConflictAttempts}/2</Badge>}
        <span className="grow" />
        <button className="icon-btn" onClick={onClose}><X size={16} /></button>
      </div>
      <div className="drawer-body">
        <EditableText value={t.Title} rows={2} onSave={save("title")} />
        <div className="muted small">F{t.FeatureID} {feature?.Title} · phase {t.Phase}</div>

        <div className="row gap-sm wrap actions">
          {(t.Status === "backlog" || t.NeedsUser) && (
            <Button small kind="primary" onClick={() => run(`#${t.ID} approved`, () => api.ApproveTicket(repo, t.ID))}><Check size={14} /> Approve</Button>
          )}
          <Button small onClick={() => run("TechLead chat opened in Terminal", () => api.StartChat(repo, "ticket", t.ID))}><MessageSquare size={14} /> Chat with TechLead</Button>
          {runOf && <Button small onClick={() => run("", () => api.OpenInTerminal(runOf.Session))}><TerminalSquare size={14} /> Watch agent</Button>}
          {runOf && <Button small kind="danger" onClick={() => run(`Stopping ${runOf.ID}`, () => api.Daemon(repo, "stop", runOf.ID))}><Square size={14} /> Stop agent</Button>}
          {t.PRURL && <Button small onClick={() => run("", () => api.OpenURL(t.PRURL))}><ExternalLink size={14} /> Open PR</Button>}
          {(t.Status === "backlog" || t.Status === "pending") && (
            <Button small kind="ghost" onClick={() => run(`#${t.ID} outdated`, () => api.OutdateTicket(repo, t.ID))}><Trash2 size={14} /> Outdated</Button>
          )}
        </div>

        <dl className="meta">
          <dt>Branch</dt><dd className="mono">{t.Branch || "–"}</dd>
          <dt>Depends on</dt><dd><EditableText value={(t.DependsOn ?? []).join(",")} rows={1} placeholder="none" onSave={save("depends")} /></dd>
          <dt>Review</dt><dd>{t.ReviewState || "–"} {t.ReviewRounds > 0 && `(round ${t.ReviewRounds})`}</dd>
        </dl>

        <div className="label">Requirements</div>
        <EditableText value={t.Requirements} rows={12} onSave={save("requirements")} />
        <div className="label">Acceptance criteria</div>
        <EditableText value={t.Acceptance} rows={8} onSave={save("acceptance")} />
        {t.ChangeRequest && (<><div className="label">Change request (TechLead)</div><pre className="prose callout">{t.ChangeRequest}</pre></>)}
        {t.ReviewSummary && (<><div className="label">Review (Tester)</div><pre className="prose callout">{t.ReviewSummary}</pre></>)}
      </div>
    </aside>
  );
}
