# TRIVY_VERSION is a global ARG (declared before any FROM) so it can interpolate
# into the trivy stage's FROM line below.
ARG TRIVY_VERSION=0.70.0

# Build the backend as a static, CGO-free binary (modernc sqlite => no gcc).
# Build context is the repo root; this module has no local-module deps, so we
# build it standalone with GOWORK=off.
FROM golang:1.26 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ENV GOWORK=off CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/stratum .

# Trivy is bundled so container-image CVE scanning works out of the box with zero
# config. The aquasec/trivy image is multi-arch; BuildKit resolves the platform
# for this stage from the build's TARGETPLATFORM, so the COPY below lands the
# correct amd64/arm64 binary automatically (no hardcoded arch).
FROM aquasec/trivy:${TRIVY_VERSION} AS trivy

FROM gcr.io/distroless/static-debian12:nonroot
# Bundled trivy lands on PATH at /usr/local/bin/trivy so exec.LookPath("trivy")
# finds it with zero config. TRIVY_PATH still overrides when set. (BuildKit does
# not expand ARGs in COPY --from, so the stage name is fixed; to ship without
# trivy, build a variant that skips this COPY and rely on TRIVY_PATH instead.)
COPY --from=trivy /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=build /out/stratum /stratum
# Container-troubleshooting skill library (reference data). Build context is the
# repo root, so assets/skills is available; SKILLS_DIR defaults to /app/skills.
COPY assets/skills /app/skills
# Persistent, writable Trivy cache/DB on the mounted data volume (uid 65532 owns
# /data). First scan downloads the DB from mirror.gcr.io/aquasec/trivy-db, then
# it's reused across restarts. Overridable via TRIVY_CACHE_DIR.
ENV TRIVY_CACHE_DIR=/data/trivy-cache
# /data holds the SQLite DB + agent CA + trivy cache; mount a volume here.
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/stratum"]
