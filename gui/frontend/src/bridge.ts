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
 * Call a Go method on SettingsService.
 * Method name is auto-capitalized: "getConfig" → "GetConfig"
 */
export async function goCall(method: string, ...args: unknown[]): Promise<string | null> {
  try {
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
 */
export function onEvent(name: string, callback: (data: unknown) => void): () => void {
  return window.wails.Events.On(name, (event) => callback(event.data));
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
