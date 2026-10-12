import { useState } from "react";
import { CheckCheck, GitPullRequest, Plus, RefreshCw } from "lucide-react";
import { api, Ticket, TICKET_COLUMNS } from "../api";
import type { Ctx } from "../App";
import { Badge, Button } from "../ui";
import TicketDrawer from "./TicketDrawer";

/** Tickets: kanban by status. Drag backlog → pending to approve; click a card for details. */
export default function Tickets({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run } = ctx;
  const [open, setOpen] = useState<number | null>(null);
  const [showOutdated, setShowOutdated] = useState(false);
  const [dragOver, setDragOver] = useState<string | null>(null);
  const tickets = snap.Tickets ?? [];
  const cols = showOutdated ? [...TICKET_COLUMNS, "outdated"] : [...TICKET_COLUMNS];
  const chat = (t: Ticket) => (snap.Chats ?? []).some((c) => c.endsWith(`-chat-t${t.ID}`));
  const newChat = (snap.Chats ?? []).some((c) => c.endsWith("-chat-new"));
  const selected = tickets.find((t) => t.ID === open);

  const drop = (col: string, e: React.DragEvent) => {
    setDragOver(null);
    const id = Number(e.dataTransfer.getData("text/ticket"));
    const t = tickets.find((x) => x.ID === id);
    if (!t) return;
    if (col === "pending" && t.Status === "backlog") run(`#${id} approved`, () => api.ApproveTicket(repo, id));
    else if (col === "outdated" && (t.Status === "backlog" || t.Status === "pending")) run(`#${id} outdated`, () => api.OutdateTicket(repo, id));
  };

  return (
    <div className="board-page">
      <div className="page-head">
        <h1>Tickets</h1>
        <div className="row gap-sm">
          <label className="check small"><input type="checkbox" checked={showOutdated} onChange={(e) => setShowOutdated(e.target.checked)} /> outdated</label>
          <Button small onClick={() => run("Checking PRs for merges & conflicts", () => api.Daemon(repo, "checkprs", ""))}><RefreshCw size={14} /> Check PRs</Button>
          <Button small onClick={() => run("", () => api.ApproveAllTickets(repo))} disabled={snap.Counts.Backlog === 0}><CheckCheck size={14} /> Approve all</Button>
          <Button small kind="primary" onClick={() => run("New-ticket chat opened in Terminal", () => api.StartChat(repo, "new-ticket", 0))}>
            <Plus size={14} /> New ticket {newChat && "💬"}
          </Button>
        </div>
      </div>
      <div className="board">
        {cols.map((col) => {
          const items = tickets.filter((t) => t.Status === col);
          return (
            <div key={col} className={`column${dragOver === col ? " drag-over" : ""}`}
              onDragOver={(e) => { e.preventDefault(); setDragOver(col); }} onDragLeave={() => setDragOver(null)} onDrop={(e) => drop(col, e)}>
              <div className="column-head"><Badge>{col}</Badge><span className="muted small">{items.length}</span></div>
              <div className="column-body">
                {items.map((t) => (
                  <div key={t.ID} className={`ticket-card${open === t.ID ? " selected" : ""}`} draggable
                    onDragStart={(e) => e.dataTransfer.setData("text/ticket", String(t.ID))} onClick={() => setOpen(t.ID)}>
                    <div className="row gap-sm">
                      <span className="mono muted">#{t.ID}</span>
                      <span className="muted small">F{t.FeatureID}</span>
                      <span className="grow" />
                      {t.NeedsUser && <span title="needs you">⚠</span>}
                      {t.PRConflict && <span title="PR conflict">⚔</span>}
                      {chat(t) && <span title="TechLead chat running">💬</span>}
                    </div>
                    <div className="ticket-title">{t.Title}</div>
                    <div className="row gap-sm wrap small">
                      {t.LockedBy && <Badge tone="accent">{t.LockedBy.split("-")[0]}</Badge>}
                      {t.PRURL && <span className="muted"><GitPullRequest size={12} /> {t.PRURL.split("/").pop()}</span>}
                      {t.DependsOn?.length > 0 && t.Status !== "done" && <span className="muted">⇠ {t.DependsOn.join(",")}</span>}
                      {t.ReviewState && <Badge tone={t.ReviewState === "approve" ? "ok" : "warn"}>{t.ReviewState}</Badge>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
      {selected && <TicketDrawer ctx={ctx} t={selected} onClose={() => setOpen(null)} />}
    </div>
  );
}
