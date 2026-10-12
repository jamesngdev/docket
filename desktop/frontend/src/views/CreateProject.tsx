import { useState } from "react";
import { api, errText } from "../api";
import { project, store } from "../../wailsjs/go/models";
import { Button, Modal } from "../ui";

const FIELDS: { key: string; label: string; hint?: string; def?: string }[] = [
  { key: "Name", label: "Name" },
  { key: "Description", label: "Description" },
  { key: "RepoURL", label: "GitHub repo", hint: "git@github.com:org/repo.git" },
  { key: "LocalPath", label: "Local path", hint: "~/Work/repo — cloned when missing" },
  { key: "DocsRepoURL", label: "Docs repo", hint: "git@github.com:org/docs.git" },
  { key: "Phase", label: "Docs phase", hint: "docs branch, empty = default branch" },
  { key: "BaseBranch", label: "Base branch", def: "develop" },
  { key: "PRTarget", label: "PR target", def: "develop" },
  { key: "MaxParallelAgents", label: "Max agents", def: "3" },
  { key: "techlead", label: "TechLead model", def: "opus" },
  { key: "developer", label: "Developer model", def: "sonnet" },
  { key: "tester", label: "Tester model", def: "sonnet" },
];

/** CreateProject is the new-project wizard (same steps as the TUI form). */
export default function CreateProject({ onClose, onCreated }: { onClose: () => void; onCreated: (repo: string) => void }) {
  const [v, setV] = useState<Record<string, string>>(Object.fromEntries(FIELDS.map((f) => [f.key, f.def ?? ""])));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const submit = async () => {
    setBusy(true);
    setErr("");
    try {
      const settings = new store.Settings({
        max_parallel_agents: Number(v.MaxParallelAgents) || 3, base_branch: v.BaseBranch, pr_target: v.PRTarget,
        max_review_rounds: 2, models: { techlead: v.techlead, developer: v.developer, tester: v.tester },
      });
      const input = new project.Input({
        Name: v.Name, Description: v.Description, RepoURL: v.RepoURL, LocalPath: v.LocalPath,
        DocsRepoURL: v.DocsRepoURL, Phase: v.Phase, Settings: settings,
      });
      onCreated(await api.CreateProject(input));
    } catch (e) {
      setErr(errText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New project" onClose={onClose} wide>
      <div className="form-grid">
        {FIELDS.map((f) => (
          <label key={f.key} className="field">
            <span>{f.label}</span>
            <input value={v[f.key]} placeholder={f.hint} onChange={(e) => setV({ ...v, [f.key]: e.target.value })} />
          </label>
        ))}
      </div>
      {err && <div className="error">{err}</div>}
      <div className="row gap-sm end">
        <Button kind="ghost" onClick={onClose}>Cancel</Button>
        <Button kind="primary" onClick={submit} disabled={busy || !v.Name || !v.LocalPath}>
          {busy ? "Cloning & preparing…" : "Create project"}
        </Button>
      </div>
    </Modal>
  );
}
