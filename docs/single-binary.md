# Single-binary distribution

By default Stratum runs as two separate services (backend API + Vite/nginx for
the frontend).  A `go build -tags embed` build embeds the compiled SPA into the
Go binary so a single process serves both on one port.  This is useful for
simple self-hosted installs where running two containers is inconvenient.

---

## Build steps

```bash
# 1. Build the frontend SPA.
cd frontend
npm ci
npm run build           # output: frontend/dist/

# 2. Copy the built assets where the Go embed directive expects them.
#    (Go embed cannot reference paths outside the module.)
cp -r dist ../backend/embedspa/dist

# 3. Build the single binary.
cd ../backend
go build -tags embed -o stratum .

# 4. Run it.
./stratum
```

The binary will serve:

- `GET /api/...` — backend REST API (unchanged)
- `GET /metrics` — Prometheus scrape endpoint (unchanged)
- `GET /*` — embedded React SPA (index.html for all unknown paths)

---

## Default (no-tag) build — unchanged behaviour

```bash
cd backend
go build -o stratum .
```

Without the `embed` tag, the `/*` catch-all route is **not registered**.  The
binary serves the API only, and the frontend must be served separately (Vite dev
server, nginx, Caddy, etc.).  This is the standard docker-compose setup and is
not affected by the embed machinery.

---

## How it works

Two build-tag-gated file pairs implement the pattern:

| File | Tag | Purpose |
|---|---|---|
| `backend/embedspa/embed_on.go` | `embed` | `//go:embed all:dist` directive; exposes `Handler()` |
| `backend/embedspa/embed_off.go` | `!embed` | `Handler()` returns `nil` (no-op) |
| `backend/server/embed_on.go` | `embed` | Calls `embedspa.Handler()` and mounts it as chi `/*` |
| `backend/server/embed_off.go` | `!embed` | `mountSPA()` is a no-op |

`backend/embedspa/dist/` is listed in `.gitignore` — it must be populated by
the build step above before compiling with `-tags embed`.

---

## Docker single-image build

```dockerfile
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./embedspa/dist
RUN go build -tags embed -o stratum .

FROM alpine:3.21
COPY --from=backend /app/backend/stratum /stratum
ENTRYPOINT ["/stratum"]
```

---

## Caveats

- `backend/embedspa/dist/` must exist and be non-empty before running
  `go build -tags embed`.  The build will fail with a missing-file error if the
  frontend has not been built first.
- The SPA is baked into the binary at compile time.  Updating the frontend
  requires a rebuild of the binary.
- Cache headers: the embedded file server sets no explicit cache-control headers.
  For production, put Caddy or nginx in front to set `Cache-Control: immutable`
  on the hashed asset files in `dist/assets/`.
