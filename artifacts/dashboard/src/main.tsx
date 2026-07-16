import { createRoot } from "react-dom/client";
import { setBaseUrl } from "@workspace/api-client-react";
import App from "./App";
import { API_BASE_URL } from "@/lib/api-config";

// Self-hosted fonts — only the 3 weights the type system uses (400/500/600;
// never 700 or 300, see Craft Principles). Avoids the render-blocking
// Google Fonts CDN request the app made previously.
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@fontsource/jetbrains-mono/600.css";

import "./index.css";

// Point the generated API client at the backend origin. In dev this is empty
// (Vite proxies /api), so the client keeps using relative paths.
if (API_BASE_URL) {
  setBaseUrl(API_BASE_URL);
}

createRoot(document.getElementById("root")!).render(<App />);
