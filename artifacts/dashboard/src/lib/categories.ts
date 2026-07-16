import {
  DollarSign, Briefcase, TrendingUp, Landmark, Rocket,
  Building2, BarChart3, Bitcoin, AlertTriangle, Cpu, Circle,
  type LucideIcon,
} from "lucide-react";

/**
 * The single category → { label, chart token, icon } map for the whole app.
 * Previously redefined 3-5 times (dashboard, analytics, entity, digest) with
 * different hex values for the same category — this is the one source of
 * truth. Colors resolve through the --chart-1..6 tokens, not hardcoded hex,
 * so they stay in sync with the theme (Craft Principle #14).
 */
export type CategoryId =
  | "funding" | "ipo" | "markets" | "policy" | "startup"
  | "mergers" | "earnings" | "technology" | "crypto" | "general";

interface CategoryMeta {
  label: string;
  /** CSS var token name, e.g. "chart-1" — resolve via `hsl(var(--${token}))`. */
  token: string;
  icon: LucideIcon;
}

export const CATEGORIES: Record<CategoryId, CategoryMeta> = {
  funding:    { label: "Funding",    token: "chart-1", icon: DollarSign },
  ipo:        { label: "IPO",        token: "chart-2", icon: Briefcase },
  markets:    { label: "Markets",    token: "chart-3", icon: TrendingUp },
  policy:     { label: "Policy",     token: "chart-5", icon: Landmark },
  startup:    { label: "Startup",    token: "chart-2", icon: Rocket },
  mergers:    { label: "M&A",        token: "chart-4", icon: Building2 },
  earnings:   { label: "Earnings",   token: "chart-4", icon: BarChart3 },
  technology: { label: "Technology", token: "chart-6", icon: Cpu },
  crypto:     { label: "Crypto",     token: "chart-3", icon: Bitcoin },
  general:    { label: "General",    token: "muted-foreground", icon: Circle },
};

export function categoryMeta(category: string | null | undefined): CategoryMeta {
  return CATEGORIES[(category as CategoryId) ?? "general"] ?? CATEGORIES.general;
}

/** `hsl(var(--chart-1))` etc. — ready to drop into inline style or Recharts fill/stroke. */
export function categoryColorVar(category: string | null | undefined): string {
  return `hsl(var(--${categoryMeta(category).token}))`;
}
