import { useState, useRef, useEffect } from "react";
import { motion } from "framer-motion";
import { formatDistanceToNow } from "date-fns";
import { useQueryClient } from "@tanstack/react-query";
import {
  useGetFeed, getGetFeedQueryKey,
  useGetFeedStats, getGetFeedStatsQueryKey,
  useGetTrending, getGetTrendingQueryKey,
} from "@workspace/api-client-react";
import type { Article, FeedStats, TrendingTopic } from "@workspace/api-client-react";
import { RefreshCw, Star, Clock, DollarSign, Search } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ArticleCard } from "@/components/shared/ArticleCard";
import { DataState } from "@/components/shared/DataState";
import { useCommandPalette } from "@/components/shared/CommandPalette";
import { CATEGORIES, categoryMeta, type CategoryId } from "@/lib/categories";
import { FEED_STATS_MS, TRENDING_MS } from "@/lib/query-config";
import { apiUrl } from "@/lib/api-config";
import { sanitizeUrl } from "@/lib/utils";

type CategoryFilter = "all" | CategoryId;

const CATEGORY_FILTERS: { id: CategoryFilter; label: string }[] = [
  { id: "all", label: "All" },
  ...(Object.entries(CATEGORIES) as [CategoryId, typeof CATEGORIES[CategoryId]][])
    .map(([id, meta]) => ({ id, label: meta.label })),
];

const SORT_OPTIONS = [
  { id: "fiscore" as const, label: "Score", icon: Star },
  { id: "recency" as const, label: "Recent", icon: Clock },
  { id: "amount" as const, label: "Amount", icon: DollarSign },
];

/* ── Bento hero: today's briefing + top story + trending ──────────────── */
function BentoHero({
  topArticle, stats, trending, isLoading,
}: {
  topArticle: Article | null;
  stats: FeedStats | null | undefined;
  trending: TrendingTopic[];
  isLoading: boolean;
}) {
  const topCat = Object.entries(stats?.categoryCounts ?? {}).sort((a, b) => b[1] - a[1])[0];
  const briefing = stats && (stats.todayArticles ?? 0) > 0
    ? `${stats.todayArticles} stories tracked today. ${
        topCat ? `${categoryMeta(topCat[0]).label} leads with ${topCat[1]} articles` : "Multiple sectors active"
      }, with ${stats.indiaArticles ?? 0} India signals and ${stats.globalArticles ?? 0} global dispatches.`
    : null;

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-3 p-4 shrink-0 border-b border-border">
      {/* Briefing — the page's one focal element */}
      <div className="md:col-span-2 surface-1 p-5 flex flex-col justify-between min-h-[130px]">
        <div>
          <div className="text-2xs font-medium uppercase tracking-wide text-muted-foreground mb-2.5">
            Today's briefing
          </div>
          {isLoading ? (
            <div className="space-y-2">
              <div className="h-4 shimmer rounded w-3/4" />
              <div className="h-4 shimmer rounded w-1/2" />
            </div>
          ) : briefing ? (
            <p className="text-base leading-relaxed text-foreground/85">{briefing}</p>
          ) : (
            <p className="text-sm text-muted-foreground">Waiting for the first feed fetch — the workers are running.</p>
          )}
        </div>

        <div className="flex items-center gap-7 mt-4 pt-3.5 border-t border-border">
          {[
            { label: "Today", value: stats?.todayArticles, className: "text-primary" },
            { label: "India", value: stats?.indiaArticles, className: "text-bullish" },
            { label: "Global", value: stats?.globalArticles, className: "text-[hsl(var(--chart-6))]" },
          ].map(({ label, value, className }) => (
            <div key={label}>
              <div className={`tnum text-xl font-semibold leading-none ${className}`}>
                {value?.toLocaleString() ?? "—"}
              </div>
              <div className="text-2xs uppercase tracking-wide text-muted-foreground mt-1">{label}</div>
            </div>
          ))}

          {stats?.lastFetch && (
            <div className="ml-auto flex items-center gap-1.5">
              <span className="w-1.5 h-1.5 rounded-full bg-bullish animate-live" />
              <span className="text-2xs text-muted-foreground">
                {formatDistanceToNow(new Date(stats.lastFetch), { addSuffix: true })}
              </span>
            </div>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <div className="surface-1 p-4 flex-1 flex flex-col">
          <div className="text-2xs font-medium uppercase tracking-wide text-muted-foreground mb-2.5">Top story now</div>
          {topArticle ? (
            <>
              <a
                href={sanitizeUrl(topArticle.sourceUrl)}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm leading-snug flex-1 text-foreground/85 hover:text-foreground transition-colors"
              >
                {topArticle.title}
              </a>
              <div className="flex items-center justify-between mt-2.5 pt-2.5 border-t border-border">
                <span className="text-2xs text-muted-foreground">{topArticle.source}</span>
                {topArticle.fiScore != null && (
                  <span className="tnum text-xs font-semibold text-primary/80">FI {Math.round(topArticle.fiScore)}</span>
                )}
              </div>
            </>
          ) : (
            <span className="text-sm text-muted-foreground">Scanning the feed…</span>
          )}
        </div>

        <div className="surface-1 p-4">
          <div className="text-2xs font-medium uppercase tracking-wide text-muted-foreground mb-2.5">Trending, 48h</div>
          <div className="flex flex-wrap gap-1.5">
            {trending.length === 0 ? (
              <span className="text-sm text-muted-foreground">Gathering data…</span>
            ) : trending.slice(0, 7).map((t, i) => (
              <span
                key={t.term}
                className="text-xs rounded-md border border-border px-2 py-0.5 text-muted-foreground hover:text-primary hover:border-primary/30 transition-colors cursor-default"
                style={{ opacity: 1 - i * 0.08 }}
              >
                {t.term}{t.isHot ? " ↑" : ""}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ── Stats strip — reuses the stats already fetched by the page, no
   second useGetFeedStats subscription (previously fetched twice). ──── */
function StatsStrip({ stats, region }: { stats: FeedStats | undefined; region: string }) {
  if (!stats) return null;
  const count = region === "india" ? stats.indiaArticles : region === "global" ? stats.globalArticles : stats.totalArticles;
  const cats = Object.entries(stats.categoryCounts ?? {}).sort((a, b) => b[1] - a[1]).slice(0, 5);

  return (
    <div className="flex items-center gap-3 px-5 py-1.5 text-2xs overflow-x-auto scrollbar-none shrink-0 border-b border-border bg-card/40">
      <span className="text-muted-foreground"><span className="tnum font-semibold text-foreground/80">{(count ?? 0).toLocaleString()}</span> articles</span>
      <span className="text-border">·</span>
      <span className="text-muted-foreground">Today <span className="tnum font-semibold text-primary">{stats.todayArticles}</span></span>
      <span className="text-border">·</span>
      <span className="text-muted-foreground">
        India <span className="tnum font-semibold text-bullish">{stats.indiaArticles}</span>
        <span className="mx-1 text-border">/</span>
        Global <span className="tnum font-semibold text-[hsl(var(--chart-6))]">{stats.globalArticles}</span>
      </span>
      {cats.map(([cat, cnt]) => (
        <span key={cat} className="text-muted-foreground">
          {categoryMeta(cat).label} <span className="tnum font-semibold" style={{ color: `hsl(var(--${categoryMeta(cat).token}))` }}>{cnt}</span>
        </span>
      ))}
      {(stats.unprocessedQueue ?? 0) > 0 && (
        // Static — the page's one live signal is the "last fetch" dot in
        // BentoHero above; a second pulsing dot here would be redundant.
        // Number still gets the same tnum/semibold treatment as every
        // sibling stat in this strip, just without color or motion.
        <span className="ml-auto text-muted-foreground">
          <span className="tnum font-semibold text-foreground/80">{stats.unprocessedQueue}</span> queued
        </span>
      )}
    </div>
  );
}

export default function Dashboard() {
  const [region, setRegion] = useState<"india" | "global" | "both">("india");
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [sort, setSort] = useState<"fiscore" | "recency" | "amount">("fiscore");
  const { openPalette } = useCommandPalette();
  const scrollRef = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();

  const feedParams = { region, ...(category !== "all" ? { category } : {}), sort, limit: 80 };

  const { data, isLoading, isFetching, isError, refetch } = useGetFeed(feedParams, {
    query: { queryKey: getGetFeedQueryKey(feedParams), refetchInterval: false },
  });

  useEffect(() => {
    const sse = new EventSource(apiUrl("/api/feed/stream"));
    let timer: ReturnType<typeof setTimeout>;
    sse.onmessage = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: getGetFeedQueryKey() });
      }, 2000);
    };
    return () => {
      sse.close();
      clearTimeout(timer);
    };
  }, [queryClient]);
  const { data: stats } = useGetFeedStats({
    query: { queryKey: getGetFeedStatsQueryKey(), refetchInterval: FEED_STATS_MS },
  });
  const { data: trendingData } = useGetTrending({
    query: { queryKey: getGetTrendingQueryKey(), refetchInterval: TRENDING_MS },
  });

  const articles = data?.items ?? [];
  const topArticle = articles[0] ?? null;
  const trending = trendingData?.topics ?? [];

  const rowVirtualizer = useVirtualizer({
    count: articles.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 140, // rough height of an ArticleCard
    overscan: 5,
  });

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <BentoHero topArticle={topArticle} stats={stats ?? null} trending={trending} isLoading={isLoading} />

      <div className="shrink-0 border-b border-border">
        <div className="flex items-center gap-1 px-5 pt-2">
          {(["india", "global", "both"] as const).map(r => (
            <button
              key={r}
              onClick={() => { setRegion(r); setCategory("all"); }}
              className={`px-3.5 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors capitalize ${
                region === r ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              {r}
            </button>
          ))}

          <div className="ml-4 flex items-center gap-0.5 pb-1.5">
            {SORT_OPTIONS.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                onClick={() => setSort(id)}
                className={`flex items-center gap-1 px-2.5 py-1 text-xs rounded-md transition-colors ${
                  sort === id ? "text-foreground bg-secondary" : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <Icon size={12} />{label}
              </button>
            ))}
          </div>

          <div className="ml-auto flex items-center gap-2 pb-1.5">
            <button
              onClick={openPalette}
              className="flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-md border border-border text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            >
              <Search size={12} /><span>Search</span>
            </button>
            {isFetching && <RefreshCw size={12} className="animate-spin text-muted-foreground" />}
          </div>
        </div>

        <div className="flex items-center gap-1 px-5 pb-2 pt-1.5 overflow-x-auto scrollbar-none">
          {CATEGORY_FILTERS.map(cat => {
            const isActive = category === cat.id;
            const token = cat.id === "all" ? "primary" : categoryMeta(cat.id).token;
            return (
              <button
                key={cat.id}
                onClick={() => setCategory(cat.id)}
                className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md transition-colors whitespace-nowrap"
                style={{
                  color: isActive ? `hsl(var(--${token}))` : "hsl(var(--muted-foreground))",
                  background: isActive ? "hsl(var(--secondary))" : "transparent",
                }}
              >
                {isActive && cat.id !== "all" && (
                  <span className="w-[5px] h-[5px] rounded-full shrink-0" style={{ background: `hsl(var(--${token}))` }} />
                )}
                {cat.label}
              </button>
            );
          })}
        </div>
      </div>

      <StatsStrip stats={stats} region={region} />

      <div ref={scrollRef} className="flex-1 overflow-y-auto relative">
        <DataState
          isLoading={isLoading}
          isError={isError}
          isEmpty={articles.length === 0}
          onRetry={() => void refetch()}
          loadingSkeleton={
            <div className="flex flex-col items-center justify-center h-40 gap-4">
              <div className="flex gap-1.5">
                {[0, 1, 2, 3].map(i => (
                  <motion.div
                    key={i}
                    className="w-[3px] h-5 rounded-full bg-primary/40"
                    animate={{ scaleY: [0.4, 1.8, 0.4] }}
                    transition={{ duration: 0.9, delay: i * 0.15, repeat: Infinity, ease: "easeInOut" }}
                  />
                ))}
              </div>
              <span className="text-xs text-muted-foreground">Loading feed</span>
            </div>
          }
          errorTitle="Couldn't load the feed"
          errorDescription="The API didn't respond. Check your connection or try again."
          emptyTitle="First fetch in progress"
          emptyDescription="130+ sources are being crawled. Articles appear within about 30 seconds."
        >
          <div
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
              width: "100%",
              position: "relative",
            }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const article = articles[virtualRow.index];
              return (
                <div
                  key={virtualRow.key}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <ArticleCard article={article} rank={virtualRow.index + 1} index={virtualRow.index} />
                </div>
              );
            })}
          </div>
          <div className="px-5 py-5 text-center border-t border-border mt-4">
            <span className="text-2xs text-muted-foreground">
              {articles.length} of {(data?.total ?? articles.length).toLocaleString()} articles
            </span>
          </div>
        </DataState>
      </div>
    </div>
  );
}
