import { useState, useEffect, useCallback } from "react";
import { goCall, onEvent } from "@/bridge";

/**
 * Hook that loads and caches the app config from Go.
 * Auto-refreshes when the configChanged event fires.
 */
export function useConfig() {
  const [config, setConfig] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    const raw = await goCall("getConfig");
    if (raw) {
      try {
        setConfig(JSON.parse(raw));
      } catch {
        console.error("Failed to parse config");
      }
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    reload();
    const unsub = onEvent("configChanged", reload);
    return unsub;
  }, [reload]);

  return { config, loading, reload };
}
