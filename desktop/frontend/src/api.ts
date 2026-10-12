// Thin typed facade over the generated Wails bindings, plus shared types.
import * as App from "../wailsjs/go/main/App";
import { appsvc, main, store } from "../wailsjs/go/models";

export const api = App;
export type Snapshot = appsvc.Snapshot;
export type ProjectRow = main.ProjectRow;
export type Question = store.Question;
export type Feature = store.Feature;
export type Ticket = store.Ticket;
export type Run = store.Run;
export type Doc = store.Document;
export type ThirdDoc = store.ThirdDoc;

export type View = "dashboard" | "questions" | "features" | "tickets" | "agents" | "docs";

export const TICKET_COLUMNS = ["backlog", "pending", "processing", "wait_review", "done"] as const;

/** errText turns a rejected Wails call into a readable message. */
export function errText(e: unknown): string {
  if (typeof e === "string") return e;
  if (e && typeof e === "object" && "message" in e) return String((e as Error).message);
  return String(e);
}

/** ago formats a Go time (ISO string) as "3m", "2h", "4d". */
export function ago(t: unknown): string {
  const ms = Date.parse(String(t));
  if (!ms || ms < 0) return "";
  const s = Math.max(0, (Date.now() - ms) / 1000);
  if (s < 60) return `${Math.floor(s)}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

/** duration between two Go times (end may be zero = now). */
export function duration(start: unknown, end: unknown): string {
  const a = Date.parse(String(start));
  let b = Date.parse(String(end));
  if (!b || b < a) b = Date.now();
  const m = Math.floor((b - a) / 60000);
  return m < 60 ? `${m}m` : `${Math.floor(m / 60)}h${String(m % 60).padStart(2, "0")}m`;
}
