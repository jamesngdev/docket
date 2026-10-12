import { GitBranch, RefreshCw } from "lucide-react";
import { api } from "./api";
import type { Ctx } from "./App";
import { Dot } from "./ui";

/** TopBar is the draggable title bar: project, phase switcher, agents and daemon status. */
export default function TopBar({ ctx, title }: { ctx: Ctx | null; title?: string }) {
  if (!ctx) {
    return <header className="topbar"><span className="topbar-title">{title}</span></header>;
  }
  const { snap, repo, run } = ctx;
  const p = snap.Project;
  const running = (snap.Runs ?? []).filter((r) => r.Status === "running");
  const devs = running.filter((r) => r.Role === "developer").length;
  return (
    <header className="topbar">
      <span className="topbar-title">{p.Name}</span>
      <span className="phase-pill" title="Active phase (docs branch) — switch it in Docs">
        <GitBranch size={13} /> {p.ActivePhase || "default"}
      </span>
      <div className="grow" />
      <span className="topbar-stat">agents <b>{running.length}</b> · dev {devs}/{p.Settings?.max_parallel_agents ?? 3}</span>
      <button className="icon-btn nodrag" title="Pull docs & rescan" onClick={() => run("Docs rescan requested", () => api.Daemon(repo, "rescan", ""))}>
        <RefreshCw size={14} />
      </button>
      <span className="topbar-stat" title={snap.DaemonUp ? "daemon running" : "daemon stopped"}>
        <Dot tone={snap.DaemonUp ? "ok" : "err"} /> daemon
      </span>
    </header>
  );
}
