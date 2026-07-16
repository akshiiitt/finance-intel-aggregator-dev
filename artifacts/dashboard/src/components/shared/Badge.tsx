import { type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { categoryMeta } from "@/lib/categories";

/**
 * The one pill/badge component for category, sentiment, and status labels.
 * Colors are resolved from CSS custom properties via inline style, never by
 * constructing a Tailwind class string at runtime — that pattern is exactly
 * what silently broke the IPO table's header padding (a build-time-purged
 * dynamic class), so it's banned here structurally, not just by convention.
 *
 * Desaturated per Craft Principle #14: colors come straight from the theme
 * tokens (--chart-*, --bullish, --bearish), never raw Tailwind neon.
 */
export function TokenBadge({
  token,
  label,
  icon: Icon,
  className,
}: {
  token: string; // e.g. "chart-1", "bullish", "bearish", "muted-foreground"
  label: string;
  icon?: LucideIcon;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-2xs font-medium tracking-wide",
        className,
      )}
      style={{
        color: `hsl(var(--${token}))`,
        borderColor: `hsl(var(--${token}) / 0.28)`,
        background: `hsl(var(--${token}) / 0.1)`,
      }}
    >
      {Icon && <Icon size={10} strokeWidth={2} />}
      {label}
    </span>
  );
}

/** Convenience wrapper: category id -> TokenBadge with the shared label/icon/color. */
export function CategoryBadge({ category, className }: { category: string | null | undefined; className?: string }) {
  const meta = categoryMeta(category);
  return <TokenBadge token={meta.token} label={meta.label} icon={meta.icon} className={className} />;
}

/** Sentiment arrow — kept minimal (no pill chrome), matches the existing
 * SentimentMark treatment but tokenized. */
export function SentimentMark({ sentiment }: { sentiment: string | null | undefined }) {
  if (sentiment === "bullish") return <span className="tnum text-xs font-semibold text-bullish">↑</span>;
  if (sentiment === "bearish") return <span className="tnum text-xs font-semibold text-bearish">↓</span>;
  return null;
}
