# FinanceIntel — Documentation

## Welcome

> [!NOTE]
> The code itself is the source of truth for how the system works, and this README is a map to it, not a substitute for it.

---

If you want to understand how the system works **today**, read the code directly — it's a well-organized Go + React monorepo, not a black box:

| Want to understand... | Go read... |
|---|---|
| The overall architecture, data flow | `artifacts/go-backend/internal/worker/scheduler.go` — every worker and its interval, in one file |
| The AI pipeline (free classify + paid enrich split) | `artifacts/go-backend/internal/worker/ai/worker.go` (free pass) and `enrich.go` (paid pass) |
| The ranking algorithm | `artifacts/go-backend/internal/scorer/scorer.go` |
| RSS sources | `artifacts/go-backend/internal/worker/rss/sources.go` |
| The database schema | `artifacts/go-backend/migrations/*.sql` (authoritative) and `lib/db/src/schema/*.ts` (Drizzle mirror) |
| The API routes | `artifacts/go-backend/internal/api/routes/routes.go` |
| The API contract | `lib/api-spec/openapi.yaml` |
| The dashboard pages | `artifacts/dashboard/src/pages/*.tsx` |
| Environment variables | `artifacts/go-backend/.env.example` (documented inline) |
| Deployment | `artifacts/go-backend/docker-compose.yml` |

---

## Tech Stack (current)

| Technology | Used For |
|---|---|
| **Go** (Gin, pgx, gocron, gorilla/websocket, zerolog) | API server + all background workers |
| **PostgreSQL + pgvector** | Storage, plus vector embeddings for semantic dedup/search |
| **Supabase** | Managed Postgres hosting (free tier) |
| **React 19 + Vite + TypeScript** | Dashboard SPA |
| **Tailwind CSS + shadcn/ui + Radix** | Dashboard styling and components |
| **TanStack Query** | Data fetching/caching in the dashboard |
| **Recharts, Framer Motion** | Charts and animation |
| **OpenAPI + Orval** | API contract → generated Zod schemas + React Query hooks |
| **Drizzle ORM** | TypeScript-side schema mirror (see `lib/db`) |
| **Groq + Gemini** | Optional paid AI — entity extraction and summaries on a small, gated slice of the feed only |
| **fastembed (Python sidecar)** | Free, local, unlimited embeddings — semantic dedup, search, niche tagging |
| **Docker Compose** | Local/self-hosted deployment |
| **pnpm workspaces** | Monorepo tooling |

---

## The Three Main Parts

```
FRONTEND                 BACKEND                      DATABASE
(Vercel or static host)  (Go binaries, self-hosted)   (Supabase Postgres + pgvector)

artifacts/                artifacts/go-backend/         Tables:
dashboard/                 cmd/server  — API + scheduler  - raw_items
                           cmd/worker  — workers only      - processed_items
React + Vite                                              - market_snapshots
10 pages                  5+ workers:                     - ipo_calendar
                             RSS, AI (free), Enrich (paid), - alerts / alert_triggers
Served as                    Market, IPO, Alert, Digest     - ai_quotas
static files                                              - daily_digests
                           + embed-sidecar (Python,
Calls API ─────────────►    free local embeddings) ─────► Serves/stores data
via HTTP/WS
```

---

## Where to Start

**If you want to understand the project:** Read `internal/worker/scheduler.go` first — every worker and how often it runs is right there — then follow the "Go read..." table above into whichever piece you're curious about.

**If you want to deploy it:** `artifacts/go-backend/docker-compose.yml` + `.env.example`.

**If something breaks:** Check `.env.example` for required config (note: `API_KEY` is required outside local development) and `internal/worker/scheduler.go` for what should be running.
