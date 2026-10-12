import { useState } from "react";
import { GitBranch, Link2, StickyNote, Trash2 } from "lucide-react";
import { api } from "../api";
import { store } from "../../wailsjs/go/models";
import type { Ctx } from "../App";
import { Badge, Button, Section } from "../ui";

/** Docs: phase (docs branch) switcher, scanned documents, third-party docs. */
export default function Docs({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run } = ctx;
  const active = snap.Project.ActivePhase;
  const [form, setForm] = useState({ Type: "link", Title: "", URL: "", Note: "", Content: "" });

  const addThird = () => run("Third-party doc added", async () => {
    await api.AddThirdDoc(repo, new store.ThirdDoc({ ...form, ID: 0 }));
    setForm({ Type: "link", Title: "", URL: "", Note: "", Content: "" });
  });

  return (
    <div className="page">
      <Section title="Phases (docs branches)">
        <div className="row gap-sm wrap">
          {(snap.Phases ?? []).map((ph) => (
            <button key={ph} className={`phase-chip${ph === active ? " active" : ""}`} disabled={ph === active}
              onClick={() => run(`Switching to ${ph} (checkout + rescan)`, () => api.Daemon(repo, "phase", ph))}>
              <GitBranch size={13} /> {ph}
            </button>
          ))}
          {(snap.Phases ?? []).length === 0 && <span className="muted">Loading branches…</span>}
        </div>
        <p className="muted small">Switching diffs the new phase against the current one, so TechLead only plans what changed.</p>
      </Section>

      <Section title={`Documents · ${(snap.Docs ?? []).length} files`}>
        <div className="doc-list">
          {(snap.Docs ?? []).map((d) => (
            <button key={d.Path} className="doc-row" onClick={() => run("", async () => api.OpenPath(await api.DocPath(repo, d.Path)))}>
              {d.Change ? <Badge tone={d.Change === "D" ? "err" : d.Change === "A" ? "ok" : "warn"}>{d.Change}</Badge> : <span className="badge-space" />}
              <span className="mono grow ellipsis">{d.Path}</span>
            </button>
          ))}
        </div>
      </Section>

      <Section title="Third-party docs & knowledge">
        {(snap.Third ?? []).map((t) => (
          <div key={t.ID} className="row gap-sm list-line">
            {t.Type === "note" ? <StickyNote size={14} /> : <Link2 size={14} />}
            <b>{t.Title}</b>
            {t.URL && <a className="muted small ellipsis grow" onClick={() => api.OpenURL(t.URL)}>{t.URL}</a>}
            {!t.URL && <span className="grow muted small ellipsis">{t.Note || t.Content.slice(0, 80)}</span>}
            <button className="icon-btn" onClick={() => run("Deleted", () => api.DeleteThirdDoc(repo, t.ID))}><Trash2 size={14} /></button>
          </div>
        ))}
        <div className="third-form">
          <select value={form.Type} onChange={(e) => setForm({ ...form, Type: e.target.value })}>
            <option value="link">link</option><option value="note">note</option>
          </select>
          <input placeholder="Title" value={form.Title} onChange={(e) => setForm({ ...form, Title: e.target.value })} />
          {form.Type === "link" && <input placeholder="https://…" value={form.URL} onChange={(e) => setForm({ ...form, URL: e.target.value })} />}
          {form.Type === "note"
            ? <textarea rows={3} placeholder="Knowledge for every agent (conventions, env layout…)" value={form.Content} onChange={(e) => setForm({ ...form, Content: e.target.value })} />
            : <input placeholder="Note (optional)" value={form.Note} onChange={(e) => setForm({ ...form, Note: e.target.value })} />}
          <Button kind="primary" small onClick={addThird} disabled={!form.Title}>Add</Button>
        </div>
      </Section>
    </div>
  );
}
