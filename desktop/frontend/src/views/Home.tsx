import { useEffect, useState } from "react";
import { CheckCircle2, FolderGit2, Plus, Trash2, XCircle } from "lucide-react";
import { api, ProjectRow } from "../api";
import { Badge, Button, Empty, Section } from "../ui";
import CreateProject from "./CreateProject";

const TOOLS: Record<string, string> = {
  claude: "brew install --cask claude-code (then run `claude` once to log in)",
  gh: "brew install gh && gh auth login",
  git: "xcode-select --install",
  tmux: "brew install tmux",
  docker: "install Docker Desktop or OrbStack",
  codegraph: "npm i -g @colbymchenry/codegraph (optional, recommended)",
};

type Props = {
  projects: ProjectRow[];
  onOpen: (repo: string) => void;
  reload: () => void;
  run: (label: string, fn: () => Promise<unknown>) => Promise<void>;
};

/** Home lists projects and checks the machine has every tool agents need. */
export default function Home({ projects, onOpen, reload, run }: Props) {
  const [health, setHealth] = useState<Record<string, boolean>>({});
  const [creating, setCreating] = useState(false);
  useEffect(() => { api.Health().then(setHealth); }, []);
  const missing = Object.entries(health).filter(([, ok]) => !ok);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <h1>Projects</h1>
          <p className="muted">Docs in, pull requests out.</p>
        </div>
        <Button kind="primary" onClick={() => setCreating(true)}><Plus size={14} /> New project</Button>
      </div>

      <Section title="Projects">
        {projects.length === 0 && <Empty title="No projects yet" hint="Create one from a GitHub repo and a docs repo." />}
        <div className="card-grid">
          {projects.map((p) => (
            <div key={p.Repo} className="card clickable" onClick={() => onOpen(p.Repo)}>
              <div className="row gap-sm">
                <FolderGit2 size={16} />
                <b className="grow ellipsis">{p.Name}</b>
                <button className="icon-btn" title="Remove from list (files are kept)"
                  onClick={(e) => { e.stopPropagation(); run("Removed from list", () => api.RemoveProject(p.Repo)).then(reload); }}>
                  <Trash2 size={14} />
                </button>
              </div>
              <div className="muted small ellipsis">{p.Repo}</div>
              {p.Error ? <div className="error small">{p.Error}</div> : (
                <div className="row gap-sm wrap">
                  <Badge tone="info">phase {p.Phase || "default"}</Badge>
                  <Badge tone="ok">{p.Done}/{p.Total} done</Badge>
                  {p.Running > 0 && <Badge tone="accent">{p.Running} agents</Badge>}
                  {p.OpenQ > 0 && <Badge tone="warn">{p.OpenQ} questions</Badge>}
                  {p.NeedsUser > 0 && <Badge tone="warn">{p.NeedsUser} need you</Badge>}
                </div>
              )}
            </div>
          ))}
        </div>
      </Section>

      <Section title="Environment" right={missing.length ? <Badge tone="warn">{missing.length} missing</Badge> : <Badge tone="ok">ready</Badge>}>
        <div className="tool-list">
          {Object.keys(TOOLS).map((t) => (
            <div key={t} className="tool-row">
              {health[t] ? <CheckCircle2 size={15} className="ok" /> : <XCircle size={15} className="err" />}
              <b>{t}</b>
              {!health[t] && <code className="muted small">{TOOLS[t]}</code>}
            </div>
          ))}
        </div>
      </Section>

      {creating && <CreateProject onClose={() => setCreating(false)} onCreated={(r) => { setCreating(false); reload(); onOpen(r); }} />}
    </div>
  );
}
