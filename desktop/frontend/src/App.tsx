import { useCallback, useEffect, useState } from "react";
import { api, errText, ProjectRow, Snapshot, View } from "./api";
import { useRunner, useSnapshot, useToasts } from "./hooks";
import Sidebar from "./Sidebar";
import TopBar from "./TopBar";
import Home from "./views/Home";
import Dashboard from "./views/Dashboard";
import Questions from "./views/Questions";
import Features from "./views/Features";
import Tickets from "./views/Tickets";
import Agents from "./views/Agents";
import Docs from "./views/Docs";

/** Ctx is what every project view receives. */
export type Ctx = {
  repo: string;
  snap: Snapshot;
  reload: () => void;
  run: (label: string, fn: () => Promise<unknown>) => Promise<void>;
  go: (v: View) => void;
};

export default function App() {
  const [projects, setProjects] = useState<ProjectRow[]>([]);
  const [repo, setRepo] = useState<string | null>(null);
  const [view, setView] = useState<View>("dashboard");
  const { toasts, push } = useToasts();
  const { snap, error, reload } = useSnapshot(repo, view === "docs");
  const run = useRunner(push, reload);

  const loadProjects = useCallback(() => {
    api.Projects().then(setProjects).catch((e) => push(errText(e), "err"));
  }, [push]);

  useEffect(() => {
    loadProjects();
    const t = setInterval(loadProjects, 5000);
    return () => clearInterval(t);
  }, [loadProjects]);

  const open = useCallback(async (r: string) => {
    setRepo(r);
    setView("dashboard");
    try {
      await api.OpenProject(r); // starts / upgrades the project daemon
    } catch (e) {
      push(errText(e), "err");
    }
  }, [push]);

  // Deep link (dev + screenshots): ?repo=/path/to/repo&view=tickets
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const r = q.get("repo");
    if (r) {
      open(r).then(() => q.get("view") && setView(q.get("view") as View));
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const ctx: Ctx | null = repo && snap ? { repo, snap, reload, run, go: setView } : null;

  return (
    <div className="app">
      <Sidebar projects={projects} repo={repo} view={view} snap={snap}
        onOpen={open} onView={setView} onHome={() => setRepo(null)} />
      <div className="main">
        <TopBar ctx={ctx} title={repo ? undefined : "Projects"} />
        <div className="content">
          {!repo && <Home projects={projects} onOpen={open} reload={loadProjects} run={run} />}
          {repo && !snap && <div className="loading">{error || "Loading project…"}</div>}
          {ctx && view === "dashboard" && <Dashboard ctx={ctx} />}
          {ctx && view === "questions" && <Questions ctx={ctx} />}
          {ctx && view === "features" && <Features ctx={ctx} />}
          {ctx && view === "tickets" && <Tickets ctx={ctx} />}
          {ctx && view === "agents" && <Agents ctx={ctx} />}
          {ctx && view === "docs" && <Docs ctx={ctx} />}
        </div>
      </div>
      <div className="toasts">
        {toasts.map((t) => <div key={t.id} className={`toast toast-${t.kind}`}>{t.text}</div>)}
      </div>
    </div>
  );
}
