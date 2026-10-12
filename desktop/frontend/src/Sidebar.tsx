import { Bot, FileText, FolderGit2, HelpCircle, Home, KanbanSquare, LayoutDashboard, Puzzle } from "lucide-react";
import type { ReactNode } from "react";
import { ProjectRow, Snapshot, View } from "./api";
import { Dot } from "./ui";

const NAV: { view: View; label: string; icon: ReactNode }[] = [
  { view: "dashboard", label: "Dashboard", icon: <LayoutDashboard size={15} /> },
  { view: "questions", label: "Questions", icon: <HelpCircle size={15} /> },
  { view: "features", label: "Features", icon: <Puzzle size={15} /> },
  { view: "tickets", label: "Tickets", icon: <KanbanSquare size={15} /> },
  { view: "agents", label: "Agents", icon: <Bot size={15} /> },
  { view: "docs", label: "Docs", icon: <FileText size={15} /> },
];

function badge(view: View, s: Snapshot | null): number {
  if (!s) return 0;
  const c = s.Counts;
  switch (view) {
    case "questions": return c.OpenQ;
    case "features": return c.Drafts;
    case "tickets": return c.Backlog + c.NeedsUser;
    case "agents": return (s.Runs ?? []).filter((r) => r.Status === "running").length;
  }
  return 0;
}

type Props = {
  projects: ProjectRow[];
  repo: string | null;
  view: View;
  snap: Snapshot | null;
  onOpen: (repo: string) => void;
  onView: (v: View) => void;
  onHome: () => void;
};

export default function Sidebar({ projects, repo, view, snap, onOpen, onView, onHome }: Props) {
  return (
    <aside className="sidebar">
      <div className="sidebar-drag" />
      <button className={`nav-item${repo ? "" : " active"}`} onClick={onHome}>
        <Home size={15} /> <span>Projects</span>
      </button>
      <div className="sidebar-label">PROJECTS</div>
      {projects.map((p) => {
        const current = p.Repo === repo;
        return (
          <div key={p.Repo} className="project-block">
            <button className={`project-item${current ? " current" : ""}`} onClick={() => onOpen(p.Repo)} title={p.Repo}>
              <FolderGit2 size={15} />
              <span className="grow ellipsis">{p.Name}</span>
              {p.Running > 0 && <Dot tone="accent" />}
              {(p.OpenQ > 0 || p.NeedsUser > 0) && <Dot tone="warn" />}
            </button>
            {current && (
              <div className="project-nav">
                {NAV.map((n) => {
                  const b = badge(n.view, snap);
                  return (
                    <button key={n.view} className={`nav-item sub${view === n.view ? " active" : ""}`} onClick={() => onView(n.view)}>
                      {n.icon}<span className="grow">{n.label}</span>
                      {b > 0 && <span className="count">{b}</span>}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
      {projects.length === 0 && <div className="sidebar-empty">No projects yet</div>}
    </aside>
  );
}
