/**
 * Shared Recharts theming — resolves the --chart-1..6 / --bullish / --bearish
 * tokens to real color strings (Recharts can't consume CSS custom properties
 * directly in SVG fill/stroke in all browsers, so we read the computed value
 * once at module load). Replaces the hardcoded neon hex (#10b981, #6366f1,
 * #ef4444...) previously duplicated across analytics.tsx and market-intel.tsx.
 */

function cssVar(name: string): string {
  if (typeof window === "undefined") return "";
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function hslVar(name: string): string {
  const v = cssVar(name);
  return v ? `hsl(${v})` : "#888";
}

type ChartColors = ReturnType<typeof computeChartColors>;
let cached: ChartColors | null = null;

function computeChartColors() {
  return {
    series: [
      hslVar("--chart-1"),
      hslVar("--chart-2"),
      hslVar("--chart-3"),
      hslVar("--chart-4"),
      hslVar("--chart-5"),
      hslVar("--chart-6"),
    ],
    bullish: hslVar("--bullish"),
    bearish: hslVar("--bearish"),
    muted: hslVar("--muted-foreground"),
    border: hslVar("--border"),
    foreground: hslVar("--foreground"),
    card: hslVar("--card"),
    popover: hslVar("--popover"),
  };
}

/**
 * Resolved once and cached — these 13 getComputedStyle reads never change
 * for the lifetime of the app (dark-only, no theme toggle), so recomputing
 * them on every render of every chart-bearing page (Analytics, Market) was
 * pure waste. Safe to cache permanently here: main.tsx imports index.css
 * before the component tree ever mounts, so by the time any component can
 * call this, the stylesheet is already applied — there's no pre-paint call
 * site that would freeze a bad fallback value.
 */
export function chartColors(): ChartColors {
  if (!cached) cached = computeChartColors();
  return cached;
}

/** Shared Recharts axis/grid/tooltip props so every chart looks consistent. */
export function chartAxisProps() {
  const c = chartColors();
  return {
    grid: { stroke: c.border, strokeDasharray: "0" },
    tick: { fill: c.muted, fontSize: 11, fontFamily: "JetBrains Mono, monospace" },
    tooltipStyle: {
      background: c.popover,
      border: `1px solid ${c.border}`,
      borderRadius: 8,
      fontSize: 12,
      color: c.foreground,
    },
  };
}

/** Sentiment-specific colors (direction, not category) — kept separate from
 * the categorical series ramp per Craft Principle: never mix semantic
 * bullish/bearish into the category color rotation. */
export function sentimentColor(sentiment: "bullish" | "bearish" | "neutral" | string | null | undefined): string {
  const c = chartColors();
  if (sentiment === "bullish") return c.bullish;
  if (sentiment === "bearish") return c.bearish;
  return c.muted;
}
