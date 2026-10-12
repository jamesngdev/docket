import { FileText, RotateCcw, Square, TerminalSquare, X } from "lucide-react";
import { api, duration } from "../api";
import type { Ctx } from "../App";
import { Badge, Button, Empty, Section } from "../ui";

/** Agents: tracked runs (TechLead/Developer/Tester) and live chats. */
export default function Agents({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run } = ctx;
  const runs = snap.Runs ?? [];
  const title = (id: number) => (snap.Tickets ?? []).find((t) => t.ID === id)?.Title ?? "";
  const openLog = async (id: string) => api.OpenPath(await api.RunLogPath(repo, id));

  return (
    <div className="page">
      <Section title="Chats">
        {(snap.Chats ?? []).length === 0 && <div className="muted">No chat running. Start one from a question or ticket.</div>}
        {(snap.Chats ?? []).map((c) => (
          <div key={c} className="row gap-sm list-line">
            <span className="mono grow">{c}</span>
            <Button small onClick={() => run("", () => api.OpenInTerminal(c))}><TerminalSquare size={14} /> Open</Button>
            <Button small kind="ghost" onClick={() => run("Chat ended", () => api.EndSession(c))}><X size={14} /> End</Button>
          </div>
        ))}
      </Section>
      <Section title="Runs">
        {runs.length === 0 && <Empty title="No agent runs yet" />}
        <table className="table">
          <thead><tr><th /><th>Run</th><th>Role</th><th>Work</th><th>Time</th><th>Summary</th><th /></tr></thead>
          <tbody>
            {runs.map((r) => {
              const live = r.Status === "running";
              return (
                <tr key={r.ID}>
                  <td><Badge>{r.Status}</Badge></td>
                  <td className="mono small">{r.ID}</td>
                  <td>{r.Role}</td>
                  <td className="ellipsis">{r.TicketID ? `#${r.TicketID} ${title(r.TicketID)}` : r.Task}</td>
                  <td className="mono small">{duration(r.StartedAt, r.EndedAt)}</td>
                  <td className="muted small ellipsis">{r.Summary}</td>
                  <td className="row gap-xs end">
                    {live && <button className="icon-btn" title="Watch in Terminal" onClick={() => run("", () => api.OpenInTerminal(r.Session))}><TerminalSquare size={14} /></button>}
                    {live && <button className="icon-btn" title="Stop" onClick={() => run(`Stopping ${r.ID}`, () => api.Daemon(repo, "stop", r.ID))}><Square size={14} /></button>}
                    {(live || r.Role === "techlead") && <button className="icon-btn" title="Restart" onClick={() => run(`Restarting ${r.ID}`, () => api.Daemon(repo, "restart", r.ID))}><RotateCcw size={14} /></button>}
                    <button className="icon-btn" title="Open full log" onClick={() => run("", () => openLog(r.ID))}><FileText size={14} /></button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Section>
    </div>
  );
}
