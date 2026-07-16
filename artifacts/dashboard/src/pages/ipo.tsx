import { useState } from "react";
import { Clock, CheckCircle, AlertCircle, Star } from "lucide-react";
import { useGetIpoCalendar, getGetIpoCalendarQueryKey } from "@workspace/api-client-react";
import type { IpoEntry } from "@workspace/api-client-react";
import { PageHeader } from "@/components/shared/PageHeader";
import { DataState } from "@/components/shared/DataState";
import { DataTable, type DataTableColumn } from "@/components/shared/DataTable";
import { TokenBadge } from "@/components/shared/Badge";
import { IPO_MS } from "@/lib/query-config";

function fmtDate(d: string | null | undefined): string {
  if (!d) return "—";
  try { return new Date(d).toLocaleDateString("en-IN", { day: "2-digit", month: "short" }); }
  catch { return d; }
}

function GmpCell({ gmp, priceBandHigh }: { gmp: number | null | undefined; priceBandHigh: number | null | undefined }) {
  if (!gmp) return <span className="text-muted-foreground">—</span>;
  const pct = priceBandHigh ? ((gmp / priceBandHigh) * 100).toFixed(1) : null;
  const pos = gmp > 0;
  return (
    <div>
      <span className={`tnum font-semibold ${pos ? "text-bullish" : "text-bearish"}`}>{pos ? "+" : ""}{gmp}</span>
      {pct && <div className={`tnum text-2xs ${pos ? "text-bullish/70" : "text-bearish/70"}`}>{pos ? "+" : ""}{pct}%</div>}
    </div>
  );
}

const STATUS_CFG: Record<string, { label: string; token: string; icon: typeof CheckCircle }> = {
  open: { label: "Open", token: "bullish", icon: CheckCircle },
  upcoming: { label: "Upcoming", token: "chart-6", icon: Clock },
  closed: { label: "Closed", token: "muted-foreground", icon: AlertCircle },
  listed: { label: "Listed", token: "chart-5", icon: Star },
};

/** Subscription bar — scaled against 10x as "full" (typical strong retail
 * IPO range), not the previous (x/100)*100 no-op that made a 2x
 * subscription render as a 2%-wide bar regardless of its real magnitude. */
function SubsBar({ x }: { x: number | null | undefined }) {
  if (!x) return <span className="text-muted-foreground">—</span>;
  const pct = Math.min((x / 10) * 100, 100);
  const color = x >= 5 ? "hsl(var(--primary))" : x >= 1 ? "hsl(var(--bullish))" : "hsl(var(--chart-6))";
  return (
    <div>
      <span className="tnum font-semibold" style={{ color }}>{x.toFixed(1)}×</span>
      <div className="h-[3px] w-16 mt-1 rounded-full bg-secondary overflow-hidden">
        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
      </div>
    </div>
  );
}

const TABS = ["open", "upcoming", "closed", "listed"] as const;
type Tab = typeof TABS[number];

export default function IpoCalendarPage() {
  const { data, isLoading, isError, refetch } = useGetIpoCalendar({
    query: { queryKey: getGetIpoCalendarQueryKey(), refetchInterval: IPO_MS },
  });
  const [tab, setTab] = useState<Tab>("open");
  const current = data ? data[tab] : [];

  const columns: DataTableColumn<IpoEntry>[] = [
    {
      key: "company", header: "Company",
      render: ipo => (
        <div className="flex items-center gap-2">
          <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: `hsl(var(--${STATUS_CFG[ipo.status]?.token ?? "muted-foreground"}))` }} />
          <div>
            <div className="font-medium text-sm text-foreground/90">{ipo.companyName}</div>
            <div className="flex items-center gap-1.5 text-2xs text-muted-foreground mt-0.5">
              {ipo.exchange}{ipo.exchange && ipo.sector && " · "}{ipo.sector}
            </div>
          </div>
        </div>
      ),
    },
    {
      key: "priceBand", header: "Price band",
      render: ipo => ipo.priceBandLow && ipo.priceBandHigh ? (
        <div>
          <div className="tnum text-sm text-foreground/80">₹{ipo.priceBandLow}–{ipo.priceBandHigh}</div>
          {ipo.lotSize && <div className="tnum text-2xs text-muted-foreground mt-0.5">Lot {ipo.lotSize}</div>}
        </div>
      ) : <span className="text-muted-foreground">—</span>,
    },
    {
      key: "issueSize", header: "Issue size", align: "right",
      render: ipo => ipo.issueSizeCr ? `₹${ipo.issueSizeCr.toLocaleString()} Cr` : "—",
    },
    {
      key: "dates", header: "Dates",
      render: ipo => (
        <div className="space-y-0.5 text-2xs">
          {ipo.openDate && <div className="text-muted-foreground">Open <span className="tnum text-foreground/70">{fmtDate(ipo.openDate)}</span></div>}
          {ipo.closeDate && <div className="text-muted-foreground">Close <span className="tnum text-foreground/70">{fmtDate(ipo.closeDate)}</span></div>}
          {ipo.listingDate && <div className="tnum text-[hsl(var(--chart-5))]">List {fmtDate(ipo.listingDate)}</div>}
        </div>
      ),
    },
    { key: "gmp", header: "GMP", render: ipo => <GmpCell gmp={ipo.gmp} priceBandHigh={ipo.priceBandHigh} /> },
    { key: "subs", header: "Subs", render: ipo => <SubsBar x={ipo.subscriptionX} /> },
    {
      key: "status", header: "Status",
      render: ipo => {
        const cfg = STATUS_CFG[ipo.status] ?? STATUS_CFG.closed;
        return <TokenBadge token={cfg.token} label={cfg.label} icon={cfg.icon} />;
      },
    },
  ];

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="IPO calendar"
        description="Grey market premium · subscription data · listing dates"
        actions={<span className="text-2xs px-2 py-1 rounded-md border border-border text-muted-foreground">NSE · BSE</span>}
      />

      <div className="flex px-5 shrink-0 border-b border-border">
        {TABS.map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex items-center gap-1.5 px-3.5 py-2.5 text-xs font-medium border-b-2 -mb-px transition-colors capitalize ${
              tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t === "open" ? "Open now" : t}
            {data && data[t].length > 0 && (
              <span className="text-2xs px-1.5 py-0.5 rounded-md bg-secondary text-muted-foreground">{data[t].length}</span>
            )}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto">
        <DataState
          isLoading={isLoading}
          isError={isError}
          isEmpty={current.length === 0}
          onRetry={() => void refetch()}
          errorTitle="Couldn't load the IPO calendar"
          emptyTitle={
            tab === "open" ? "No IPOs currently open" :
            tab === "upcoming" ? "No upcoming IPOs" :
            tab === "closed" ? "No recently closed IPOs" : "No recently listed IPOs"
          }
          emptyDescription=""
        >
          <DataTable columns={columns} data={current} keyFn={ipo => ipo.id} />
          <div className="px-6 py-4 mt-2 border-t border-border">
            <p className="text-2xs text-muted-foreground">
              GMP = grey market premium (unofficial pre-listing price). Not financial advice.
            </p>
          </div>
        </DataState>
      </div>
    </div>
  );
}
