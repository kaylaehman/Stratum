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

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/stratum /stratum
# /data holds the SQLite DB + agent CA; mount a volume here.
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/stratum"]
