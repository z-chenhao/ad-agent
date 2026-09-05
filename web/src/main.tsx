import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./style.css";
import { api, setCSRF } from "./api";

// Consume OAuth query values once, before React mounts or any report requests run.
// The authorization code is never stored in browser storage or application logs.
if (location.pathname === "/model-auth/callback") {
  const query = new URLSearchParams(location.search);
  const code = query.get("code"),
    state = query.get("state");
  history.replaceState(null, "", "/");
  if (code && state) {
    try {
      const auth = await api<{ csrf: string }>("/auth");
      setCSRF(auth.csrf);
      await api("/settings/openrouter/complete", { code, state });
      sessionStorage.setItem("ad-agent.openrouter-connected", "1");
      sessionStorage.setItem("ad-agent.open-settings", "1");
    } catch {
      // No provider response or code is included in the error message.
      window.alert(
        "OpenRouter authorization was not completed. Reopen Settings and connect again.",
      );
    }
  }
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
