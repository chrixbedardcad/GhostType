import { useState, useEffect } from "react";
import { goCall, openURL } from "@/bridge";

/**
 * About tab — app info, update check, bug report, licenses.
 * Zen: centered, calm, just the essentials.
 */
export function AboutTab() {
  const [version, setVersion] = useState("");
  const [updateStatus, setUpdateStatus] = useState("");
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    goCall("getVersion").then((v) => v && setVersion(v));
  }, []);

  async function checkUpdate() {
    setChecking(true);
    setUpdateStatus("");
    const result = await goCall("checkForUpdate");
    if (result === "up-to-date") {
      setUpdateStatus("You're up to date.");
    } else if (result && result.startsWith("update:")) {
      setUpdateStatus(`Update available: ${result.replace("update:", "")}`);
    } else {
      setUpdateStatus(result ?? "Check failed.");
    }
    setChecking(false);
  }

  return (
    <div className="space-y-8">
      {/* App identity */}
      <div className="text-center py-4">
        <img src="/ghost-icon.png" alt="" className="w-16 h-16 mx-auto mb-4 opacity-80" />
        <h2 className="text-xl font-semibold text-text tracking-tight">GhostSpell</h2>
        {version && <p className="text-sm text-overlay-0 mt-1">v{version}</p>}
        <p className="text-xs text-overlay-0 mt-3">AI-powered text correction and rewriting.</p>

        <div className="flex items-center justify-center gap-4 mt-4 text-xs">
          <button
            onClick={() => openURL("https://github.com/chrixbedardcad/GhostSpell")}
            className="text-accent-blue hover:text-accent-blue/80 transition-colors"
          >
            GitHub
          </button>
          <span className="text-surface-1">·</span>
          <span className="text-accent-green">AGPL-3.0</span>
        </div>
      </div>

      {/* Update check */}
      <section className="bg-surface-0/30 rounded-xl p-5">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-medium text-text">Updates</h3>
            {updateStatus && (
              <p className="text-xs text-overlay-0 mt-1">{updateStatus}</p>
            )}
          </div>
          <button
            onClick={checkUpdate}
            disabled={checking}
            className="px-4 py-1.5 rounded-lg text-xs font-medium
                       bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25
                       disabled:opacity-50 transition-colors"
          >
            {checking ? "Checking..." : "Check for Updates"}
          </button>
        </div>
      </section>

      {/* Open source licenses */}
      <details className="group">
        <summary className="text-xs font-medium text-overlay-0 cursor-pointer
                          flex items-center gap-2 hover:text-subtext-0 transition-colors">
          <span className="text-[10px] transition-transform group-open:rotate-90">▶</span>
          Open Source Licenses
        </summary>
        <div className="mt-3 space-y-3 text-xs text-overlay-1 leading-relaxed">
          <div>
            <strong className="text-subtext-0">llama.cpp</strong> — MIT License
            <br />Local AI inference engine used by GhostSpell Local.
          </div>
          <div>
            <strong className="text-subtext-0">Wails</strong> — MIT License
            <br />Desktop application framework.
          </div>
          <div>
            <strong className="text-subtext-0">Qwen3 / Qwen3.5 Models</strong> — Apache 2.0
            <br />Language models by Alibaba Cloud.
          </div>
          <div>
            <strong className="text-subtext-0">Phi-4 Mini</strong> — MIT License
            <br />Language model by Microsoft.
          </div>
        </div>
      </details>
    </div>
  );
}
