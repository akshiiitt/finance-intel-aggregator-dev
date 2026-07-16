import { type ReactNode } from "react";
import { TrendingUp, TrendingDown } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * The one stat/metric tile — replaces 5 independent inline implementations
 * (dashboard StatsBar, analytics StatCard, funding filter grid, workers
 * stat tiles, market-intel InstrumentCard) that each had their own color
 * system and type sizing.
 *
 * Accent color appears only on the delta, per Craft Principle #7 — the
 * tile chrome itself never gets tinted.
 */
export function MetricTile({
  label,
  value,
  delta,
  deltaDirection,
  icon,
  hero = false,
  className,
}: {
  label: string;
  value: ReactNode;
  delta?: string;
  deltaDirection?: "up" | "down" | "flat";
  icon?: ReactNode;
  /** Hero tiles use the 3xl size — at most one per page (Craft Principle #11). */
  hero?: boolean;
  className?: string;
}) {
  const deltaColor =
    deltaDirection === "up" ? "text-bullish" :
    deltaDirection === "down" ? "text-bearish" :
    "text-muted-foreground";
  const DeltaIcon = deltaDirection === "up" ? TrendingUp : deltaDirection === "down" ? TrendingDown : null;

  return (
    <div className={cn("surface-1 p-5", className)}>
      <div className="flex items-center justify-between">
        <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
        {icon && <span className="text-muted-foreground/70">{icon}</span>}
      </div>
      <div className="mt-2 flex items-baseline gap-2">
        <span className={cn("tnum font-semibold leading-none text-foreground", hero ? "text-3xl" : "text-2xl")}>
          {value}
        </span>
        {delta && (
          <span className={cn("tnum flex items-center gap-0.5 text-xs font-medium", deltaColor)}>
            {DeltaIcon && <DeltaIcon size={12} />}
            {delta}
          </span>
        )}
      </div>
    </div>
  );
}
