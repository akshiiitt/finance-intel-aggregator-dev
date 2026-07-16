import { useGetDigest, getGetDigestQueryKey } from "@workspace/api-client-react";
import type { Article } from "@workspace/api-client-react";
import {
  TrendingUp, Globe, Landmark, Briefcase, Cpu, Bitcoin, AlertTriangle, DollarSign,
} from "lucide-react";
import { ArticleCard } from "@/components/shared/ArticleCard";
import { PageHeader } from "@/components/shared/PageHeader";
import { DataState } from "@/components/shared/DataState";
import { DIGEST_MS } from "@/lib/query-config";

const SECTIONS = [
  { key: "funding", label: "Funding radar", icon: DollarSign, token: "chart-1", filter: (a: Article) => a.category === "funding" },
  { key: "markets", label: "Market moves", icon: TrendingUp, token: "chart-3", filter: (a: Article) => a.category === "markets" },
  { key: "policy", label: "Policy & regulatory", icon: Landmark, token: "chart-5", filter: (a: Article) => a.category === "policy" },
  { key: "ipo", label: "IPO watch", icon: Briefcase, token: "chart-2", filter: (a: Article) => a.category === "ipo" },
  { key: "mergers", label: "M&A", icon: Briefcase, token: "chart-4", filter: (a: Article) => a.category === "mergers" },
  { key: "earnings", label: "Earnings", icon: TrendingUp, token: "chart-4", filter: (a: Article) => a.category === "earnings" },
  { key: "startup", label: "Startup corner", icon: Briefcase, token: "chart-2", filter: (a: Article) => a.category === "startup" },
  { key: "technology", label: "Tech", icon: Cpu, token: "chart-6", filter: (a: Article) => a.category === "technology" },
  { key: "crypto", label: "Crypto", icon: Bitcoin, token: "chart-3", filter: (a: Article) => a.category === "crypto" },
  { key: "global", label: "Global signals", icon: Globe, token: "muted-foreground", filter: (a: Article) => a.region === "global" },
] as const;

function DigestSection({
  label, icon: Icon, token, articles,
}: { label: string; icon: typeof DollarSign; token: string; articles: Article[] }) {
  return (
    <section className="mb-7">
      <div className="flex items-center gap-2 mb-3 pb-2.5 border-b border-border">
        <Icon size={13} style={{ color: `hsl(var(--${token}))` }} />
        <h2 className="text-sm font-semibold" style={{ color: `hsl(var(--${token}))` }}>{label}</h2>
        <span className="ml-auto text-2xs text-muted-foreground">{articles.length}</span>
      </div>
      {articles.length === 0 ? (
        <p className="text-sm text-muted-foreground py-2">No stories in this category today.</p>
      ) : (
        <div className="flex flex-col divide-y divide-border/50">
          {articles.map((a, i) => <ArticleCard key={a.id} article={a} variant="compact" index={i} />)}
        </div>
      )}
    </section>
  );
}

export default function Digest() {
  const { data, isLoading, isError, refetch } = useGetDigest({
    query: { queryKey: getGetDigestQueryKey(), refetchInterval: DIGEST_MS },
  });
  const topArticles = data?.topArticles ?? [];

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="max-w-[680px] mx-auto px-5 py-6">
        <PageHeader
          title="Morning digest"
          description={data?.date ?? new Date().toLocaleDateString("en-IN", { weekday: "long", year: "numeric", month: "long", day: "numeric" })}
          live
          className="px-0 py-0 mb-7"
        />

        <DataState
          isLoading={isLoading}
          isError={isError}
          isEmpty={topArticles.length === 0}
          onRetry={() => void refetch()}
          errorTitle="Couldn't load the digest"
          errorDescription="Something went wrong reaching the server."
          emptyTitle="No articles yet today"
          emptyDescription="Check back after the first feed fetch completes."
        >
          {data?.content && (
            <div className="surface-1 p-5 mb-7 text-sm leading-relaxed text-foreground/85 whitespace-pre-line">
              {data.content}
            </div>
          )}
          {SECTIONS.map(({ key, label, icon, token, filter }) => (
            <DigestSection
              key={key}
              label={label}
              icon={icon}
              token={token}
              articles={topArticles.filter(filter).slice(0, key === "global" ? 5 : undefined)}
            />
          ))}
          <div className="mt-6 pt-4 text-center border-t border-border">
            <p className="text-2xs text-muted-foreground">
              {topArticles.length} top articles · FI score ranked · updated continuously
            </p>
          </div>
        </DataState>
      </div>
    </div>
  );
}
