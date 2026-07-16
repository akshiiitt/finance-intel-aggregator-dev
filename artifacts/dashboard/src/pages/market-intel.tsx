import { useGetMarketData, getGetMarketDataQueryKey, useGetMarketHistory, getGetMarketHistoryQueryKey } from "@workspace/api-client-react";
import { TrendingUp, TrendingDown, Minus, RefreshCw } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { PageHeader } from "@/components/shared/PageHeader";
import { DataState } from "@/components/shared/DataState";
import { MetricTile } from "@/components/shared/MetricTile";
import { Sparkline } from "@/components/shared/Sparkline";
import { Button } from "@/components/ui/button";
import { MARKET_DATA_MS } from "@/lib/query-config";

type Snapshot = {
  symbol: string; name: string; exchange: string; price: number;
  changePct: number | null; changeAbs: number | null; prevClose: number | null; capturedAt: string;
};

const INSTRUMENT_GROUPS = [
  { label: "India indices", token: "chart-1", symbols: ["^NSEI", "^BSESN", "^NSEBANK", "NIFTY_IT.NS"] },
  { label: "Global indices", token: "chart-6", symbols: ["^GSPC", "^IXIC", "^DJI"] },
  { label: "Forex", token: "chart-3", symbols: ["USDINR=X", "EURINR=X"] },
  { label: "Commodities", token: "chart-4", symbols: ["GC=F", "CL=F"] },
  { label: "Crypto", token: "chart-5", symbols: ["BTC-USD", "ETH-USD"] },
];

const INSTRUMENT_META: Record<string, { label: string }> = {
  "^NSEI": { label: "Nifty 50" }, "^BSESN": { label: "Sensex" }, "^NSEBANK": { label: "Bank Nifty" },
  "NIFTY_IT.NS": { label: "Nifty IT" }, "^GSPC": { label: "S&P 500" }, "^IXIC": { label: "NASDAQ" },
  "^DJI": { label: "Dow Jones" }, "USDINR=X": { label: "USD/INR" }, "EURINR=X": { label: "EUR/INR" },
  "GC=F": { label: "Gold" }, "CL=F": { label: "Crude oil" }, "BTC-USD": { label: "Bitcoin" }, "ETH-USD": { label: "Ethereum" },
};

function formatPrice(price: number, symbol: string): string {
  if (symbol.includes("INR") || symbol.startsWith("^NSE") || symbol.startsWith("^BSE") || symbol.includes("NIFTY")) {
    return price.toLocaleString("en-IN", { maximumFractionDigits: 0 });
  }
  if (symbol === "BTC-USD" || symbol === "ETH-USD") return price.toLocaleString("en-US", { maximumFractionDigits: 0 });
  if (symbol.includes("INR=X")) return price.toFixed(2);
  return price.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

function InstrumentCard({ snapshot, history, groupToken }: { snapshot: Snapshot; history: number[]; groupToken: string }) {
  const pct = snapshot.changePct ?? 0;
  const abs = snapshot.changeAbs ?? 0;
  const up = pct > 0;
  const flat = pct === 0;
  const meta = INSTRUMENT_META[snapshot.symbol];
  const changeColor = flat ? "text-muted-foreground" : up ? "text-bullish" : "text-bearish";

  return (
    <div className="surface-1 p-3" style={{ borderColor: `hsl(var(--${groupToken}) / 0.15)` }}>
      <div className="flex items-start justify-between mb-2">
        <div>
          <div className="text-xs font-medium text-foreground/80">{meta?.label ?? snapshot.name}</div>
          <div className="text-2xs text-muted-foreground">{snapshot.exchange} · {snapshot.symbol}</div>
        </div>
        <div className="text-right">
          <span className={`tnum text-xs font-semibold flex items-center gap-0.5 justify-end ${changeColor}`}>
            {flat ? <Minus size={9} /> : up ? <TrendingUp size={9} /> : <TrendingDown size={9} />}
            {up && "+"}{pct.toFixed(2)}%
          </span>
          <div className={`tnum text-2xs ${changeColor}`}>{up ? "+" : ""}{abs.toFixed(2)}</div>
        </div>
      </div>

      <div className="flex items-end justify-between">
        <div>
          <div className="tnum text-lg font-semibold text-foreground leading-none">{formatPrice(snapshot.price, snapshot.symbol)}</div>
          {snapshot.prevClose && (
            <div className="tnum text-2xs text-muted-foreground mt-0.5">prev {formatPrice(snapshot.prevClose, snapshot.symbol)}</div>
          )}
        </div>
        <Sparkline data={history} width={64} height={24} up={flat ? undefined : up} />
      </div>
    </div>
  );
}

export default function MarketIntelPage() {
  const { data: marketData, isLoading, isError, dataUpdatedAt, refetch, isFetching } = useGetMarketData({
    query: { queryKey: getGetMarketDataQueryKey(), refetchInterval: MARKET_DATA_MS },
  });
  const { data: historyData } = useGetMarketHistory(
    { hours: 24 },
    { query: { queryKey: getGetMarketHistoryQueryKey({ hours: 24 }), refetchInterval: MARKET_DATA_MS } }
  );

  const snapshots = marketData?.snapshots ?? [];

  const sparklines = new Map<string, number[]>();
  for (const h of historyData?.history ?? []) {
    if (!sparklines.has(h.symbol)) sparklines.set(h.symbol, []);
    sparklines.get(h.symbol)!.push(h.price);
  }

  const snapshotMap = new Map(snapshots.map(s => [s.symbol, s]));
  const updatedAt = marketData?.updatedAt;

  const gainers = snapshots.filter(s => (s.changePct ?? 0) > 0);
  const losers = snapshots.filter(s => (s.changePct ?? 0) < 0);
  const biggestGainer = [...snapshots].sort((a, b) => (b.changePct ?? 0) - (a.changePct ?? 0))[0];
  const biggestLoser = [...snapshots].sort((a, b) => (a.changePct ?? 0) - (b.changePct ?? 0))[0];

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Market intelligence"
        description={updatedAt ? `Updated ${formatDistanceToNow(new Date(updatedAt), { addSuffix: true })}` : undefined}
        live
        actions={
          <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
            <RefreshCw size={12} className={isFetching ? "animate-spin" : ""} /> Refresh
          </Button>
        }
      />

      <div className="flex-1 overflow-y-auto">
        <div className="max-w-[1440px] mx-auto px-5 pb-6 space-y-6">
          <DataState
            isLoading={isLoading}
            isError={isError}
            isEmpty={snapshots.length === 0}
            onRetry={() => void refetch()}
            errorTitle="Couldn't load market data"
            emptyTitle="Market data loading"
            emptyDescription="Snapshots are fetched every few minutes — check back shortly."
          >
            {gainers.length + losers.length > 0 && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
                <MetricTile label="Gainers" value={gainers.length} icon={<TrendingUp size={14} className="text-bullish" />} />
                <MetricTile label="Losers" value={losers.length} icon={<TrendingDown size={14} className="text-bearish" />} />
                {biggestGainer && (
                  <MetricTile
                    label="Best"
                    value={INSTRUMENT_META[biggestGainer.symbol]?.label ?? biggestGainer.name}
                    delta={`+${(biggestGainer.changePct ?? 0).toFixed(2)}%`}
                    deltaDirection="up"
                  />
                )}
                {biggestLoser && (biggestLoser.changePct ?? 0) < 0 && (
                  <MetricTile
                    label="Worst"
                    value={INSTRUMENT_META[biggestLoser.symbol]?.label ?? biggestLoser.name}
                    delta={`${(biggestLoser.changePct ?? 0).toFixed(2)}%`}
                    deltaDirection="down"
                  />
                )}
              </div>
            )}

            {INSTRUMENT_GROUPS.map(group => {
              const groupSnapshots = group.symbols.map(sym => snapshotMap.get(sym)).filter(Boolean) as Snapshot[];
              if (groupSnapshots.length === 0) return null;

              return (
                <div key={group.label} className="mb-6">
                  <div className="flex items-center gap-2 mb-3">
                    <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: `hsl(var(--${group.token}))` }} />
                    <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">{group.label}</span>
                    <div className="flex-1 h-px bg-border" />
                  </div>
                  <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                    {groupSnapshots.map(snapshot => (
                      <InstrumentCard
                        key={snapshot.symbol}
                        snapshot={snapshot}
                        history={sparklines.get(snapshot.symbol) ?? []}
                        groupToken={group.token}
                      />
                    ))}
                  </div>
                </div>
              );
            })}

            <p className="text-2xs text-muted-foreground">
              Sparklines: last 24h from stored snapshots · {historyData?.history.length ?? 0} data points
            </p>
          </DataState>
        </div>
      </div>
    </div>
  );
}
