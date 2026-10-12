import { useCallback, useEffect, useRef, useState } from "react";
import { api, errText, Snapshot } from "./api";

/** useSnapshot polls the project snapshot every 2s (and on demand via reload). */
export function useSnapshot(repo: string | null, withPhases: boolean) {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [error, setError] = useState("");
  const alive = useRef(true);

  const reload = useCallback(async () => {
    if (!repo) return;
    try {
      const s = await api.Snapshot(repo, withPhases);
      if (alive.current) {
        setSnap(s);
        setError("");
      }
    } catch (e) {
      if (alive.current) setError(errText(e));
    }
  }, [repo, withPhases]);

  useEffect(() => {
    alive.current = true;
    setSnap(null);
    reload();
    const t = setInterval(reload, 2000);
    return () => {
      alive.current = false;
      clearInterval(t);
    };
  }, [reload]);

  return { snap, error, reload };
}

export type Toast = { id: number; text: string; kind: "ok" | "err" };

/** useToasts keeps a short-lived list of notifications. */
export function useToasts() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const push = useCallback((text: string, kind: "ok" | "err" = "ok") => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, text, kind }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), kind === "err" ? 7000 : 3500);
  }, []);
  return { toasts, push };
}

/** run wraps an async action with success/error toasts. */
export function useRunner(push: (t: string, k?: "ok" | "err") => void, after?: () => void) {
  return useCallback(
    async (label: string, fn: () => Promise<unknown>) => {
      try {
        await fn();
        if (label) push(label);
        after?.();
      } catch (e) {
        push(errText(e), "err");
      }
    },
    [push, after],
  );
}
