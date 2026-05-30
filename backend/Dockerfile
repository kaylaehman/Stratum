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
ARG TRIVY_VERSION=0.70.0
FROM aquasec/trivy:${TRIVY_VERSION} AS trivy

# Opt-out source: a stage that carries trivy under the SAME path but sourced from
# an empty scratch layer, so the COPY below is satisfied without bundling the
# ~160MB binary. We seed it from the build stage's tiny static binary directory
# only to provide a valid path; the scanner then reports "not available" unless
# TRIVY_PATH points at a host trivy.
FROM busybox:1.37 AS notrivy
RUN mkdir -p /usr/local/bin && : > /usr/local/bin/trivy

# TRIVY_SRC selects the source stage for the trivy COPY (BuildKit resolves the
# COPY --from=${TRIVY_SRC} indirection):
#   - "trivy"   (default) -> bundle the multi-arch ~160MB trivy binary
#   - "notrivy"           -> ship an empty placeholder (size-sensitive opt-out);
#                            scanning reports "not available" unless TRIVY_PATH set.
# Build with --build-arg TRIVY_SRC=notrivy to opt out.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TRIVY_SRC=trivy
# Bundled trivy lands on PATH at /usr/local/bin/trivy so exec.LookPath("trivy")
# finds it with zero config. TRIVY_PATH still overrides when set.
COPY --from=${TRIVY_SRC} /usr/local/bin/trivy /usr/local/bin/trivy
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
