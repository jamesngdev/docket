import { useEffect, useState } from "react";
import { MessageSquare, SkipForward } from "lucide-react";
import { api, Question } from "../api";
import type { Ctx } from "../App";
import { Badge, Button, Empty } from "../ui";

type Parsed = { intro: string; options: { key: string; text: string }[]; recommend: string };

/** parseQuestion splits "Intro. Chọn: (A) …; (B) …. Đề xuất A" into intro, options and recommendation. */
export function parseQuestion(text: string): Parsed {
  let body = text;
  let recommend = "";
  const rec = body.match(/[.;]?\s*(Đề xuất|Recommend(?:ation|ed)?|Suggest(?:ion|ed)?)\s*:?\s*(.*)$/i);
  if (rec && rec.index !== undefined) {
    recommend = rec[2].trim();
    body = body.slice(0, rec.index);
  }
  const parts = body.split(/\s*\(([A-Za-z0-9])\)\s+/);
  const options: Parsed["options"] = [];
  for (let i = 1; i + 1 < parts.length; i += 2) {
    options.push({ key: parts[i], text: parts[i + 1].replace(/[;.,]\s*$/, "").trim() });
  }
  return { intro: parts[0].trim(), options, recommend };
}

export default function Questions({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run } = ctx;
  const qs = snap.Questions ?? [];
  const [sel, setSel] = useState<number | null>(null);
  const q = qs.find((x) => x.ID === sel) ?? qs[0];
  useEffect(() => { if (q && sel === null) setSel(q.ID); }, [q, sel]);
  if (qs.length === 0) return <Empty title="No questions in this phase" hint="TechLead asks questions after scanning the docs." />;

  return (
    <div className="split">
      <div className="list-pane">
        {qs.map((x) => (
          <button key={x.ID} className={`list-row${x.ID === q?.ID ? " selected" : ""}`} onClick={() => setSel(x.ID)}>
            <div className="row gap-sm">
              <b className="mono">Q{x.ID}</b><Badge>{x.Status}</Badge>
              {(snap.Chats ?? []).some((c) => c.endsWith(`-advisor-q${x.ID}`)) && <span title="chat running">💬</span>}
            </div>
            <div className="clamp-2">{x.Text}</div>
          </button>
        ))}
      </div>
      {q && <QuestionDetail key={q.ID} q={q} repo={repo} run={run} />}
    </div>
  );
}

function QuestionDetail({ q, repo, run }: { q: Question; repo: string; run: Ctx["run"] }) {
  const p = parseQuestion(q.Text);
  const [answer, setAnswer] = useState(q.Answer);
  const save = (text: string) => run(`Q${q.ID} answered`, () => api.AnswerQuestion(repo, q.ID, text));
  return (
    <div className="detail-pane">
      <div className="row gap-sm">
        <h2 className="grow">Question Q{q.ID}</h2>
        <Badge>{q.Status}</Badge>
        <Button small onClick={() => run("Advisor chat opened in Terminal", () => api.StartChat(repo, "question", q.ID))}>
          <MessageSquare size={14} /> Ask Advisor
        </Button>
        <Button small kind="ghost" onClick={() => run(`Q${q.ID} skipped`, () => api.SkipQuestion(repo, q.ID))}>
          <SkipForward size={14} /> Skip
        </Button>
      </div>
      <div className="label">Source</div>
      <ul className="sources">{q.Source.split(";").map((s) => s.trim()).filter(Boolean).map((s) => <li key={s}>{s}</li>)}</ul>
      <div className="label">Question</div>
      <p className="question-text">{p.intro}</p>
      {p.options.length > 0 && (
        <div className="options">
          {p.options.map((o) => {
            const recommended = p.recommend.toUpperCase().startsWith(o.key.toUpperCase());
            return (
              <div key={o.key} className={`option${recommended ? " recommended" : ""}`}>
                <span className="option-key">{o.key}</span>
                <span className="grow">{o.text}</span>
                {recommended && <Badge tone="accent">recommended</Badge>}
                <Button small kind={recommended ? "primary" : "default"} onClick={() => { const t = `(${o.key}) ${o.text}`; setAnswer(t); save(t); }}>
                  Choose
                </Button>
              </div>
            );
          })}
        </div>
      )}
      {p.recommend && p.options.length === 0 && <p className="recommend">→ Recommendation: {p.recommend}</p>}
      <div className="label">Answer</div>
      <textarea rows={5} value={answer} placeholder="Write your answer (or choose an option above)…" onChange={(e) => setAnswer(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) save(answer); }} />
      <div className="row gap-sm end">
        <Button kind="primary" onClick={() => save(answer)} disabled={answer.trim() === q.Answer.trim()}>Save answer ⌘↵</Button>
      </div>
    </div>
  );
}
