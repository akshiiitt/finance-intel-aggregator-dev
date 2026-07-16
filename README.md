# India News Aggregator

A high-performance financial and general news aggregation platform that fetches RSS feeds, classifies content, calculates custom relevance scores, and performs local semantic deduplication.

---

## Description

The India News Aggregator is a production-grade system designed to capture, enrich, index, and visualize financial and business news in real-time. The application consists of a modular Go backend that manages scheduling workers, an offline Python sidecar container that generates text embeddings using fastembed, a PostgreSQL database with the pgvector extension, and an interactive React 19 single-page dashboard.

---

## Features

- **Automated RSS Ingestion**: Multi-feed parsing scheduler utilizing gocron and gofeed.
- **Relevance Scoring**: A custom scoring engine calculates article relevance based on keyword density, source credibility, and publication freshness.
- **AI Classification**: Dual-phase pipeline that filters junk using keyword structures (free) and fetches rich entities and summaries using Groq/Gemini APIs (paid) for high-value articles.
- **Semantic Deduplication**: Integrates an offline Python embedding sidecar to group and deduplicate near-identical articles based on cosine similarity of title/content vectors.
- **Interactive Control Center**: Dashboard featuring charts, news timelines, active alert toggles, market snapshot trends, and real-time WebSocket feed updates.
- **Data Querying**: Fast search using pgvector vector indexing for semantic search queries.

---

## Screenshots

*(Placeholder: Add application dashboard screenshots showing the live feed, analytics charts, and alert panel)*

---

## Tech Stack

- **Backend**: Go (Gin, pgx, gocron, gorilla/websocket, zerolog)
- **Frontend**: React 19, Vite, TypeScript, Tailwind CSS, Radix UI primitives, Recharts, Framer Motion
- **Database**: PostgreSQL (with pgvector), Supabase
- **Machine Learning**: Python, FastEmbed, ONNX Runtime
- **API Spec**: OpenAPI Specification, Orval for automatic types and query hook generation
- **Orchestration**: Docker Compose, pnpm workspaces

---

## Configuration & Environment Variables

Create a `.env` file in `artifacts/go-backend/` based on the `.env.example` file.

Key Environment Variables:
- `ENVIRONMENT`: Set to `production` for strict authorization and performance, or `development` to enable debug features.
- `PORT`: Port the Go backend API runs on (default: `8080`).
- `API_KEY`: Authentication key required for mutating API routes and worker executions in production.
- `DATABASE_URL`: PostgreSQL connection URI.
- `DATABASE_MIGRATE_URL`: PostgreSQL connection URI with SSL options enabled for golang-migrate tasks.
- `EMBED_SIDECAR_URL`: URL of the Python embedding sidecar container (default: `http://embed-sidecar:8900`).
- `CORS_ALLOWED_ORIGINS`: Comma-separated list of browser domains allowed to consume the API.

---

## Installation

Install workspace dependencies using `pnpm`:
```bash
pnpm install
```

---

## Running Locally

To run the entire system (Database, Migrations, Embedding sidecar, and Go API) locally:

1. Navigate to the Go backend folder:
   ```bash
   cd artifacts/go-backend
   ```
2. Build and launch the services:
   ```bash
   docker compose up -d --build
   ```

---

## Build Instructions

### Compile Backend (Go)
To compile the Go binary manually:
```bash
cd artifacts/go-backend
go build -o server ./cmd/server
```

### Build Frontend (React)
To build the static assets for the React application:
```bash
cd artifacts/dashboard
npm run build
```

---

## Deployment

### Frontend Deployment
The React frontend in `artifacts/dashboard` is pre-configured for **Vercel** with a custom `vercel.json` file.
1. Connect your repository to Vercel.
2. Set the root directory to `artifacts/dashboard`.
3. Set the framework preset to `Vite`.
4. Add the `VITE_API_BASE_URL` environment variable pointing to your deployed API domain.

### Backend Deployment
The Go backend, migrations, and Python embedding sidecar are packaged as Docker services. Deploy them to any cloud provider instance by installing Docker and running:
```bash
docker compose up -d --build
```
Ensure you set up Nginx, Caddy, or an equivalent reverse proxy to enable SSL (HTTPS) for API calls.

---

## Roadmap

- Implement custom news source RSS entry forms in the admin dashboard panel.
- Add support for custom Webhook notifications for matching keywords.
- Support advanced semantic search filtering (by source type, category, or exact entity match).
- Integrate additional local LLMs for offline article summary generation.

---

## Contributing

1. Fork the repository.
2. Create a feature branch.
3. Submit a Pull Request.

---

## License

All Rights Reserved. Copyright (c) 2026 Akshit. This project is proprietary and confidential. No part of this software may be copied, modified, distributed, or used without explicit permission.

---

## Acknowledgements

- The pgvector team for database vector search support.
- Replit for custom development plugins.
- Shadcn for components and layout templates.
