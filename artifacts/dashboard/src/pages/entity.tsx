import { useSearch, useLocation } from "wouter";
import { useState, useMemo, useEffect } from "react";
import { useGetEntityArticles, getGetEntityArticlesQueryKey } from "@workspace/api-client-react";
import { Building2, ArrowLeft, Search } from "lucide-react";
import { ArticleCard } from "@/components/shared/ArticleCard";
import { DataState } from "@/components/shared/DataState";
import { CategoryBadge } from "@/components/shared/Badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export default function EntityPage() {
  const search = useSearch();
  const [, navigate] = useLocation();
  const params = new URLSearchParams(search);
  const nameParam = params.get("name") ?? "";
  const [inputValue, setInputValue] = useState(nameParam);
  const [searchName, setSearchName] = useState(nameParam);

  useEffect(() => {
    setInputValue(nameParam);
    setSearchName(nameParam);
  }, [nameParam]);

  const { data, isLoading, isError, refetch } = useGetEntityArticles(
    { name: searchName, limit: 50 },
    { query: { queryKey: getGetEntityArticlesQueryKey({ name: searchName, limit: 50 }), enabled: searchName.length >= 2, refetchOnWindowFocus: false } }
  );

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = inputValue.trim();
    if (trimmed.length < 2) return;
    setSearchName(trimmed);
    navigate(`/entity?name=${encodeURIComponent(trimmed)}`);
  }

  function goTo(name: string) {
    setInputValue(name);
    setSearchName(name);
    navigate(`/entity?name=${encodeURIComponent(name)}`);
  }

  const articles = data?.articles ?? [];
  const sentimentBreakdown = data?.sentimentBreakdown;
  const topCoMentioned = data?.topCoMentioned ?? [];

  const catBreakdown = useMemo(() => {
    const map = new Map<string, number>();
    for (const a of articles) {
      const cat = a.category ?? "general";
      map.set(cat, (map.get(cat) ?? 0) + 1);
    }
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]).slice(0, 6);
  }, [articles]);

  const totalSentiment = (sentimentBreakdown?.bullish ?? 0) + (sentimentBreakdown?.bearish ?? 0) + (sentimentBreakdown?.neutral ?? 0);

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="max-w-[900px] mx-auto px-5 py-6">
        <form onSubmit={handleSearch} className="flex items-center gap-3 mb-5">
          <button type="button" onClick={() => navigate("/")} className="text-muted-foreground hover:text-foreground transition-colors">
            <ArrowLeft size={16} />
          </button>
          <div className="flex-1 relative">
            <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={inputValue}
              onChange={e => setInputValue(e.target.value)}
              placeholder="Search company or investor — e.g. Zepto, Sequoia, Razorpay"
              className="pl-9"
            />
          </div>
          <Button type="submit" size="sm">Search</Button>
        </form>

        {!searchName ? (
          <div className="surface-1 p-10 text-center">
            <Building2 size={22} className="mx-auto mb-3 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">Enter a company or investor name.</p>
            <p className="text-2xs text-muted-foreground/70 mt-1">Examples: Zepto · Swiggy · Sequoia India · Razorpay · Groww</p>
          </div>
        ) : (
          <DataState
            isLoading={isLoading}
            isError={isError}
            isEmpty={!!data && data.totalMentions === 0}
            onRetry={() => void refetch()}
            errorTitle="Couldn't search"
            emptyTitle={`No results for "${searchName}"`}
            emptyDescription="Try the full company name — data populates as articles are processed."
          >
            {data && data.totalMentions > 0 && (
              <>
                <div className="surface-1 p-4 mb-5">
                  <div className="flex items-start justify-between flex-wrap gap-4">
                    <div>
                      <h1 className="text-xl font-semibold text-foreground mb-1">{data.name}</h1>
                      <div className="flex items-center gap-3 text-2xs text-muted-foreground flex-wrap">
                        <span>{data.totalMentions.toLocaleString()} mentions</span>
                        <span>·</span>
                        <span className="tnum">Avg FI score {data.avgFiScore?.toFixed(1) ?? "0"}</span>
                        {(data.totalFundingCr ?? 0) > 0 && (
                          <>
                            <span>·</span>
                            <span className="tnum text-bullish">₹{data.totalFundingCr?.toLocaleString()} Cr tracked</span>
                          </>
                        )}
                      </div>
                    </div>

                    {sentimentBreakdown && totalSentiment > 0 && (
                      <div className="flex flex-col gap-1 min-w-40">
                        <div className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">Sentiment</div>
                        <div className="flex h-2 rounded-full overflow-hidden gap-px bg-secondary">
                          {(sentimentBreakdown.bullish ?? 0) > 0 && (
                            <div className="bg-bullish h-full" style={{ width: `${(sentimentBreakdown.bullish ?? 0) / totalSentiment * 100}%` }} />
                          )}
                          {(sentimentBreakdown.neutral ?? 0) > 0 && (
                            <div className="bg-muted-foreground h-full" style={{ width: `${(sentimentBreakdown.neutral ?? 0) / totalSentiment * 100}%` }} />
                          )}
                          {(sentimentBreakdown.bearish ?? 0) > 0 && (
                            <div className="bg-bearish h-full" style={{ width: `${(sentimentBreakdown.bearish ?? 0) / totalSentiment * 100}%` }} />
                          )}
                        </div>
                        <div className="tnum flex items-center gap-3 text-2xs">
                          <span className="text-bullish">{sentimentBreakdown.bullish ?? 0} bull</span>
                          <span className="text-muted-foreground">{sentimentBreakdown.neutral ?? 0} neu</span>
                          <span className="text-bearish">{sentimentBreakdown.bearish ?? 0} bear</span>
                        </div>
                      </div>
                    )}
                  </div>

                  {catBreakdown.length > 0 && (
                    <div className="flex items-center gap-2 mt-3 flex-wrap">
                      <span className="text-2xs text-muted-foreground">Coverage:</span>
                      {catBreakdown.map(([cat, cnt]) => (
                        <span key={cat} className="inline-flex items-center gap-1">
                          <CategoryBadge category={cat} />
                          <span className="text-2xs text-muted-foreground">({cnt})</span>
                        </span>
                      ))}
                    </div>
                  )}

                  {topCoMentioned.length > 0 && (
                    <div className="flex items-center gap-2 mt-2 flex-wrap">
                      <span className="text-2xs text-muted-foreground">Co-mentioned:</span>
                      {topCoMentioned.map(e => (
                        <button
                          key={e}
                          onClick={() => goTo(e)}
                          className="text-2xs text-muted-foreground hover:text-foreground border border-border hover:border-foreground/30 px-1.5 py-0.5 rounded-md transition-colors"
                        >
                          {e}
                        </button>
                      ))}
                    </div>
                  )}
                </div>

                <div className="text-2xs text-muted-foreground mb-2 px-1">
                  {articles.length} articles · ranked by FI score
                </div>
                <div className="flex flex-col divide-y divide-border/50">
                  {articles.map(article => <ArticleCard key={article.id} article={article} />)}
                </div>
              </>
            )}
          </DataState>
        )}
      </div>
    </div>
  );
}
