# TRIVY_VERSION is a global ARG (declared before any FROM) so it can interpolate
# into the trivy stage's FROM line below.
ARG TRIVY_VERSION=0.70.0

# Build the backend as a static, CGO-free binary (modernc sqlite => no gcc).
# Build context is the repo root; this module has no local-module deps, so we
# build it standalone with GOWORK=off.
FROM golang:1.26 AS build
ENV GOWORK=off CGO_ENABLED=0
WORKDIR /src
# backend depends on the local proto module via `replace ... => ../proto`, so the
# proto module must be present in the image at the path the replace resolves to
# (/proto, i.e. ../proto from /src). Copy it before downloading/building.
COPY proto/ /proto/
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/stratum .

# Build the per-host agent for both linux arches so the backend can serve either
# to the install script (the target node's arch is detected at install time).
# Separate stage: the agent is its own module. GOWORK=off, and the agent's
# `replace => ../proto` resolves /proto from /src, mirroring the backend stage.
FROM golang:1.26 AS agent-build
ENV GOWORK=off CGO_ENABLED=0 GOOS=linux
WORKDIR /src
COPY proto/ /proto/
COPY agent/go.mod agent/go.sum ./
RUN go mod download
COPY agent/ ./
RUN GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/agent-bin/stratum-agent-linux-amd64 . \
 && GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o /out/agent-bin/stratum-agent-linux-arm64 .

# Optional Trivy stage. The aquasec/trivy image is multi-arch; BuildKit resolves
# the platform from TARGETPLATFORM, so the COPY in app-trivy lands the correct
# amd64/arm64 binary. BuildKit skips unreferenced stages, so the default build
# (target: app) never pulls this ~150MB image — keeping redeploys fast.
FROM aquasec/trivy:${TRIVY_VERSION} AS trivy

# Shared runtime image (no Trivy). Basis for both variants below.
FROM gcr.io/distroless/static-debian12:nonroot AS app-base
COPY --from=build /out/stratum /stratum
# Per-arch agent binaries served to the install script; AGENT_BIN_DIR defaults
# to /app/agent-bin.
COPY --from=agent-build /out/agent-bin /app/agent-bin
# Container-troubleshooting skill library (reference data). Build context is the
# repo root, so assets/skills is available; SKILLS_DIR defaults to /app/skills.
COPY assets/skills /app/skills
# Writable Trivy cache/DB on the mounted data volume (uid 65532 owns /data); used
# only when Trivy is available (bundled via app-trivy, or via TRIVY_PATH).
ENV TRIVY_CACHE_DIR=/data/trivy-cache
# /data holds the SQLite DB + agent CA + trivy cache; mount a volume here.
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/stratum"]

# Opt-in variant: bundle Trivy for zero-config container-image CVE scanning. It
# pulls the ~150MB aquasec/trivy image, so it is NOT the default. Enable with:
#   docker build --target app-trivy ...
#   or compose: services.backend.build.target: app-trivy
# trivy lands on PATH at /usr/local/bin/trivy so exec.LookPath("trivy") finds it;
# TRIVY_PATH still overrides when set.
FROM app-base AS app-trivy
COPY --from=trivy /usr/local/bin/trivy /usr/local/bin/trivy

# Default target (last stage): slim image WITHOUT Trivy. CVE scanning reports
# "not available" until you bundle Trivy (app-trivy) or set TRIVY_PATH to a host
# trivy. Fast to build — no aquasec/trivy pull.
FROM app-base AS app
