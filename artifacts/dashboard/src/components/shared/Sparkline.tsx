/**
 * Lightweight inline-SVG sparkline — no Recharts/ResponsiveContainer.
 * Market-intel previously mounted a full ResponsiveContainer per
 * instrument card (~13 on screen at once) just to draw a 32x20px line;
 * this is a plain polyline with no layout observers or resize listeners.
 */
export function Sparkline({
  data,
  width = 56,
  height = 20,
  up,
}: {
  data: number[];
  width?: number;
  height?: number;
  up?: boolean;
}) {
  if (!data || data.length < 2) {
    return <div style={{ width, height }} />;
  }

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;
  const step = width / (data.length - 1);

  const points = data
    .map((v, i) => `${i * step},${height - ((v - min) / range) * height}`)
    .join(" ");

  const color = up === undefined
    ? "hsl(var(--muted-foreground))"
    : up ? "hsl(var(--bullish))" : "hsl(var(--bearish))";

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="shrink-0" aria-hidden>
      <polyline points={points} fill="none" stroke={color} strokeWidth={1.25} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
