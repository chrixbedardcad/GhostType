/**
 * Go ↔ JS bridge for Wails v3.
 *
 * All communication with the Go backend goes through this module.
 * Components never call wails.Call.ByName directly.
 */

declare global {
  interface Window {
    wails: {
      Call: {
        ByName: (method: string, ...args: unknown[]) => Promise<unknown>;
      };
      Events: {
        On: (name: string, callback: (event: { data: unknown }) => void) => () => void;
      };
      Browser: {
        OpenURL: (url: string) => void;
      };
      Window: {
        Close: () => void;
      };
    };
  }
}

const PKG = "github.com/chrixbedardcad/GhostSpell/gui.SettingsService.";

/**
 * Wait for the Wails runtime to be injected into the window.
 * Retries up to 30 times (100ms each = 3s max), matching the old indicator's approach.
 */
async function waitForWails(): Promise<boolean> {
  for (let i = 0; i < 30; i++) {
    if (typeof window.wails !== "undefined" && window.wails.Call) return true;
    await new Promise((r) => setTimeout(r, 100));
  }
  console.error("[bridge] Wails runtime not available after 3s");
  return false;
}

/**
 * Call a Go method on SettingsService.
 * Method name is auto-capitalized: "getConfig" → "GetConfig"
 */
export async function goCall(method: string, ...args: unknown[]): Promise<string | null> {
  try {
    if (!(await waitForWails())) return null;
    const fullMethod = PKG + method.charAt(0).toUpperCase() + method.slice(1);
    const result = await window.wails.Call.ByName(fullMethod, ...args);
    return result as string | null;
  } catch (e) {
    console.error("Bridge call failed:", method, e);
    return null;
  }
}

/**
 * Subscribe to a Wails event. Returns an unsubscribe function.
 * Waits for the Wails runtime to be available before subscribing.
 */
export function onEvent(name: string, callback: (data: unknown) => void): () => void {
  // If wails is already available, subscribe immediately.
  if (typeof window.wails !== "undefined" && window.wails.Events) {
    return window.wails.Events.On(name, (event) => callback(event.data));
  }

  // Otherwise, wait for it asynchronously and subscribe once ready.
  let unsub: (() => void) | null = null;
  let cancelled = false;

  waitForWails().then((ok) => {
    if (cancelled || !ok) return;
    unsub = window.wails.Events.On(name, (event) => callback(event.data));
  });

  return () => {
    cancelled = true;
    if (unsub) unsub();
  };
}

/**
 * Open a URL in the system browser.
 */
export function openURL(url: string): void {
  window.wails.Browser.OpenURL(url);
}

/**
 * Close the current window.
 */
export function closeWindow(): void {
  window.wails.Window.Close();
}
