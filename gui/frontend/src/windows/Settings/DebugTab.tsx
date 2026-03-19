import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

/**
 * Debug tab — logging controls, log viewer, bug report.
 * Zen: functional, no frills. Clear hierarchy.
 */
export function DebugTab() {
  const [debugEnabled, setDebugEnabled] = useState(false);
  const [logPath, setLogPath] = useState("");
  const [status, setStatus] = useState("");
  const [bugDesc, setBugDesc] = useState("");
  const [bugStatus, setBugStatus] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    goCall("getDebugEnabled").then((v) => setDebugEnabled(v === "true"));
    goCall("getDebugLogPath").then((p) => p && setLogPath(p));
  }, []);

  async function toggleDebug(enable: boolean) {
    if (enable) {
      const result = await goCall("enableDebug");
      if (result && !result.startsWith("error")) {
        setDebugEnabled(true);
        setLogPath(result);
      }
    } else {
      await goCall("disableDebug");
      setDebugEnabled(false);
    }
  }

  async function submitBugReport() {
    setSubmitting(true);
    setBugStatus("");
    const result = await goCall("submitBugReport", bugDesc);
    if (result === "ok") {
      setBugStatus("GitHub opened — please review and submit.");
      setBugDesc("");
    } else {
      setBugStatus(result ?? "Failed to submit.");
    }
    setSubmitting(false);
    setTimeout(() => setBugStatus(""), 8000);
  }

  return (
    <div className="space-y-8">
      {/* Debug session */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Debug Session
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Debug Logging</p>
              <p className="text-xs text-overlay-0 mt-0.5">Verbose logging for troubleshooting</p>
            </div>
            <button
              onClick={() => toggleDebug(!debugEnabled)}
              className={`relative w-9 h-5 rounded-full transition-colors ${
                debugEnabled ? "bg-accent-blue" : "bg-surface-1"
              }`}
            >
              <span
                className={`absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform ${
                  debugEnabled ? "translate-x-4" : "translate-x-0.5"
                }`}
              />
            </button>
          </div>

          {logPath && (
            <p className="text-[11px] text-overlay-0 font-mono truncate">{logPath}</p>
          )}

          <div className="flex gap-2">
            <button onClick={() => goCall("openLogFile")}
              className="px-3 py-1.5 rounded-lg text-xs bg-surface-0 text-subtext-0
                         hover:bg-surface-1 transition-colors">
              Open Log
            </button>
            <button onClick={async () => {
              const log = await goCall("tailDebugLog");
              if (log) { navigator.clipboard.writeText(log); setStatus("Copied!"); }
              setTimeout(() => setStatus(""), 2000);
            }}
              className="px-3 py-1.5 rounded-lg text-xs bg-surface-0 text-subtext-0
                         hover:bg-surface-1 transition-colors">
              Copy Log
            </button>
            <button onClick={async () => {
              await goCall("clearDebugLog");
              setStatus("Log cleared.");
              setTimeout(() => setStatus(""), 2000);
            }}
              className="px-3 py-1.5 rounded-lg text-xs bg-surface-0 text-subtext-0
                         hover:bg-surface-1 transition-colors">
              Clear
            </button>
            {status && <span className="text-xs text-accent-green self-center">{status}</span>}
          </div>
        </div>
      </section>

      {/* Bug report */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Report a Bug
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-3">
          <p className="text-xs text-overlay-0">
            Describe the issue below. System info and logs are included automatically.
          </p>
          <textarea
            value={bugDesc}
            onChange={(e) => setBugDesc(e.target.value)}
            placeholder="What happened? What did you expect?"
            className="w-full min-h-[80px] bg-crust border border-surface-0 rounded-lg p-3
                       text-sm text-text placeholder:text-overlay-0 resize-y
                       focus:border-accent-blue/50 focus:outline-none transition-colors"
          />
          <div className="flex items-center gap-3">
            <button
              onClick={submitBugReport}
              disabled={submitting}
              className="px-4 py-1.5 rounded-lg text-xs font-medium
                         bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25
                         disabled:opacity-50 transition-colors"
            >
              {submitting ? "Collecting..." : "Report a Bug"}
            </button>
            {bugStatus && (
              <span className={`text-xs ${bugStatus.includes("GitHub") ? "text-accent-green" : "text-accent-red"}`}>
                {bugStatus}
              </span>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
