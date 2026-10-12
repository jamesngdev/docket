import { ArrowRight, Check, Circle, Loader2 } from "lucide-react";
import { api, ago, View } from "../api";
import type { Ctx } from "../App";
import { Badge, Button, Section } from "../ui";

/** Dashboard: pipeline stepper with the one next action, board summary, activity. */
export default function Dashboard({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run, go } = ctx;
  const c = snap.Counts;
  const tl = c.TechleadTask;
  const steps = [
    { name: "Scan", done: c.Scanned, current: !c.Scanned, running: false, detail: `${c.Docs} docs` },
    { name: "Questions", done: c.Scanned && c.OpenQ === 0, current: c.OpenQ > 0, running: tl === "bootstrap" || tl === "docdiff", detail: `${c.AnsweredQ}/${c.AnsweredQ + c.OpenQ} answered` },
    { name: "Features", done: c.Approved > 0 && c.Drafts === 0, current: c.Drafts > 0, running: tl === "features", detail: `${c.Drafts} to approve` },
    { name: "Tickets", done: c.Total > 0 && c.Backlog === 0, current: c.Backlog > 0, running: tl === "tickets", detail: `${c.Backlog} to approve` },
    { name: "Run", done: c.Total > 0 && c.Done === c.Total, current: c.NeedsUser > 0, running: c.Active > 0, detail: `${c.Done}/${c.Total} done` },
  ];
  const next = snap.Next;
  const doNext = () => {
    if (next.Kind === "view") go(next.Arg as View);
    else if (next.Kind === "rescan") run("Docs scan started", () => api.Daemon(repo, "rescan", ""));
    else if (next.Kind === "techlead") run(`TechLead ${next.Arg} started`, () => api.Daemon(repo, "techlead", next.Arg));
  };
  const byStatus = (s: string) => (snap.Tickets ?? []).filter((t) => t.Status === s).length;

  return (
    <div className="page">
      <Section title={`Pipeline · phase ${snap.Project.ActivePhase || "default"}`}>
        <div className="stepper">
          {steps.map((s, i) => (
            <div key={s.name} className={`step${s.current ? " current" : ""}${s.done ? " done" : ""}`}>
              <div className="step-icon">
                {s.running ? <Loader2 size={16} className="spin" /> : s.done ? <Check size={16} /> : <Circle size={16} />}
              </div>
              <div>
                <div className="step-name">{s.name}</div>
                <div className="muted small">{s.detail}</div>
              </div>
              {i < steps.length - 1 && <div className="step-line" />}
            </div>
          ))}
        </div>
        <div className="next-action">
          <span className="muted">Next</span>
          <Button kind="primary" onClick={doNext}>{next.Label} <ArrowRight size={14} /></Button>
        </div>
      </Section>

      <Section title="Board">
        <div className="row gap-md wrap">
          {["backlog", "pending", "processing", "wait_review", "done"].map((s) => (
            <button key={s} className="stat" onClick={() => go("tickets")}>
              <div className="stat-num">{byStatus(s)}</div>
              <Badge>{s}</Badge>
            </button>
          ))}
        </div>
      </Section>

      <Section title="Activity">
        <div className="activity">
          {(snap.Activity ?? []).map((a, i) => (
            <div key={i} className="activity-row">
              <span className="muted small mono">{ago(a.At)}</span>
              <Badge tone="muted">{a.Actor}</Badge>
              <span className="grow">{a.Message}</span>
            </div>
          ))}
          {(snap.Activity ?? []).length === 0 && <div className="muted">No activity yet</div>}
        </div>
      </Section>
    </div>
  );
}
