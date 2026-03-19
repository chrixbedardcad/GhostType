import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

export type Platform = "darwin" | "windows" | "linux" | "";

/**
 * Hook that detects the current platform (darwin, windows, linux).
 */
export function usePlatform(): Platform {
  const [platform, setPlatform] = useState<Platform>("");

  useEffect(() => {
    goCall("getPlatform").then((p) => {
      if (p) setPlatform(p as Platform);
    });
  }, []);

  return platform;
}
