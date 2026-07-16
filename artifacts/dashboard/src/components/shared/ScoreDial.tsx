import { cn } from "@/lib/utils";

/**
 * FI score ring. Motion only fires on a real value change (the
 * stroke-dashoffset transition), never ambiently — Craft Principle #9.
 */
export function ScoreDial({ score, size = 32, className }: { score: number | null | undefined; size?: number; className?: string }) {
  if (score === null || score === undefined) return null;

  const r = (size - 6) / 2;
  const circ = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(1, score / 100));
  const dashOffset = circ * (1 - pct);

  // Tone steps through neutral -> primary as the score climbs, using only
  // tokens already in the palette (no new colors introduced for this).
  const color =
    pct >= 0.7 ? "hsl(var(--primary))" :
    pct >= 0.5 ? "hsl(var(--bullish))" :
    pct >= 0.35 ? "hsl(var(--muted-foreground))" :
    "hsl(var(--border))";

  const cx = size / 2;

  return (
    <div
      className={cn("relative shrink-0", className)}
      style={{ width: size, height: size }}
      title={`FI score ${Math.round(score)}/100`}
    >
      <svg width={size} height={size} className="-rotate-90" style={{ overflow: "visible" }}>
        <circle cx={cx} cy={cx} r={r} fill="none" stroke="hsl(var(--border))" strokeWidth={2} />
        <circle
          cx={cx} cy={cx} r={r} fill="none"
          stroke={color} strokeWidth={2}
          strokeDasharray={circ} strokeDashoffset={dashOffset}
          strokeLinecap="round"
          style={{ transition: "stroke-dashoffset 0.7s var(--ease-enter)" }}
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center">
        <span className="tnum text-2xs font-semibold leading-none" style={{ color }}>
          {Math.round(score)}
        </span>
      </div>
    </div>
  );
}
