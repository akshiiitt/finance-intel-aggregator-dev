import { useState } from "react";
import { useGetDeals, getGetDealsQueryKey } from "@workspace/api-client-react";
import { Filter } from "lucide-react";
import { ArticleCard } from "@/components/shared/ArticleCard";
import { PageHeader } from "@/components/shared/PageHeader";
import { DataState } from "@/components/shared/DataState";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { DEALS_MS } from "@/lib/query-config";

const ROUND_TYPES = [
  "All", "Seed", "Pre-Seed", "Series A", "Series B", "Series C",
  "Series D", "Series E", "Series F", "IPO", "M&A", "Debt", "Growth Round",
];
const CATEGORIES = ["All", "funding", "ipo", "mergers", "startup", "general"];
const REGIONS = ["All", "india", "global"];
const SORT_OPTIONS = [
  { value: "amount", label: "Largest deal" },
  { value: "recency", label: "Most recent" },
  { value: "fiscore", label: "Highest FI score" },
];

function FilterSelect({ label, value, onChange, options }: {
  label: string; value: string; onChange: (v: string) => void; options: { value: string; label: string }[];
}) {
  return (
    <div>
      <Label className="text-2xs uppercase tracking-wide text-muted-foreground">{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="mt-1.5"><SelectValue /></SelectTrigger>
        <SelectContent>
          {options.map(o => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  );
}

export default function FundingTrackerPage() {
  const [roundType, setRoundType] = useState("All");
  const [minAmountCr, setMinAmountCr] = useState("");
  const [region, setRegion] = useState("All");
  const [category, setCategory] = useState("All");
  const [sort, setSort] = useState("amount");
  const [offset, setOffset] = useState(0);
  const LIMIT = 25;

  const params = {
    roundType: roundType !== "All" ? roundType : undefined,
    minAmountCr: minAmountCr ? Number(minAmountCr) : undefined,
    region: region !== "All" ? region : undefined,
    category: category !== "All" ? category : undefined,
    sort: sort as "amount" | "recency" | "fiscore",
    limit: LIMIT,
    offset,
  };

  const { data, isLoading, isError, isFetching, refetch } = useGetDeals(params, {
    query: { queryKey: getGetDealsQueryKey(params), refetchInterval: DEALS_MS },
  });

  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  function reset() {
    setRoundType("All"); setMinAmountCr(""); setRegion("All"); setCategory("All"); setSort("amount"); setOffset(0);
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Deal flow tracker"
        description={isFetching ? "Syncing…" : `${total.toLocaleString()} deals`}
        actions={
          <Button variant="outline" size="sm" onClick={reset}>
            <Filter size={12} /> Reset
          </Button>
        }
      />

      <div className="flex-1 overflow-y-auto">
        <div className="max-w-[1200px] mx-auto px-5 pb-6">
          <div className="surface-1 p-3.5 mb-5">
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
              <FilterSelect label="Round type" value={roundType} onChange={v => { setRoundType(v); setOffset(0); }}
                options={ROUND_TYPES.map(r => ({ value: r, label: r }))} />
              <FilterSelect label="Region" value={region} onChange={v => { setRegion(v); setOffset(0); }}
                options={REGIONS.map(r => ({ value: r, label: r === "All" ? "All" : r.charAt(0).toUpperCase() + r.slice(1) }))} />
              <FilterSelect label="Category" value={category} onChange={v => { setCategory(v); setOffset(0); }}
                options={CATEGORIES.map(c => ({ value: c, label: c === "All" ? "All categories" : c.charAt(0).toUpperCase() + c.slice(1) }))} />
              <div>
                <Label htmlFor="min-amount" className="text-2xs uppercase tracking-wide text-muted-foreground">Min amount (₹Cr)</Label>
                <Input
                  id="min-amount" type="number" placeholder="e.g. 100" value={minAmountCr}
                  onChange={e => { setMinAmountCr(e.target.value); setOffset(0); }}
                  className="mt-1.5"
                />
              </div>
              <FilterSelect label="Sort by" value={sort} onChange={v => { setSort(v); setOffset(0); }} options={SORT_OPTIONS} />
            </div>
          </div>

          <DataState
            isLoading={isLoading}
            isError={isError}
            isEmpty={items.length === 0}
            onRetry={() => void refetch()}
            errorTitle="Couldn't load deals"
            emptyTitle="No deals match your filters"
            emptyDescription="Deal data populates as the pipeline processes articles with amounts."
          >
            <div className="flex flex-col divide-y divide-border/50">
              {items.map(article => <ArticleCard key={article.id} article={article} variant="deal" />)}
            </div>

            {total > LIMIT && (
              <div className="flex items-center justify-between mt-5 pt-4 border-t border-border">
                <span className="text-2xs text-muted-foreground">
                  {offset + 1}–{Math.min(offset + LIMIT, total)} of {total.toLocaleString()} deals
                </span>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - LIMIT))}>
                    ← Prev
                  </Button>
                  <Button variant="outline" size="sm" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
                    Next →
                  </Button>
                </div>
              </div>
            )}
          </DataState>
        </div>
      </div>
    </div>
  );
}
