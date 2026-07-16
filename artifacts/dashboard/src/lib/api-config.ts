// Central API/WebSocket origin resolution.
//
// In local dev VITE_API_BASE_URL is unset and Vite's proxy forwards /api to
// the Go backend, so relative paths work. In production (Vercel), the frontend
// and backend are on different origins, so set VITE_API_BASE_URL to the
// backend's HTTPS origin (e.g. https://api.yourdomain.com) at build time.

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

/** Prefix a relative "/api/..." path with the backend origin (no-op in dev). */
export function apiUrl(path: string): string {
  return `${API_BASE_URL}${path}`;
}

/**
 * Build the WebSocket URL for a backend path. Derives ws://→wss:// from the
 * configured backend origin in production; falls back to the current page
 * origin (dev / same-origin) otherwise. Never uses the Vercel page host as the
 * WS host in production — Vercel has no WS endpoint.
 */
export function wsUrl(path: string): string {
  if (API_BASE_URL) {
    return API_BASE_URL.replace(/^http/, "ws") + path; // https→wss, http→ws
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${path}`;
}
