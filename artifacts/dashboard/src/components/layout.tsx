import { type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import {
  Radio, BookOpen, Zap, TrendingUp, TrendingDown, Minus, Bell, DollarSign,
  BarChart2, Activity, Search, Command as CommandIcon, Calendar,
} from "lucide-react";
import { useGetMarketData, getGetMarketDataQueryKey } from "@workspace/api-client-react";
import { MARKET_DATA_MS } from "@/lib/query-config";
import { CommandPaletteProvider, useCommandPalette } from "@/components/shared/CommandPalette";
import {
  SidebarProvider, Sidebar, SidebarHeader, SidebarContent, SidebarFooter,
  SidebarGroup, SidebarGroupLabel, SidebarGroupContent, SidebarMenu, SidebarMenuItem,
  SidebarMenuButton, SidebarInset, SidebarTrigger, SidebarSeparator,
} from "@/components/ui/sidebar";

const TICKER_ORDER = [
  "^NSEI", "^BSESN", "^NSEBANK", "NIFTY_IT.NS", "^GSPC", "^IXIC", "^DJI",
  "USDINR=X", "EURINR=X", "GC=F", "CL=F", "BTC-USD", "ETH-USD",
];

function formatPrice(price: number, symbol: string): string {
  if (symbol.startsWith("^NSE") || symbol.startsWith("^BSE") || symbol.includes("NIFTY"))
    return price.toLocaleString("en-IN", { maximumFractionDigits: 0 });
  if (symbol === "BTC-USD" || symbol === "ETH-USD")
    return price.toLocaleString("en-US", { maximumFractionDigits: 0 });
  if (symbol === "GC=F" || symbol === "CL=F") return price.toFixed(2);
  return price.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

/** Slim ambient market strip — a real finance-terminal signal, not
 * decoration (Craft Principle #9: the live dot is the one surviving
 * ambient loop). Hover is a plain CSS class now, not a DOM style mutation. */
function AmbientTicker() {
  const { data } = useGetMarketData({
    query: { queryKey: getGetMarketDataQueryKey(), refetchInterval: MARKET_DATA_MS },
  });
  const snapshots = data?.snapshots ?? [];

  const sorted = snapshots.length
    ? [...snapshots].sort((a, b) => {
        const ai = TICKER_ORDER.indexOf(a.symbol);
        const bi = TICKER_ORDER.indexOf(b.symbol);
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
      })
    : [];

  return (
    <div className="h-7 flex items-stretch overflow-x-auto scrollbar-none select-none border-b border-border bg-card/60">
      <div className="flex items-center gap-1.5 px-4 shrink-0 border-r border-border">
        <span className="w-1.5 h-1.5 rounded-full bg-bullish animate-live" />
        <span className="text-2xs font-medium text-muted-foreground tracking-wide">Live</span>
      </div>

      {sorted.length === 0 ? (
        <div className="flex items-center px-5">
          <span className="text-2xs text-muted-foreground">Connecting to market data…</span>
        </div>
      ) : sorted.map((s) => {
        const pct = s.changePct ?? 0;
        const up = pct > 0;
        const flat = Math.abs(pct) < 0.005;
        const changeClass = flat ? "text-muted-foreground" : up ? "text-bullish" : "text-bearish";
        const Icon = flat ? Minus : up ? TrendingUp : TrendingDown;
        return (
          <div
            key={s.symbol}
            className="flex items-center gap-2.5 px-4 shrink-0 border-r border-border hover:bg-secondary/50 transition-colors"
          >
            <span className="text-2xs text-muted-foreground">{s.name}</span>
            <span className="tnum text-xs font-medium text-foreground/80">{formatPrice(s.price, s.symbol)}</span>
            <span className={`tnum flex items-center gap-0.5 text-2xs ${changeClass}`}>
              <Icon size={9} />
              {up && "+"}{pct.toFixed(2)}%
            </span>
          </div>
        );
      })}
    </div>
  );
}

interface NavLink { href: string; label: string; icon: typeof Radio }
const NAV_GROUPS: { label: string; items: NavLink[] }[] = [
  { label: "News", items: [
    { href: "/", label: "Feed", icon: Radio },
    { href: "/digest", label: "Digest", icon: BookOpen },
    { href: "/terminal", label: "Live", icon: Zap },
  ]},
  { label: "Markets", items: [
    { href: "/market", label: "Markets", icon: TrendingUp },
  ]},
  { label: "Deals", items: [
    { href: "/funding", label: "Funding", icon: DollarSign },
    { href: "/ipo", label: "IPO", icon: Calendar },
    { href: "/analytics", label: "Analytics", icon: BarChart2 },
  ]},
  { label: "Monitor", items: [
    { href: "/alerts", label: "Alerts", icon: Bell },
  ]},
];

export function Layout({ children }: { children: ReactNode }) {
  return (
    <CommandPaletteProvider>
      <LayoutShell>{children}</LayoutShell>
    </CommandPaletteProvider>
  );
}

function LayoutShell({ children }: { children: ReactNode }) {
  const [location] = useLocation();
  const { openPalette } = useCommandPalette();

  function isActive(href: string) {
    return href === "/" ? location === "/" : location.startsWith(href);
  }

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon">
        <SidebarHeader>
          <Link href="/" className="flex items-center gap-2 px-2 py-1.5">
            <div className="w-6 h-6 flex items-center justify-center rounded-md bg-primary/15 border border-primary/25 shrink-0">
              <Zap size={13} className="text-primary" />
            </div>
            <span className="font-semibold text-sm tracking-tight text-foreground truncate group-data-[collapsible=icon]:hidden">
              FinanceIntel
            </span>
          </Link>
        </SidebarHeader>

        <SidebarContent>
          {NAV_GROUPS.map(group => (
            <SidebarGroup key={group.label}>
              <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {group.items.map(({ href, label, icon: Icon }) => (
                    <SidebarMenuItem key={href}>
                      <SidebarMenuButton asChild isActive={isActive(href)} tooltip={label}>
                        <Link href={href}>
                          <Icon />
                          <span>{label}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ))}
        </SidebarContent>

        {/* Ops tooling — visually separated from consumer nav (Phase C). */}
        <SidebarFooter>
          <SidebarSeparator className="mb-1" />
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild isActive={isActive("/workers")} tooltip="Workers">
                <Link href="/workers">
                  <Activity />
                  <span>Workers</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      </Sidebar>

      <SidebarInset>
        <div className="h-11 flex items-center gap-2 px-3 border-b border-border shrink-0">
          <SidebarTrigger />
          <button
            onClick={openPalette}
            className="flex items-center gap-2 px-2.5 py-1.5 rounded-md border border-border text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors text-sm ml-1"
          >
            <Search size={13} />
            <span className="hidden sm:inline">Search or jump to…</span>
            <span className="hidden sm:flex items-center gap-0.5 ml-2 text-2xs text-muted-foreground/70">
              <CommandIcon size={10} />K
            </span>
          </button>
        </div>

        <AmbientTicker />

        <div className="flex-1 overflow-hidden flex flex-col">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
