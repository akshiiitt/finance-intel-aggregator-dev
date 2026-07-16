import { formatDistanceToNow } from "date-fns";

/** ₹ crore / $ M/B formatting for deal amounts — previously duplicated inline
 * in dashboard.tsx and funding.tsx with slightly different rounding. */
export function formatAmount(amount: number | null | undefined, currency: string | null | undefined): string {
  if (!amount) return "";
  if (currency === "INR") {
    const cr = amount / 10_000_000;
    if (cr >= 1000) return `₹${(cr / 1000).toFixed(1)}K Cr`;
    return `₹${cr.toFixed(0)} Cr`;
  }
  if (amount >= 1_000_000_000) return `$${(amount / 1_000_000_000).toFixed(1)}B`;
  if (amount >= 1_000_000) return `$${(amount / 1_000_000).toFixed(0)}M`;
  return `$${amount.toLocaleString()}`;
}

export function formatValuation(val: number | null | undefined, currency: string | null | undefined): string {
  if (!val || !currency) return "";
  if (currency === "INR") {
    const cr = val / 10_000_000;
    if (cr >= 1000) return `₹${(cr / 1000).toFixed(1)}K Cr valuation`;
    return `₹${cr.toFixed(0)} Cr valuation`;
  }
  const b = val / 1_000_000_000;
  if (b >= 1) return `$${b.toFixed(1)}B valuation`;
  return `$${(val / 1_000_000).toFixed(0)}M valuation`;
}

export function timeAgo(dt: string | null | undefined): string {
  if (!dt) return "";
  try { return formatDistanceToNow(new Date(dt), { addSuffix: true }); }
  catch { return ""; }
}

export function parseKeyPoints(kp: string | null | undefined): string[] {
  if (!kp) return [];
  try { const p = JSON.parse(kp); return Array.isArray(p) ? p : []; }
  catch { return []; }
}

/** Splits a comma-separated "companies"/"investors" string into a clean list. */
export function splitList(value: string | null | undefined): string[] {
  if (!value) return [];
  return value.split(",").map(v => v.trim()).filter(Boolean);
}
