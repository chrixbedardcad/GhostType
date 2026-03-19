import { useEffect } from "react";
import { onEvent } from "@/bridge";

/**
 * Hook that subscribes to a Wails event and auto-cleans up.
 */
export function useWailsEvent(name: string, callback: (data: unknown) => void) {
  useEffect(() => {
    const unsub = onEvent(name, callback);
    return unsub;
  }, [name, callback]);
}
