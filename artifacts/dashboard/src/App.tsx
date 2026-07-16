import { lazy, Suspense } from "react";
import { Switch, Route, Router as WouterRouter } from "wouter";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ErrorBoundary } from "@/components/error-boundary";
import NotFound from "@/pages/not-found";
import { Layout } from "@/components/layout";

// Route-level code splitting: each page becomes its own chunk, so the initial
// load only ships the shell + the landing route, not all 10 pages at once.
const Dashboard = lazy(() => import("@/pages/dashboard"));
const Digest = lazy(() => import("@/pages/digest"));
const Workers = lazy(() => import("@/pages/workers"));
const IpoCalendarPage = lazy(() => import("@/pages/ipo"));
const AlertsPage = lazy(() => import("@/pages/alerts"));
const TerminalPage = lazy(() => import("@/pages/terminal"));
const MarketIntelPage = lazy(() => import("@/pages/market-intel"));
const FundingTrackerPage = lazy(() => import("@/pages/funding"));
const EntityPage = lazy(() => import("@/pages/entity"));
const AnalyticsPage = lazy(() => import("@/pages/analytics"));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      // Treat data as fresh for 30s so navigating between pages doesn't
      // refetch immediately on every mount (staleTime defaults to 0).
      staleTime: 30_000,
    },
  },
});

function Router() {
  return (
    <Layout>
      <Suspense fallback={null}>
        <Switch>
          <Route path="/" component={Dashboard} />
          <Route path="/digest" component={Digest} />
          <Route path="/ipo" component={IpoCalendarPage} />
          <Route path="/alerts" component={AlertsPage} />
          <Route path="/market" component={MarketIntelPage} />
          <Route path="/funding" component={FundingTrackerPage} />
          <Route path="/entity" component={EntityPage} />
          <Route path="/analytics" component={AnalyticsPage} />
          <Route path="/workers" component={Workers} />
          <Route path="/terminal" component={TerminalPage} />
          <Route component={NotFound} />
        </Switch>
      </Suspense>
    </Layout>
  );
}

function App() {
  // Force dark mode
  if (typeof document !== "undefined") {
    document.documentElement.classList.add("dark");
  }

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <WouterRouter base={import.meta.env.BASE_URL.replace(/\/$/, "")}>
            <Router />
          </WouterRouter>
          <Toaster />
        </TooltipProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
