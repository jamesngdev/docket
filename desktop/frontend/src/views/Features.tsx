import { Check, X } from "lucide-react";
import { api } from "../api";
import type { Ctx } from "../App";
import { Badge, Button, EditableText, Empty } from "../ui";

/** Features: approve / reject drafts, edit title and description inline. */
export default function Features({ ctx }: { ctx: Ctx }) {
  const { snap, repo, run } = ctx;
  const fs = snap.Features ?? [];
  const drafts = fs.filter((f) => f.Status === "draft");
  const tickets = (id: number) => (snap.Tickets ?? []).filter((t) => t.FeatureID === id && t.Status !== "outdated").length;
  if (fs.length === 0) return <Empty title="No features yet" hint="Run TechLead: features from the Dashboard once questions are answered." />;

  return (
    <div className="page">
      <div className="page-head">
        <h1>Features</h1>
        {drafts.length > 0 && (
          <Button kind="primary" onClick={() => run(`${drafts.length} features approved`, async () => {
            for (const f of drafts) await api.SetFeatureStatus(repo, f.ID, "approved");
          })}>Approve all drafts ({drafts.length})</Button>
        )}
      </div>
      <div className="stack">
        {fs.map((f) => (
          <div key={f.ID} className={`card${f.Status === "outdated" ? " faded" : ""}`}>
            <div className="row gap-sm">
              <b className="mono">F{f.ID}</b>
              <div className="grow">
                <EditableText value={f.Title} rows={1} onSave={(v) => run("Title saved", () => api.UpdateFeature(repo, f.ID, v, f.Description))} />
              </div>
              <Badge>{f.Status}</Badge>
              <Badge tone="muted">{tickets(f.ID)} tickets</Badge>
              {f.Status === "draft" && (
                <>
                  <Button small kind="primary" onClick={() => run(`F${f.ID} approved`, () => api.SetFeatureStatus(repo, f.ID, "approved"))}><Check size={14} /> Approve</Button>
                  <Button small kind="danger" onClick={() => run(`F${f.ID} rejected`, () => api.SetFeatureStatus(repo, f.ID, "outdated"))}><X size={14} /> Reject</Button>
                </>
              )}
            </div>
            {f.Documents && <div className="muted small">docs: {f.Documents}</div>}
            <EditableText value={f.Description} rows={8} onSave={(v) => run("Description saved", () => api.UpdateFeature(repo, f.ID, f.Title, v))} />
          </div>
        ))}
      </div>
    </div>
  );
}
