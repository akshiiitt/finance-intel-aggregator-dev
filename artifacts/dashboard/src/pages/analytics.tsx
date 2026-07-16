import {
  useGetAnalyticsOverview, getGetAnalyticsOverviewQueryKey,
  useGetFundingAnalytics, getGetFundingAnalyticsQueryKey,
  useGetSentimentAnalytics, getGetSentimentAnalyticsQueryKey,
  useGetTimeline, getGetTimelineQueryKey,
} from "@workspace/api-client-react";
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts";
import { Database, Zap, TrendingUp, DollarSign, Globe, Layers, Building2, Users } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { PageHeader } from "@/components/shared/PageHeader";
import { MetricTile } from "@/components/shared/MetricTile";
import { DataState } from "@/components/shared/DataState";
import { chartColors, chartAxisProps } from "@/lib/chart-theme";
import { categoryMeta } from "@/lib/categories";
import { formatAmount } from "@/lib/format";
import { ANALYTICS_OVERVIEW_MS, ANALYTICS_SECONDARY_MS } from "@/lib/query-config";
import { sanitizeUrl } from "@/lib/utils";

function formatCr(cr: number): string {
  if (cr >= 100000) return `₹${(cr / 100000).toFixed(1)}L Cr`;
  if (cr >= 1000) return `₹${(cr / 1000).toFixed(1)}K Cr`;
  return `₹${cr.toFixed(0)} Cr`;
}

function ChartPanel({ title, sub, isEmpty, emptyLabel = "No data yet — pipeline is running", children }: {
  title: string; sub?: string; isEmpty: boolean; emptyLabel?: string; children: React.ReactNode;
}) {
  return (
    <div className="surface-1 p-4">
      <div className="flex items-center justify-between mb-4">
        <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">{title}</span>
        {sub && <span className="text-2xs text-muted-foreground/70">{sub}</span>}
      </div>
      {isEmpty ? (
        <div className="h-40 flex items-center justify-center text-sm text-muted-foreground">{emptyLabel}</div>
      ) : children}
    </div>
  );
}

function ChartTooltip({ active, payload, label }: { active?: boolean; payload?: Array<{ name: string; value: number; color: string }>; label?: string }) {
  const c = chartColors();
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border px-3 py-2 text-xs" style={{ background: c.popover, borderColor: c.border }}>
      <div className="text-muted-foreground mb-1.5">{label}</div>
      {payload.map((p, i) => (
        <div key={i} className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-sm shrink-0" style={{ background: p.color }} />
          <span className="text-muted-foreground">{p.name}:</span>
          <span className="tnum font-semibold" style={{ color: c.foreground }}>
            {typeof p.value === "number" && p.value > 999 ? p.value.toLocaleString() : p.value}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function AnalyticsPage() {
  const c = chartColors();
  const axis = chartAxisProps();

  const { data: overview, isLoading: loadingOverview, isError: errorOverview, refetch: retryOverview } = useGetAnalyticsOverview({
    query: { queryKey: getGetAnalyticsOverviewQueryKey(), refetchInterval: ANALYTICS_OVERVIEW_MS },
  });
  const { data: funding, isLoading: loadingFunding, isError: errorFunding, refetch: retryFunding } = useGetFundingAnalytics({
    query: { queryKey: getGetFundingAnalyticsQueryKey(), refetchInterval: ANALYTICS_SECONDARY_MS },
  });
  const { data: sentiment, isLoading: loadingSentiment } = useGetSentimentAnalytics({
    query: { queryKey: getGetSentimentAnalyticsQueryKey(), refetchInterval: ANALYTICS_SECONDARY_MS },
  });
  const { data: timeline, isLoading: loadingTimeline } = useGetTimeline({
    query: { queryKey: getGetTimelineQueryKey(), refetchInterval: ANALYTICS_SECONDARY_MS },
  });

  const bySector = funding?.bySector ?? [];
  const byRound = funding?.byRound ?? [];
  const topDeals = funding?.topDeals ?? [];
  const sourceActivity = funding?.sourceActivity ?? [];
  const monthly = funding?.monthly ?? [];
  const daily = timeline?.daily ?? [];
  const sentimentData = sentiment?.byCategory ?? [];

  const sentimentChartData = sentimentData.slice(0, 8).map(d => ({
    category: d.category.charAt(0).toUpperCase() + d.category.slice(1),
    Bullish: d.bullish,
    Neutral: d.neutral,
    Bearish: d.bearish,
  }));

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Intelligence analytics"
        description={overview ? `${overview.totalArticles.toLocaleString()} articles indexed` : undefined}
        live
      />

      <div className="flex-1 overflow-y-auto">
        <div className="max-w-[1440px] mx-auto px-5 pb-6 space-y-5">

          <DataState isLoading={loadingOverview} isError={errorOverview} onRetry={() => void retryOverview()} errorTitle="Couldn't load overview stats">
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
              <MetricTile label="Total articles" value={(overview?.totalArticles ?? 0).toLocaleString()} icon={<Database size={14} />} hero />
              <MetricTile label="Today" value={(overview?.todayArticles ?? 0).toLocaleString()} icon={<Zap size={14} />} />
              <MetricTile label="Deals found" value={(overview?.dealsFound ?? 0).toLocaleString()} icon={<TrendingUp size={14} />} />
              <MetricTile label="Funding tracked" value={formatCr(overview?.totalFundingDiscoveredCr ?? 0)} icon={<DollarSign size={14} />} />
              <MetricTile label="Sources active" value={overview?.sourcesActive24h ?? 0} icon={<Globe size={14} />} />
              <MetricTile label="Avg FI score" value={overview?.avgFiScore ?? 0} icon={<Layers size={14} />} />
            </div>
          </DataState>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <ChartPanel title="Article volume" sub="30 days · India vs global" isEmpty={!loadingTimeline && daily.length === 0}>
              {loadingTimeline ? <div className="h-[180px] shimmer rounded" /> : (
                <ResponsiveContainer width="100%" height={180}>
                  <AreaChart data={daily} margin={{ top: 5, right: 0, left: -20, bottom: 0 }}>
                    <defs>
                      <linearGradient id="indiaGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor={c.series[0]} stopOpacity={0.3} />
                        <stop offset="95%" stopColor={c.series[0]} stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="globalGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor={c.series[5]} stopOpacity={0.3} />
                        <stop offset="95%" stopColor={c.series[5]} stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke={axis.grid.stroke} />
                    <XAxis dataKey="day" tick={axis.tick} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                    <YAxis tick={axis.tick} tickLine={false} axisLine={false} />
                    <Tooltip content={<ChartTooltip />} />
                    <Area type="monotone" dataKey="indiaCount" name="India" stroke={c.series[0]} fill="url(#indiaGrad)" strokeWidth={1.5} dot={false} />
                    <Area type="monotone" dataKey="globalCount" name="Global" stroke={c.series[5]} fill="url(#globalGrad)" strokeWidth={1.5} dot={false} />
                  </AreaChart>
                </ResponsiveContainer>
              )}
            </ChartPanel>

            <ChartPanel title="Monthly funding trend" sub="6 months · ₹Cr equivalent" isEmpty={!loadingFunding && monthly.length === 0}>
              {loadingFunding ? <div className="h-[180px] shimmer rounded" /> : (
                <ResponsiveContainer width="100%" height={180}>
                  <BarChart data={monthly} margin={{ top: 5, right: 0, left: -20, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke={axis.grid.stroke} />
                    <XAxis dataKey="month" tick={axis.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={axis.tick} tickLine={false} axisLine={false} tickFormatter={v => v >= 1000 ? `${(v / 1000).toFixed(0)}K` : v} />
                    <Tooltip content={<ChartTooltip />} />
                    <Bar dataKey="fundingCr" name="Funding (₹Cr)" fill={c.series[0]} radius={[2, 2, 0, 0]} opacity={0.9} />
                    <Bar dataKey="dealCount" name="Deals" fill={c.series[1]} radius={[2, 2, 0, 0]} opacity={0.9} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </ChartPanel>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            <div className="lg:col-span-2">
              <ChartPanel title="Funding by sector" sub="30 days · ₹Cr equivalent" isEmpty={!loadingFunding && bySector.length === 0}>
                {loadingFunding ? <div className="h-[200px] shimmer rounded" /> : (
                  <ResponsiveContainer width="100%" height={200}>
                    <BarChart
                      layout="vertical"
                      data={bySector.slice(0, 8).map(s => ({
                        category: categoryMeta(s.category).label,
                        "Funding ₹Cr": s.totalAmountCr,
                        fill: `hsl(var(--${categoryMeta(s.category).token}))`,
                      }))}
                      margin={{ top: 0, right: 20, left: 10, bottom: 0 }}
                    >
                      <CartesianGrid strokeDasharray="3 3" stroke={axis.grid.stroke} horizontal={false} />
                      <XAxis type="number" tick={axis.tick} tickLine={false} axisLine={false} tickFormatter={v => v >= 1000 ? `${(v / 1000).toFixed(0)}K` : v} />
                      <YAxis dataKey="category" type="category" tick={{ ...axis.tick, fill: c.foreground }} tickLine={false} axisLine={false} width={80} />
                      <Tooltip content={<ChartTooltip />} />
                      <Bar dataKey="Funding ₹Cr" radius={[0, 2, 2, 0]}>
                        {bySector.slice(0, 8).map((s, i) => (
                          <Cell key={i} fill={`hsl(var(--${categoryMeta(s.category).token}))`} />
                        ))}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                )}
              </ChartPanel>
            </div>

            <ChartPanel title="Deals by round type" isEmpty={!loadingFunding && byRound.length === 0} emptyLabel="No data">
              {loadingFunding ? <div className="h-[140px] shimmer rounded" /> : (
                <>
                  <ResponsiveContainer width="100%" height={140}>
                    <PieChart>
                      <Pie data={byRound.slice(0, 8)} cx="50%" cy="50%" innerRadius={35} outerRadius={60} dataKey="count" nameKey="roundType" strokeWidth={0}>
                        {byRound.slice(0, 8).map((_, i) => (
                          <Cell key={i} fill={c.series[i % c.series.length]} opacity={0.9} />
                        ))}
                      </Pie>
                      <Tooltip
                        formatter={(val, name) => [`${val} deals`, name]}
                        contentStyle={{ background: c.popover, border: `1px solid ${c.border}`, borderRadius: 8, fontSize: 12 }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="space-y-1 mt-2">
                    {byRound.slice(0, 6).map((r, i) => (
                      <div key={i} className="flex items-center justify-between text-2xs">
                        <span className="flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-sm shrink-0" style={{ background: c.series[i % c.series.length] }} />
                          <span className="text-muted-foreground">{r.roundType}</span>
                        </span>
                        <span className="tnum text-muted-foreground">{r.count}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </ChartPanel>
          </div>

          <ChartPanel
            title="Market sentiment"
            sub="7 days by category"
            isEmpty={!loadingSentiment && sentimentChartData.length === 0}
          >
            {loadingSentiment ? <div className="h-[140px] shimmer rounded" /> : (
              <>
                <div className="flex items-center gap-3 mb-2">
                  {[["Bullish", c.bullish], ["Neutral", c.muted], ["Bearish", c.bearish]].map(([label, color]) => (
                    <span key={label} className="flex items-center gap-1 text-2xs text-muted-foreground">
                      <span className="w-2 h-2 rounded-sm shrink-0" style={{ background: color }} />{label}
                    </span>
                  ))}
                </div>
                <ResponsiveContainer width="100%" height={140}>
                  <BarChart data={sentimentChartData} margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke={axis.grid.stroke} />
                    <XAxis dataKey="category" tick={axis.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={axis.tick} tickLine={false} axisLine={false} />
                    <Tooltip content={<ChartTooltip />} />
                    <Bar dataKey="Bullish" stackId="s" fill={c.bullish} opacity={0.9} />
                    <Bar dataKey="Neutral" stackId="s" fill={c.muted} opacity={0.9} />
                    <Bar dataKey="Bearish" stackId="s" fill={c.bearish} opacity={0.9} radius={[2, 2, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </>
            )}
          </ChartPanel>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            <div className="lg:col-span-2">
              <ChartPanel title="Top deals by size" sub={`${topDeals.length} deals`} isEmpty={!loadingFunding && topDeals.length === 0} emptyLabel="No deal data yet — pipeline is running">
                {loadingFunding ? <div className="h-40 shimmer rounded" /> : (
                  <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
                    {topDeals.map((deal, i) => (
                      <div key={deal.id} className="flex items-start gap-3 py-2 border-b border-border last:border-0">
                        <span className="tnum text-2xs text-muted-foreground w-5 shrink-0 mt-0.5">#{i + 1}</span>
                        <div className="flex-1 min-w-0">
                          <a href={sanitizeUrl(deal.sourceUrl)} target="_blank" rel="noopener noreferrer" className="text-sm text-foreground/85 hover:text-foreground transition-colors line-clamp-1 font-medium">
                            {deal.title}
                          </a>
                          <div className="flex items-center gap-2 mt-0.5 flex-wrap text-2xs text-muted-foreground">
                            {deal.companies && <span className="flex items-center gap-0.5"><Building2 size={9} />{deal.companies.split(",")[0]?.trim()}</span>}
                            {deal.investors && <span className="flex items-center gap-0.5"><Users size={9} />{deal.investors.split(",")[0]?.trim()}</span>}
                            <span>{deal.source}</span>
                            {deal.publishedAt && <span>{formatDistanceToNow(new Date(deal.publishedAt), { addSuffix: true })}</span>}
                          </div>
                        </div>
                        <div className="flex flex-col items-end gap-1 shrink-0">
                          {deal.amount != null && deal.currency && (
                            <span className="tnum text-2xs font-semibold text-bullish px-1.5 py-0.5 rounded-md border border-bullish/30 bg-bullish/10">
                              {formatAmount(deal.amount, deal.currency)}
                            </span>
                          )}
                          {deal.roundType && <span className="text-2xs text-muted-foreground">{deal.roundType}</span>}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </ChartPanel>
            </div>

            <ChartPanel title="Sources" sub="24h" isEmpty={!loadingFunding && sourceActivity.length === 0} emptyLabel="No data">
              {loadingFunding ? <div className="h-40 shimmer rounded" /> : (
                <div className="space-y-2">
                  {sourceActivity.slice(0, 15).map(s => {
                    const maxCount = sourceActivity[0]?.count ?? 1;
                    const pct = (s.count / maxCount) * 100;
                    return (
                      <div key={s.source}>
                        <div className="flex items-center justify-between text-2xs mb-0.5">
                          <span className="text-muted-foreground truncate max-w-[140px]">{s.source}</span>
                          <div className="flex items-center gap-1.5 tnum text-muted-foreground">
                            <span>{s.count}</span>
                            <span className="text-muted-foreground/60">fi {s.avgFiScore?.toFixed(0) ?? "0"}</span>
                          </div>
                        </div>
                        <div className="h-[3px] bg-secondary rounded-full overflow-hidden">
                          <div className="h-full rounded-full bg-primary/60" style={{ width: `${pct}%` }} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </ChartPanel>
          </div>
        </div>
      </div>
    </div>
  );
}
