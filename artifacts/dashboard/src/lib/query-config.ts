/**
 * Named polling-interval constants — replaces ad hoc `refetchInterval: 30_000`
 * literals scattered per-call with no shared source. Also fixes a real
 * duplication: the ambient ticker (layout) and the Market page both poll
 * `useGetMarketData` on the SAME query key but with different intervals
 * (3min vs 5min) — TanStack Query dedupes concurrent fetches for a shared
 * key, but only if every caller agrees on one interval. Both now import
 * MARKET_DATA_MS.
 */
export const WORKER_STATUS_MS = 30_000;
export const ALERTS_MS = 60_000;
export const FEED_STATS_MS = 60_000;
export const FEED_MS = 90_000;
export const TRENDING_MS = 3 * 60_000;
export const ANALYTICS_OVERVIEW_MS = 60_000;
export const ANALYTICS_SECONDARY_MS = 2 * 60_000;
export const MARKET_DATA_MS = 3 * 60_000;
export const IPO_MS = 5 * 60_000;
export const DIGEST_MS = 5 * 60_000;
export const DEALS_MS = 60_000;
