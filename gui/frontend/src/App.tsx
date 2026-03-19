import { SettingsWindow } from "@/windows/Settings/Settings";
import { WizardWindow } from "@/windows/Wizard/Wizard";
import { IndicatorWindow } from "@/windows/Indicator";
import { ResultWindow } from "@/windows/Result";
import { UpdateWindow } from "@/windows/Update";

/**
 * App router — renders the correct window based on ?window= query param.
 * Each Wails window loads the same index.html with a different param.
 */
export function App() {
  const params = new URLSearchParams(window.location.search);
  const windowType = params.get("window") ?? "settings";

  switch (windowType) {
    case "settings":
      return <SettingsWindow />;
    case "wizard":
      return <WizardWindow />;
    case "indicator":
      return <IndicatorWindow />;
    case "result":
      return <ResultWindow />;
    case "update":
      return <UpdateWindow />;
    default:
      return <SettingsWindow />;
  }
}
