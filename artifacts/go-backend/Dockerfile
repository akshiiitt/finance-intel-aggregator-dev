# ── Builder (shared) ───────────────────────────────────────────────────────────
# BUILDPLATFORM/TARGETARCH are provided by Docker buildx. Building natively on
# the target VM (plain `docker build`) also works: TARGETARCH defaults to the
# build machine's arch, so this produces an arm64 binary on the Ampere A1 VM
# and an amd64 binary on an x86 CI runner — no hardcoded arch to get wrong.
FROM --platform=${BUILDPLATFORM:-linux/arm64} golang:1.23-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Cache module downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# ── Server binary ─────────────────────────────────────────────────────────────
FROM builder AS build-server
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} \
    go build -trimpath -ldflags="-w -s" -o financeintel-server ./cmd/server

# ── Worker binary ─────────────────────────────────────────────────────────────
FROM builder AS build-worker
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} \
    go build -trimpath -ldflags="-w -s" -o financeintel-worker ./cmd/worker

# ── Server runtime image ───────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS server

COPY --from=build-server /build/financeintel-server /app/financeintel-server
COPY --from=build-server /build/migrations          /app/migrations

WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/financeintel-server"]

# ── Worker runtime image ───────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS worker

COPY --from=build-worker /build/financeintel-worker /app/financeintel-worker

WORKDIR /app
ENTRYPOINT ["/app/financeintel-worker"]
